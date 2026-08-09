# dbx API 设计(v0.1.0 基线)

> 已定稿(2026-08-09):**D1–D8 全部按推荐项确认**,本文作为 v0.1.0 API 基线;
> 实现过程中已确认的签名调整(见 CHANGELOG):
> 泛型扫描改为包级函数;`Query.SQL()` 增加错误返回;`QueryRow` 返回
> `(*sql.Row, error)`;`Dialect.LimitOffset` 增加起始参数序号;
> `SupportsReturning` / `InsertReturning` 延后到 P5 再定。

## 1. 包结构

```text
dbx/
├── dbx.go        # DB、Open、OpenConfig、连接池
├── tx.go         # Tx、WithTx、嵌套保存点、BatchExec
├── scan.go       # One[T]、List[T]、RowMapper
├── builder.go    # Raw、Select、条件/排序/分页
├── dialect.go    # Dialect 接口与注册
├── errors.go     # DBX_* 错误码
├── logadapter.go # 慢查询与错误日志
├── metrics.go    # 指标钩子
├── mysql/        # mysql.Open(...)
├── sqlite/       # sqlite.Open(...)
├── pg/           # pg.Open(...)
└── migrate/      # migrate.Run(ctx, db, fsys)
```

## 2. 查询值:统一入口

所有执行方法都接受 `dbx.Query`,固定 SQL 用 `Raw`,动态 SQL 用 `Select`:

```go
// Query 是 dbx 中所有可执行查询的统一形态。
type Query interface {
    SQL() (string, []any, error) // 构造失败返回 DBX_BAD_ARGUMENT
}

// Raw 包装固定 SQL,不解析、不校验,原样交给数据库。
func Raw(sql string, args ...any) Query

// Select 以原生 SQL 为主体,安全追加 WHERE/ORDER BY/LIMIT。
func Select(sql string) *SelectQuery
```

```go
// 固定 SQL(目标方言,可直接粘贴到数据库客户端)
q := dbx.Raw(`SELECT id, name FROM users WHERE id = $1`, 42)

// 动态查询(条件片段使用 ?,构造时按方言转换)
q := dbx.Select(`SELECT id, name FROM users`).
    Where(`name LIKE ?`, "%张%").
    And(`status = ?`, 1).
    OrderBy(`created_at`, false).
    Page(1, 20)
```

## 3. 核心类型

### 3.1 DB

```go
type DB struct { /* 持有 *sql.DB 与配置 */ }

// 打开连接;dialect 名称为 "mysql" / "sqlite" / "postgres"。
func Open(ctx context.Context, dialect, dsn string, opts ...Option) (*DB, error)

// 通过结构体配置打开(DSN、连接池、日志、指标)。
func OpenConfig(ctx context.Context, cfg Config, opts ...Option) (*DB, error)

func (db *DB) Ping(ctx context.Context) error
func (db *DB) Close() error

func (db *DB) Exec(ctx context.Context, q Query) (sql.Result, error)
func (db *DB) Query(ctx context.Context, q Query) (*sql.Rows, error)
func (db *DB) QueryRow(ctx context.Context, q Query) (*sql.Row, error)

// 扫描到单条记录;无数据返回 DBX_NOT_FOUND。
// 注意:Go 不支持泛型方法,因此 One/List 为包级泛型函数。
// runner 可以是 *DB 或 *Tx。
func One[T any](ctx context.Context, runner QueryRunner, q Query) (T, error)
// 使用自定义映射扫描到单条记录。
func OneWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) (T, error)
// 扫描到切片;无数据返回空切片而非 nil 错误。
func List[T any](ctx context.Context, runner QueryRunner, q Query) ([]T, error)
// 使用自定义映射扫描到切片。
func ListWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) ([]T, error)

func (db *DB) WithTx(ctx context.Context, fn func(*Tx) error, opts ...TxOption) error
func (db *DB) BatchExec(ctx context.Context, sql string, args [][]any) error
```

### 3.2 Tx

```go
type Tx struct { /* 持有 *sql.Tx 与配置 */ }

// QueryRunner 是可执行查询的最小接口,DB 与 Tx 都实现它。
type QueryRunner interface {
    Query(ctx context.Context, q Query) (*sql.Rows, error)
}

// TxOption 是事务选项。
type TxOption func(*sql.TxOptions)
func WithIsolation(level sql.IsolationLevel) TxOption
func WithReadOnly(readOnly bool) TxOption

func (tx *Tx) Exec(ctx context.Context, q Query) (sql.Result, error)
func (tx *Tx) Query(ctx context.Context, q Query) (*sql.Rows, error)
func (tx *Tx) QueryRow(ctx context.Context, q Query) (*sql.Row, error)
func (tx *Tx) Nested(ctx context.Context, fn func(*Tx) error) error
func (tx *Tx) BatchExec(ctx context.Context, sql string, args [][]any) error
```

事务内扫描复用包级 `One[T]` / `List[T]`,把 `*Tx` 作为 `QueryRunner` 传入即可。

```go
err := db.WithTx(ctx, func(tx *dbx.Tx) error {
    if err := tx.Exec(ctx, dbx.Raw(`UPDATE users SET status = $1 WHERE id = $2`, 2, 42)); err != nil {
        return err
    }
    return tx.Exec(ctx, dbx.Raw(`INSERT INTO logs(user_id, action) VALUES ($1, $2)`, 42, "状态变更"))
})
```

### 3.3 SelectQuery

```go
type SelectQuery struct{ /* 未导出 */ }

func (q *SelectQuery) Where(cond string, args ...any) *SelectQuery
func (q *SelectQuery) And(cond string, args ...any) *SelectQuery
func (q *SelectQuery) Or(cond string, args ...any) *SelectQuery
func (q *SelectQuery) In(column string, values ...any) *SelectQuery
func (q *SelectQuery) Like(column, pattern string) *SelectQuery
func (q *SelectQuery) Between(column string, lo, hi any) *SelectQuery
func (q *SelectQuery) IsNull(column string) *SelectQuery
func (q *SelectQuery) OrderBy(column string, desc bool) *SelectQuery
func (q *SelectQuery) Page(page, size int) *SelectQuery   // 页号从 1 开始
func (q *SelectQuery) LimitOffset(limit, offset int64) *SelectQuery
func (q *SelectQuery) SQL() (string, []any, error)
```

条件片段中的占位符统一写 `?`,构造时由 `Dialect` 转换成 `$n`;参数顺序与占位符严格对应。

### 3.4 配置与选项

```go
type Config struct {
    Driver string // "mysql" / "sqlite" / "postgres"
    DSN    string
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
    ConnMaxIdleTime time.Duration
    SlowQueryThreshold time.Duration
    Logger  logx.Logger
    LogSQL  bool   // 打印执行的 SQL(debug 级别)
    LogArgs bool   // SQL 日志是否附带参数
    Metrics Metrics
}

type Option func(*config)

func WithMaxOpenConns(n int) Option
func WithMaxIdleConns(n int) Option
func WithConnMaxLifetime(d time.Duration) Option
func WithConnMaxIdleTime(d time.Duration) Option
func WithSlowQueryThreshold(d time.Duration) Option
func WithLogger(l logx.Logger) Option
func WithLogSQL(enabled bool) Option
func WithLogArgs(enabled bool) Option
func WithMetrics(m Metrics) Option
```

日志对象由外部通过 `WithLogger` 注入,包内只使用不创建;
SQL 打印、参数输出、慢查询阈值、指标钩子均为独立开关。

### 3.5 指标钩子

```go
// Metrics 是最小指标接口,默认 no-op,便于外部接 Prometheus 等适配器。
type Metrics interface {
    IncCounter(name string, labels ...string)
    ObserveDuration(name string, seconds float64, labels ...string)
}
```

### 3.6 方言子包

```go
package mysql  // import "github.com/lcylpzls/dbx/mysql"
func Open(ctx context.Context, dsn string, opts ...dbx.Option) (*dbx.DB, error)

package sqlite // import "github.com/lcylpzls/dbx/sqlite"
func Open(ctx context.Context, dsn string, opts ...dbx.Option) (*dbx.DB, error)

package pg // import "github.com/lcylpzls/dbx/pg"
func Open(ctx context.Context, dsn string, opts ...dbx.Option) (*dbx.DB, error)
```

子包在 `init` 中注册对应驱动与方言,使用者只需导入子包,核心包保持零第三方依赖。

v0.2.0 起 `dbx/sqlite` 的 `Open` 接受 sqlite 专属选项:

```go
func Open(ctx context.Context, dsn string, opts ...Option) (*dbx.DB, error)
func WithPragma(name, value string) Option    // 连接级 PRAGMA,按顺序合并进 DSN _pragma
func WithDBOptions(opts ...dbx.Option) Option // 透传 dbx 通用选项
```

### 3.7 迁移

```go
package migrate

// Run 按文件名顺序执行 fsys 中的 *.sql,并记录 schema_migrations。
// 跳过文件名已存在于版本表中的迁移;每个迁移文件在单个事务内执行,
// 失败时该文件整体回滚(MySQL DDL 非事务,无法跨文件回滚)。
func Run(ctx context.Context, db *dbx.DB, fsys fs.FS) error
```

迁移文件支持多条语句(按分号拆分,自动跳过字符串与注释中的分号)。

### 3.8 confx 配置接入(可选子包)

```go
package confx // import "github.com/lcylpzls/dbx/confx"

// LoadFile 从 TOML 文件解析 dbx.Config,未声明字段走 confx 严格模式。
func LoadFile(path string) (dbx.Config, error)
// Open 从 TOML 文件加载配置并打开数据库连接。
func Open(ctx context.Context, path string, opts ...dbx.Option) (*dbx.DB, error)
```

## 4. 扫描规则

```go
type User struct {
    ID        int64     `db:"id"`
    Name      string    `db:"name"`
    Email     *string   `db:"email"` // NULL → nil
    CreatedAt time.Time `db:"created_at"`
    Ignored   string    `db:"-"`
}
```

- 无 tag 时按字段名与列名不区分大小写匹配;
- 支持嵌入结构体;
- 列多出/缺少均可(按字段名匹配,缺列时字段保持零值);
- 类型不匹配返回 `DBX_SCAN_FAILED`,错误信息包含列名与目标类型。

## 5. 错误码

| 错误码 | 含义 |
| --- | --- |
| `DBX_OPEN_FAILED` | 打开连接失败 |
| `DBX_DRIVER_NOT_REGISTERED` | 方言未注册(未导入对应子包) |
| `DBX_BAD_ARGUMENT` | 非法参数,如标识符未通过白名单 |
| `DBX_EXEC_FAILED` | Exec 失败 |
| `DBX_QUERY_FAILED` | Query 失败 |
| `DBX_SCAN_FAILED` | 扫描/类型转换失败 |
| `DBX_NOT_FOUND` | 查询无结果(包装 `sql.ErrNoRows`) |
| `DBX_TX_BEGIN_FAILED` | 开启事务失败 |
| `DBX_TX_COMMIT_FAILED` | 提交失败 |
| `DBX_TX_ROLLBACK_FAILED` | 回滚失败 |
| `DBX_DUPLICATE` | 唯一约束/重复键冲突(跨 MySQL / SQLite / PostgreSQL 统一识别) |
| `DBX_MIGRATION_FAILED` | 迁移执行失败 |

```go
func IsNotFound(err error) bool
func IsDuplicate(err error) bool
```

## 6. 端到端示例

```go
package main

import (
    "context"
    "time"

    "github.com/lcylpzls/dbx"
    "github.com/lcylpzls/dbx/pg"
)

type User struct {
    ID   int64  `db:"id"`
    Name string `db:"name"`
}

func main() {
    ctx := context.Background()
    db, err := pg.Open(ctx, "postgres://user:pass@localhost:5432/app",
        dbx.WithSlowQueryThreshold(100*time.Millisecond))
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // 固定 SQL:可审查、可粘贴
    u, err := dbx.One[User](ctx, db, dbx.Raw(
        `SELECT id, name FROM users WHERE id = $1`, 42))

    // 动态查询:条件安全构造
    q := dbx.Select(`SELECT id, name FROM users`).
        Where(`name LIKE ?`, "%张%").
        And(`status = ?`, 1).
        OrderBy(`created_at`, false).
        Page(1, 20)
    users, err := dbx.List[User](ctx, db, q)
    _ = u
    _ = users
}
```

## 7. 已确认决策(D1–D8)

> 2026-08-09 全部确认,结论均为推荐项:

| 编号 | 问题 | 推荐 | 备选 |
| --- | --- | --- | --- |
| D1 | SQLite 驱动 | **modernc.org/sqlite**(纯 Go,无 CGO,Windows 友好) | mattn/go-sqlite3(CGO) |
| D2 | PostgreSQL 驱动 | **jackc/pgx(stdlib 模式)** | lib/pq(维护模式) |
| D3 | 查询入口形态 | **统一 `Query` 值**(`Raw` + `Select`) | 字符串 + 构造器双入口 |
| D4 | 构造器范围 | **仅动态条件/排序/分页**,SQL 主体手写 | 完整 SELECT 生成器 |
| D5 | 日志集成 | **直接依赖 logx**(与 webx 一致) | 最小日志接口抽象 |
| D6 | 配置加载 | **核心自带 `Config`,confx 接入放可选子包** | 主模块直接依赖 confx |
| D7 | 迁移方向 | **v0.1 先 up-only**,down 留 v0.2 | 首版即 up/down |
| D8 | Go 版本 | **1.26**(本机与 webx 一致) | 1.21 更广兼容 |
