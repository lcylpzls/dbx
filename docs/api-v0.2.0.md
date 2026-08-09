<!-- v0.2.0 API 基线;生成方式:go doc -all . -->

package dbx // import "github.com/lcylpzls/dbx"

Package dbx 提供基于 database/sql 的薄数据访问层: 统一连接、事务、扫描、动态查询构造与可观测性, 支持
MySQL、SQLite、PostgreSQL。 对外错误统一使用 errx 结构化错误,错误码为 DBX_*。

CONSTANTS

const (
	// CodeOpenFailed 打开数据库连接失败。
	CodeOpenFailed errx.Code = "DBX_OPEN_FAILED"
	// CodeDriverNotRegistered 数据库驱动/方言未注册(未导入对应子包)。
	CodeDriverNotRegistered errx.Code = "DBX_DRIVER_NOT_REGISTERED"
	// CodeBadArgument 参数非法,如标识符未通过白名单校验。
	CodeBadArgument errx.Code = "DBX_BAD_ARGUMENT"
	// CodeExecFailed Exec 执行失败。
	CodeExecFailed errx.Code = "DBX_EXEC_FAILED"
	// CodeQueryFailed 查询失败。
	CodeQueryFailed errx.Code = "DBX_QUERY_FAILED"
	// CodeScanFailed 扫描或类型转换失败。
	CodeScanFailed errx.Code = "DBX_SCAN_FAILED"
	// CodeNotFound 查询无结果。
	CodeNotFound errx.Code = "DBX_NOT_FOUND"
	// CodeTxBeginFailed 开启事务失败。
	CodeTxBeginFailed errx.Code = "DBX_TX_BEGIN_FAILED"
	// CodeTxCommitFailed 提交事务失败。
	CodeTxCommitFailed errx.Code = "DBX_TX_COMMIT_FAILED"
	// CodeTxRollbackFailed 回滚事务失败。
	CodeTxRollbackFailed errx.Code = "DBX_TX_ROLLBACK_FAILED"
	// CodeDuplicate 唯一约束/重复键冲突。
	CodeDuplicate errx.Code = "DBX_DUPLICATE"
	// CodeMigrationFailed 迁移执行失败。
	CodeMigrationFailed errx.Code = "DBX_MIGRATION_FAILED"
)
    错误码定义:dbx 各失败场景的错误码。


FUNCTIONS

func IsDuplicate(err error) bool
    IsDuplicate 判断错误是否为唯一约束/重复键冲突(DBX_DUPLICATE)。 支持错误链,未包装错误或 nil 返回 false。

func IsNotFound(err error) bool
    IsNotFound 判断错误是否为“查询无结果”(DBX_NOT_FOUND)。 支持 errors.As / 错误链,未包装错误或 nil 返回
    false。

func List[T any](ctx context.Context, runner QueryRunner, q Query) ([]T, error)
    List 扫描多条记录到结构体切片;无数据返回空切片而非 nil。

func ListWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) ([]T, error)
    ListWith 使用自定义映射扫描多条记录;无数据返回空切片而非 nil。

func One[T any](ctx context.Context, runner QueryRunner, q Query) (T, error)
    One 扫描单条记录到结构体;无数据返回 DBX_NOT_FOUND。

func OneWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) (T, error)
    OneWith 使用自定义映射扫描单条记录;无数据返回 DBX_NOT_FOUND。

func RegisterDialect(name string, d Dialect)
    RegisterDialect 注册方言,供 mysql / sqlite / pg 子包在 init 中调用。


TYPES

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
}
    Config 是数据库连接与行为配置。

type DB struct {
	// Has unexported fields.
}
    DB 持有 *sql.DB 与配置,是 dbx 的数据库入口。

func Open(ctx context.Context, dialect, dsn string, opts ...Option) (*DB, error)
    Open 按方言名打开数据库并探测连接。 dialect 支持 "mysql" / "sqlite" / "postgres"(需先导入对应子包),
    也允许直接使用已注册的驱动名。

func OpenConfig(ctx context.Context, cfg Config, opts ...Option) (*DB, error)
    OpenConfig 通过结构体配置打开数据库并探测连接。

func (db *DB) BatchExec(ctx context.Context, sqlText string, args [][]any) error
    BatchExec 使用预编译语句批量执行固定 SQL。

func (db *DB) Close() error
    Close 关闭数据库连接池,释放全部连接。

func (db *DB) Exec(ctx context.Context, q Query) (sql.Result, error)
    Exec 执行不返回行的 SQL,并返回影响行数等信息。

func (db *DB) Ping(ctx context.Context) error
    Ping 探测数据库连接是否可用。

func (db *DB) Query(ctx context.Context, q Query) (*sql.Rows, error)
    Query 执行查询并返回结果集。

func (db *DB) QueryRow(ctx context.Context, q Query) (*sql.Row, error)
    QueryRow 执行查询并返回单行结果; 查询构造失败时直接返回错误,其余错误延迟到 Scan 时返回。

func (db *DB) WithTx(ctx context.Context, fn func(*Tx) error, opts ...TxOption) error
    WithTx 开启事务并执行回调,回调返回错误或 panic 时自动回滚。

type Dialect interface {
	// Name 返回方言名,如 "mysql" / "sqlite" / "postgres"。
	Name() string
	// Placeholder 返回第 index 个参数占位符(index 从 0 开始)。
	Placeholder(index int) string
	// QuoteIdent 白名单校验标识符并加引号,非法输入返回 DBX_BAD_ARGUMENT。
	QuoteIdent(name string) (string, error)
	// LimitOffset 生成分页片段并返回追加参数;
	// start 是分页参数在全部参数中的起始序号(供 PostgreSQL 计算 $n)。
	LimitOffset(start int, limit, offset int64) (string, []any)
}
    Dialect 定义方言层的最小能力,只覆盖无法用同一份 SQL 表达的最小差异。

type Metrics interface {
	// IncCounter 增加一个计数指标。
	IncCounter(name string, labels ...string)
	// ObserveDuration 记录一次耗时观测。
	ObserveDuration(name string, seconds float64, labels ...string)
}
    Metrics 是最小指标接口,默认 no-op,便于外部接 Prometheus 等适配器。

type Option func(*Config)
    Option 是 Config 的修改项,在 Open / OpenConfig 时按顺序应用。

func WithConnMaxIdleTime(d time.Duration) Option
    WithConnMaxIdleTime 设置连接最大空闲时间。

func WithConnMaxLifetime(d time.Duration) Option
    WithConnMaxLifetime 设置连接最大存活时间。

func WithLogArgs(enabled bool) Option
    WithLogArgs 设置 SQL 日志是否附带参数。

func WithLogSQL(enabled bool) Option
    WithLogSQL 设置是否打印执行的 SQL。

func WithLogger(l logx.Logger) Option
    WithLogger 设置结构化日志实现。

func WithMaxIdleConns(n int) Option
    WithMaxIdleConns 设置最大空闲连接数。

func WithMaxOpenConns(n int) Option
    WithMaxOpenConns 设置最大打开连接数。

func WithMetrics(m Metrics) Option
    WithMetrics 设置指标钩子。

func WithSlowQueryThreshold(d time.Duration) Option
    WithSlowQueryThreshold 设置慢查询阈值。

type Query interface {
	// SQL 返回 SQL 文本、绑定参数与构造错误。
	// 参数顺序与占位符严格对应;构造失败返回 DBX_BAD_ARGUMENT。
	SQL() (string, []any, error)
}
    Query 是 dbx 中所有可执行查询的统一形态。

func Raw(sql string, args ...any) Query
    Raw 包装固定 SQL,原样交给数据库执行。

type QueryRunner interface {
	Query(ctx context.Context, q Query) (*sql.Rows, error)
}
    QueryRunner 是可执行查询的最小接口,DB 与 Tx 都实现它。

type Row interface {
	Columns() ([]string, error)
	Scan(dest ...any) error
}
    Row 是单行结果的最小视图,可由 *sql.Rows 或自定义行实现。

type RowMapper[T any] func(row Row) (T, error)
    RowMapper 将一行原始列值映射为自定义结果。

type SelectQuery struct {
	// Has unexported fields.
}
    SelectQuery 是动态查询构造器,只扩展 WHERE / ORDER BY / LIMIT 三个层面。

func Select(sql string) *SelectQuery
    Select 以原生 SQL 为主体,安全追加 WHERE / ORDER BY / LIMIT。 SQL 主体必须非空;条件片段中的参数占位符统一写
    "?"。

func (q *SelectQuery) And(cond string, args ...any) *SelectQuery
    And 以 AND 追加一个条件。

func (q *SelectQuery) Args(args ...any) *SelectQuery
    Args 追加 SQL 主体的绑定参数,顺序位于条件参数之前。 用于 INSERT / UPDATE 等需要主体参数且不含条件参数的场景。

func (q *SelectQuery) Between(column string, lo, hi any) *SelectQuery
    Between 追加 column BETWEEN ? AND ? 条件。

func (q *SelectQuery) In(column string, values ...any) *SelectQuery
    In 追加 column IN (?, ?, ...) 条件。

func (q *SelectQuery) IsNull(column string) *SelectQuery
    IsNull 追加 column IS NULL 条件。

func (q *SelectQuery) Like(column, pattern string) *SelectQuery
    Like 追加 column LIKE ? 条件,pattern 始终作为绑定参数。

func (q *SelectQuery) LimitOffset(limit, offset int64) *SelectQuery
    LimitOffset 设置 LIMIT / OFFSET,分页参数始终作为绑定参数。

func (q *SelectQuery) Or(cond string, args ...any) *SelectQuery
    Or 以 OR 追加一个条件。

func (q *SelectQuery) OrderBy(column string, desc bool) *SelectQuery
    OrderBy 追加排序项,列名在渲染时走白名单校验。

func (q *SelectQuery) Page(page, size int) *SelectQuery
    Page 设置分页,页号从 1 开始。

func (q *SelectQuery) SQL() (string, []any, error)
    SQL 按回退方言渲染查询;数据库执行时按实际方言渲染。

func (q *SelectQuery) Where(cond string, args ...any) *SelectQuery
    Where 追加一个条件;非首个条件时自动使用 AND 连接。

type Tx struct {
	// Has unexported fields.
}
    Tx 是数据库事务,持有 *sql.Tx 与方言。

func (tx *Tx) BatchExec(ctx context.Context, sqlText string, args [][]any) error
    BatchExec 在事务内使用预编译语句批量执行固定 SQL。

func (tx *Tx) Exec(ctx context.Context, q Query) (sql.Result, error)
    Exec 在事务内执行 SQL,返回影响行数等信息。

func (tx *Tx) Nested(ctx context.Context, fn func(*Tx) error) (err error)
    Nested 在事务内开启保存点嵌套事务, 回调返回错误或 panic 时回滚到保存点并释放。

func (tx *Tx) Query(ctx context.Context, q Query) (*sql.Rows, error)
    Query 在事务内执行查询。

func (tx *Tx) QueryRow(ctx context.Context, q Query) (*sql.Row, error)
    QueryRow 在事务内执行单行查询,构造失败直接返回错误,其余错误延迟到 Scan。

type TxOption func(*sql.TxOptions)
    TxOption 是事务选项。

func WithIsolation(level sql.IsolationLevel) TxOption
    WithIsolation 设置事务隔离级别。

func WithReadOnly(readOnly bool) TxOption
    WithReadOnly 设置只读事务。

