package core

import (
	"context"
	"database/sql"
	"time"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/validx"
)

// Config 是数据库连接与行为配置。
type Config struct {
	// Driver 方言名:"mysql" / "sqlite" / "postgres",或已注册的驱动名。
	Driver string
	// DSN 数据库连接串。
	DSN string
	// MaxOpenConns 最大打开连接数,0 表示不限制。
	MaxOpenConns int
	// MaxIdleConns 最大空闲连接数,0 表示不保留空闲连接。
	MaxIdleConns int
	// ConnMaxLifetime 连接最大存活时间,0 表示不限制。
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime 连接最大空闲时间,0 表示不限制。
	ConnMaxIdleTime time.Duration
	// SlowQueryThreshold 慢查询阈值,0 表示使用默认值(100ms)。
	SlowQueryThreshold time.Duration
	// Logger 结构化日志实现,空表示关闭日志。
	Logger logx.Logger
	// LogSQL 是否打印执行的 SQL(debug 级别)。
	LogSQL bool
	// LogArgs SQL 日志是否附带参数。
	LogArgs bool
	// Metrics 指标钩子,空表示关闭指标。
	Metrics Metrics
	// TraceHook 链路追踪钩子,空表示关闭追踪。
	TraceHook TraceHook
	// EventHook 事件钩子,空表示关闭事件（默认）。
	EventHook EventHook
}

// Option 是 Config 的修改项,在 Open / OpenConfig 时按顺序应用。
type Option func(*Config)

// WithMaxOpenConns 设置最大打开连接数。
func WithMaxOpenConns(n int) Option {
	return func(c *Config) { c.MaxOpenConns = n }
}

// WithMaxIdleConns 设置最大空闲连接数。
func WithMaxIdleConns(n int) Option {
	return func(c *Config) { c.MaxIdleConns = n }
}

// WithConnMaxLifetime 设置连接最大存活时间。
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) { c.ConnMaxLifetime = d }
}

// WithConnMaxIdleTime 设置连接最大空闲时间。
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(c *Config) { c.ConnMaxIdleTime = d }
}

// WithSlowQueryThreshold 设置慢查询阈值。
func WithSlowQueryThreshold(d time.Duration) Option {
	return func(c *Config) { c.SlowQueryThreshold = d }
}

// WithLogger 设置结构化日志实现。
func WithLogger(l logx.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithLogSQL 设置是否打印执行的 SQL。
func WithLogSQL(enabled bool) Option {
	return func(c *Config) { c.LogSQL = enabled }
}

// WithLogArgs 设置 SQL 日志是否附带参数。
func WithLogArgs(enabled bool) Option {
	return func(c *Config) { c.LogArgs = enabled }
}

// WithMetrics 设置指标钩子。
func WithMetrics(m Metrics) Option {
	return func(c *Config) { c.Metrics = m }
}

// WithTraceHook 设置链路追踪钩子。
func WithTraceHook(h TraceHook) Option {
	return func(c *Config) { c.TraceHook = h }
}

// WithEventHook 设置事件钩子。
func WithEventHook(h EventHook) Option {
	return func(c *Config) { c.EventHook = h }
}

// DB 持有 *sql.DB 与配置,是 dbx 的数据库入口。
type DB struct {
	sqlDB *sql.DB
	cfg   Config
	// dialect 是当前数据库方言,用于构造器占位符/标识符/分页渲染。
	dialect Dialect
}

// Open 按方言名打开数据库并探测连接。
// dialect 支持 "mysql" / "sqlite" / "postgres"(需先导入对应子包),
// 也允许直接使用已注册的驱动名。
func Open(ctx context.Context, dialect, dsn string, opts ...Option) (*DB, error) {
	cfg := Config{Driver: dialect, DSN: dsn}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	driverName, err := resolveDriver(dialect)
	if err != nil {
		return nil, err
	}
	return open(ctx, driverName, cfg)
}

// OpenConfig 通过结构体配置打开数据库并探测连接。
func OpenConfig(ctx context.Context, cfg Config, opts ...Option) (*DB, error) {
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	driverName, err := resolveDriver(cfg.Driver)
	if err != nil {
		return nil, err
	}
	return open(ctx, driverName, cfg)
}

// Ping 探测数据库连接是否可用。
func (db *DB) Ping(ctx context.Context) error {
	if err := db.sqlDB.PingContext(ctx); err != nil {
		return errx.WrapCode(err, CodeOpenFailed, "数据库连接探测失败")
	}
	return nil
}

// Close 关闭数据库连接池,释放全部连接。
func (db *DB) Close() error {
	if err := db.sqlDB.Close(); err != nil {
		return errx.WrapCode(err, CodeCloseFailed, "关闭数据库连接失败")
	}
	return nil
}

// Exec 执行不返回行的 SQL,并返回影响行数等信息。
func (db *DB) Exec(ctx context.Context, q Query) (sql.Result, error) {
	sqlText, args, err := renderQuery(q, db.dialect)
	if err != nil {
		return nil, err
	}
	ctx, end := db.startTrace(ctx, "dbx.exec", "exec", sqlText)
	start := time.Now()
	res, err := db.sqlDB.ExecContext(ctx, sqlText, args...)
	end(err)
	observe(db.cfg, "exec", sqlText, args, start, err)
	if err != nil {
		return nil, wrapExecError(err)
	}
	return res, nil
}

// Query 执行查询并返回结果集。
func (db *DB) Query(ctx context.Context, q Query) (*sql.Rows, error) {
	sqlText, args, err := renderQuery(q, db.dialect)
	if err != nil {
		return nil, err
	}
	ctx, end := db.startTrace(ctx, "dbx.query", "query", sqlText)
	start := time.Now()
	rows, err := db.sqlDB.QueryContext(ctx, sqlText, args...)
	end(err)
	observe(db.cfg, "query", sqlText, args, start, err)
	if err != nil {
		return nil, errx.WrapCode(err, CodeQueryFailed, "查询失败")
	}
	return rows, nil
}

// QueryRow 执行查询并返回单行结果;
// 查询构造失败时直接返回错误,其余错误延迟到 Scan 时返回。
func (db *DB) QueryRow(ctx context.Context, q Query) (*sql.Row, error) {
	sqlText, args, err := renderQuery(q, db.dialect)
	if err != nil {
		return nil, err
	}
	ctx, end := db.startTrace(ctx, "dbx.query_row", "query_row", sqlText)
	start := time.Now()
	row := db.sqlDB.QueryRowContext(ctx, sqlText, args...)
	end(nil)
	observe(db.cfg, "query_row", sqlText, args, start, nil)
	return row, nil
}

// startTrace 开始链路追踪（无钩子时 no-op）。
func (db *DB) startTrace(ctx context.Context, name, op, sqlText string) (context.Context, func(error)) {
	if db.cfg.TraceHook == nil && db.cfg.EventHook == nil {
		return ctx, func(error) {}
	}
	attrs := []TraceAttr{
		TraceAttr{Key: "db.system", Value: db.dialect.Name()},
		TraceAttr{Key: "db.operation", Value: op},
		TraceAttr{Key: "db.statement", Value: sqlText},
	}
	traceEnd := func(error) {}
	if db.cfg.TraceHook != nil {
		ctx, traceEnd = db.cfg.TraceHook.Start(ctx, name, attrs...)
	}
	system := db.dialect.Name()
	return ctx, func(err error) {
		traceEnd(err)
		if db.cfg.EventHook != nil {
			db.cfg.EventHook.OnQueryEvent(ctx, QueryEvent{
				System:    system,
				Operation: op,
				Statement: sqlText,
				Err:       err,
			})
		}
	}
}

// renderQuery 按数据库方言渲染查询;固定 SQL 原样返回。
func renderQuery(q Query, d Dialect) (string, []any, error) {
	if dq, ok := q.(interface {
		render(d Dialect) (string, []any, error)
	}); ok {
		return dq.render(d)
	}
	return q.SQL()
}

// knownDrivers 是内置方言名到 database/sql 驱动名的映射。
var knownDrivers = map[string]string{
	"mysql":    "mysql",
	"sqlite":   "sqlite",
	"postgres": "pgx",
}

// resolveDriver 将方言名解析为已注册的驱动名。
func resolveDriver(dialect string) (string, error) {
	if driverName, ok := knownDrivers[dialect]; ok {
		if !driverRegistered(driverName) {
			return "", errx.NewCodef(CodeDriverNotRegistered,
				"方言 %q 未注册,请导入对应子包", dialect)
		}
		return driverName, nil
	}
	for _, name := range sql.Drivers() {
		if name == dialect {
			return dialect, nil
		}
	}
	return "", errx.NewCodef(CodeDriverNotRegistered, "方言 %q 未注册", dialect)
}

// driverRegistered 判断驱动名是否已注册到 database/sql。
func driverRegistered(name string) bool {
	for _, registered := range sql.Drivers() {
		if registered == name {
			return true
		}
	}
	return false
}

// init 注册配置校验规则到 validx 全局规则表，错误码保持 dbx 语义。
func init() {
	_ = validx.RegisterRule("dbx_config", func(value any, param, path string) error {
		// 内部调用保证 value 为 Config。
		cfg := value.(Config)
		if cfg.Driver == "" {
			return errx.NewCode(CodeBadArgument, "驱动/方言不能为空")
		}
		if cfg.DSN == "" {
			return errx.NewCode(CodeBadArgument, "DSN 不能为空")
		}
		if cfg.MaxOpenConns < 0 {
			return errx.NewCode(CodeBadArgument, "MaxOpenConns 不能为负数")
		}
		if cfg.MaxIdleConns < 0 {
			return errx.NewCode(CodeBadArgument, "MaxIdleConns 不能为负数")
		}
		if cfg.ConnMaxLifetime < 0 {
			return errx.NewCode(CodeBadArgument, "ConnMaxLifetime 不能为负数")
		}
		if cfg.ConnMaxIdleTime < 0 {
			return errx.NewCode(CodeBadArgument, "ConnMaxIdleTime 不能为负数")
		}
		if cfg.SlowQueryThreshold < 0 {
			return errx.NewCode(CodeBadArgument, "SlowQueryThreshold 不能为负数")
		}
		return nil
	})
}

// validateConfig 校验配置参数，负数连接池参数与空驱动/DSN 均视为非法
// （统一走 validx 规则）。
func validateConfig(cfg Config) error {
	return validx.ValidateField(cfg, "dbx_config")
}

// open 创建 sql.DB、应用连接池参数并探测连接。
func open(ctx context.Context, driverName string, cfg Config) (*DB, error) {
	sqlDB, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, errx.WrapCode(err, CodeOpenFailed, "打开数据库连接失败")
	}
	db := &DB{sqlDB: sqlDB, cfg: cfg, dialect: dialectFor(cfg.Driver)}
	db.applyPoolConfig()
	if err := db.Ping(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// applyPoolConfig 将配置中的连接池参数应用到 sql.DB。
func (db *DB) applyPoolConfig() {
	db.sqlDB.SetMaxOpenConns(db.cfg.MaxOpenConns)
	db.sqlDB.SetMaxIdleConns(db.cfg.MaxIdleConns)
	db.sqlDB.SetConnMaxLifetime(db.cfg.ConnMaxLifetime)
	db.sqlDB.SetConnMaxIdleTime(db.cfg.ConnMaxIdleTime)
}
