# dbx 迭代计划与质量门槛

## 1. 迭代阶段

### P0 项目骨架

- `go.mod`(module github.com/lcylpzls/dbx,go 1.26);
- 目录结构、`LICENSE`、`.gitignore`、CI 工作流(三平台矩阵 + staticcheck + fuzz);
- 依赖引入:errx v1.2.0、logx v1.0.0;
- 错误码注册 `errors.go` 与空包测试。

**验收**:空包测试通过,CI 全绿。

### P1 连接与查询执行

- `dbx.go`:`DB`、`Open` / `OpenConfig`、连接池参数、`Ping` / `Close`;
- `builder.go`:`Raw` 查询值;
- `Exec` / `Query` / `QueryRow` 三件套。

**验收**:连接参数、错误包装全路径测试,100% 覆盖率。

### P2 扫描

- `scan.go`:`One[T]` / `List[T]`、`db` tag、嵌入结构体、NULL 处理;
- 字段元信息缓存与 `RowMapper`。

**验收**:SQLite 内存库全场景测试(含 NULL、类型不匹配、空结果),`FuzzScan` 通过。

### P3 动态构造

- `SelectQuery`:条件、IN、LIKE、范围、IS NULL、排序白名单、分页;
- 占位符按方言转换,参数顺序严格对应。

**验收**:`FuzzBuilder` 证明任意输入不产生可注入 SQL;标识符白名单专项测试。

### P4 方言与驱动子包

- `dialect.go` 注册表;
- `mysql` / `sqlite` / `pg` 三个子包;
- 三数据库集成测试(CI 服务容器 + SQLite 进程内)。

**验收**:同一套测试代码在三种数据库上全部通过。

### P5 事务与批量

- `tx.go`:`WithTx`、隔离级别、嵌套保存点、`BatchExec`。

**验收**:提交/回滚/panic 兜底/保存点回滚专项测试,race 全绿。

### P6 可观测性

- `logadapter.go`:慢查询日志(阈值、SQL 截断、参数输出);
- `metrics.go`:计数与耗时钩子;
- 错误码收尾与 `IsNotFound`。

**验收**:慢查询触发/不触发、指标计数全路径测试。

### P7 迁移与配置接入

- `migrate`:版本表、embed.FS、失败回滚;
- `confx` 可选子包:TOML 配置到 `dbx.Config`。

**验收**:重复执行幂等、中途失败回滚测试通过。

### P8 示例、文档与发布 v0.1.0

- `examples/`:basic、dynamic、tx、migrate;
- README 与 docs 收尾;API 基线 `docs/api-v0.1.0.md`;CHANGELOG 定版;
- tag v0.1.0,Release 工作流。

## 2. 质量门槛(每个阶段强制)

- 语句覆盖率 **100%**(`go test -cover`);
- `go vet ./...`、`staticcheck ./...` 零告警;
- `go test -race ./...` 全绿;
- fuzz:构造器、扫描、占位符转换至少各 1 个目标(CI 10s 短跑);
- 三平台 CI:ubuntu / windows / macos × Go 1.26.x;
- 集成测试:ubuntu 上跑 MySQL + PostgreSQL 服务容器,SQLite 进程内;
- v0.1.0 起维护 API 基线,CI 增加 apidiff 检查;
- 所有日志、注释、文档使用简体中文。

## 3. 依赖策略

- 核心包:标准库 + errx + logx,零第三方;
- 方言子包:各自唯一的第三方驱动:
  - `mysql`:github.com/go-sql-driver/mysql;
  - `sqlite`:modernc.org/sqlite;
  - `pg`:github.com/jackc/pgx/v5(stdlib);
- `confx` 子包依赖 confx(自家库);
- 禁止为小功能引入第三方(如 UUID、重试、连接池自研)。

## 4. 性能基准(目标,实现后建立基线)

| 场景 | 目标 |
| --- | --- |
| `Raw` 查询值构建 | 0 分配 |
| `Select` 三个条件 + 分页构建 | ≤ 2 次分配 |
| `One[T]` 扫描(5 字段结构体) | ≤ 3 次分配,无重复反射 |
| `List[T]` 扫描 100 行 | 分配与行数线性,常数小 |

## 5. 风险与对策

| 风险 | 对策 |
| --- | --- |
| 三方言集成测试依赖外部服务 | CI 用服务容器固定版本;本机开发以 SQLite 为主 |
| 100% 覆盖率对数据库代码成本高 | 使用 SQLite 内存库覆盖核心路径,方言差异用表驱动 |
| 构造器被滥用退化成 ORM | 代码评审 + API 基线;文档明确「SQL 主体手写」 |
| modernc.org/sqlite 体积较大 | 只影响导入 sqlite 子包的用户,核心包不受影响 |
