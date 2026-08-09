# dbx 架构设计

> 版本:v0.0.0(规划稿) · 状态:评审中

## 1. 总体分层

```text
┌───────────────────────────────────────────────────┐
│ 业务代码 / AI 生成的 SQL                          │
├───────────────────────────────────────────────────┤
│ dbx 核心包                                        │
│  DB / Tx / 扫描 / 构造器 / 错误 / 日志 / 指标     │
├───────────────────────────────────────────────────┤
│ Dialect 方言层(占位符 / 分页 / Returning)        │
├───────────────────────────────────────────────────┤
│ database/sql + 第三方驱动(由子包导入)            │
├───────────────────────────────────────────────────┤
│ MySQL / SQLite / PostgreSQL                       │
└───────────────────────────────────────────────────┘
```

## 2. 设计原则的架构落点

- **原生 SQL 优先** → 执行入口接受「SQL 文本 + 参数」,不做 SQL 解析与改写;
- **薄而克制** → 核心只包含少量文件级模块,不引入复杂引擎;
- **安全默认** → 构造器是唯一允许「拼 SQL」的地方,且只拼受控片段;
- **零泄漏** → 第三方驱动类型不出现在核心包公开 API 中。

## 3. 核心模块职责

| 模块 | 职责 |
| --- | --- |
| `dbx.go` | `DB` 类型、`Open` / `OpenConfig`、连接池参数、`Ping` / `Close` |
| `tx.go` | `Tx` 类型、`WithTx`、嵌套保存点、批量执行 |
| `scan.go` | 泛型扫描 `One` / `List`、字段元信息缓存、`RowMapper` |
| `builder.go` | `Raw` / `Select` 查询值、条件、排序、分页,输出 SQL 片段与参数 |
| `dialect.go` | `Dialect` 接口与注册表 |
| `errors.go` | `DBX_*` 错误码与判定助手 |
| `logadapter.go` | logx 适配、慢查询与错误日志 |
| `metrics.go` | 执行耗时与计数钩子 |

子包:

- `dbx/mysql`、`dbx/sqlite`、`dbx/pg`:导入驱动并注册方言,提供 `Open` 快捷入口;
- `dbx/migrate`:迁移执行器;
- `dbx/confx`(可选):把连接配置接入 confx(TOML)。

## 4. 方言层

`Dialect` 只覆盖「无法用同一份 SQL 表达」的最小差异:

```go
type Dialect interface {
    Name() string
    Placeholder(index int) string           // 0 起始:返回 "?" 或 "$1"
    QuoteIdent(name string) (string, error) // 白名单校验后加引号
    LimitOffset(start int, limit, offset int64) (string, []any)
    // start 是分页参数在全部参数中的起始序号(PostgreSQL 计算 $n 需要)
}
```

**不翻译** SELECT / UPDATE / DELETE 主体;不处理 JOIN、子查询、函数等语法差异——这些留给开发者按目标数据库编写。

> Returning 相关能力(SupportsReturning / InsertReturning)延后到 P5
> 事务与插入辅助设计时再定。

## 5. 构造器设计

- 每个条件片段都是 `fragment{sql string, args []any}`;
- `Select` 维护 SQL 主体与片段链表,`Where` / `And` / `Or` / `In` / `Like` / `Between` / `IsNull` 追加片段;
- 拼接时由当前 `Dialect` 统一替换占位符(`?` → `$n`);
- 标识符白名单:`^[A-Za-z_][A-Za-z0-9_.]*$`,非法输入直接返回 `DBX_BAD_ARGUMENT`;
- 排序字段必须经 `OrderBy(column, desc)`,列名同样白名单校验;
- 分页参数永远是绑定参数,绝不拼进 SQL 文本。

## 6. 扫描器

- `One[T]` / `List[T]` 基于泛型 + 反射读取字段元信息;
- 元信息缓存在 `sync.Map`(类型 → 列索引表),避免热路径重复反射;
- tag 规则:`db:"column"` 映射列名;`db:"-"` 跳过;无 tag 时按字段名与列名不区分大小写匹配;
- 类型转换失败返回 `DBX_SCAN_FAILED`,并携带列名与目标类型;
- NULL 按目标类型归零;指针与 `sql.Scanner` 原生支持。

## 7. 事务

- `WithTx` 负责 Begin / Commit / Rollback 完整生命周期,回调用 `defer` 兜底 panic;
- 嵌套事务通过保存点实现:内部生成 `SAVEPOINT dbx_N` / `RELEASE SAVEPOINT dbx_N` / `ROLLBACK TO SAVEPOINT dbx_N`;
- `BatchExec` 复用 Prepare 语句,减少批量场景的重复解析开销。

## 8. 可观测性

- 每个执行入口统一走 `instrument(ctx, op, sql, args, fn)` 包装:
  - 记录耗时,超过阈值输出 logx 慢查询日志(SQL 截断到 512 字符,参数列表输出);
  - 指标计数:Queries / Errors / SlowQueries / TotalDuration;
- 日志接口使用 logx 类型(与 webx 一致);指标为最小接口,默认 no-op,便于后续接 Prometheus 等适配器。

## 9. 错误模型

- 所有对外错误均为 errx 包装,携带 `DBX_*` 错误码;
- 错误码完整表见 [api-design.md](api-design.md);
- `sql.ErrNoRows` 统一映射为 `DBX_NOT_FOUND`,其余驱动错误保留原始错误链,便于排查。

## 10. 目标目录结构

```text
dbx/
├── AGENTS.md
├── CHANGELOG.md
├── LICENSE
├── README.md
├── go.mod                 # module github.com/lcylpzls/dbx
├── dbx.go                 # DB 核心:Open/OpenConfig/连接池
├── tx.go                  # 事务与嵌套保存点
├── scan.go                # 泛型扫描与字段缓存
├── builder.go             # Raw/Select 查询值与条件构造
├── dialect.go             # Dialect 接口与注册表
├── errors.go              # DBX_* 错误码
├── logadapter.go          # logx 适配与慢查询日志
├── metrics.go             # 指标钩子
├── mysql/                 # MySQL 方言 + go-sql-driver/mysql
├── sqlite/                # SQLite 方言 + modernc.org/sqlite
├── pg/                    # PostgreSQL 方言 + pgx stdlib
├── migrate/               # 轻量迁移
├── confx/                 # (可选)confx 配置接入
├── examples/
└── docs/
```

## 11. 依赖策略

- 核心包:标准库 + errx + logx(自家库),**零第三方**;
- 方言子包:各自唯一的第三方驱动依赖;
- 禁止为核心功能引入额外第三方(如 UUID、重试库),与 logx / webx 的策略一致。
