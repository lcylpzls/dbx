# 更新日志

## [v1.0.3] - 2026-08-10

### 变更

- 家族正式基线锁定：依赖统一指向 v1 基线已发布版本（errx v1.5.5 / logx v1.3.2 / testx v1.4.3 / validx v1.2.4 / cryptox v1.0.2 / confx v1.0.2 / webx v1.5.4 等），此后家族依赖不再前进。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v1.0.2] - 2026-08-10

### 变更

- 家族依赖最终对齐到 v1 正式版基线（errx v1.5.4 / logx v1.3.1 / testx v1.4.2 / validx v1.2.3 / confx v1.0.1 / cryptox v1.0.1 等），无 API 变更。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v1.0.1] - 2026-08-10

### 变更

- 家族依赖统一对齐到最新基线（errx v1.5.4 / logx v1.3.0 / testx v1.4.1 / validx v1.2.2 等），无 API 变更。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v1.0.0] - 2026-08-10

### 发布

- 家族正式版 v1.0.0：当前 API 与行为作为 v1 基线；按家族规则，v1.*.* 内允许破坏性修改，不承诺向后兼容。

### 质量

- 覆盖率、race、vet、staticcheck、fuzz、govulncheck 全绿；CI/Release 自动化发布。

## [v0.6.1] - 2026-08-10

### 变更

- 扫描时间解析错误统一 errx 化（CodeScanFailed），对外错误带结构化 code/kind，消息保持原语义。

### 质量

- 覆盖率维持基线；race / vet / staticcheck / govulncheck 全绿。

## [v0.6.0] - 2026-08-10

### 变更

- 配置校验统一迁移至家族 `validx`：注册 `dbx_config` 全局规则，调用点走 `validx.ValidateField`；
- errx 错误码保持 dbx 语义，行为不变。

### 质量

- 全部库包语句覆盖率保持 100%；race / vet / staticcheck / fuzz / govulncheck 全绿。

## [v0.5.0] - 2026-08-10

### 新增

- `EventHook` 查询事件钩子（零依赖可选接口，默认 no-op）：
  Exec / Query / QueryRow 结束时触发 `QueryEvent`
  （system / operation / statement / err），由 eventx 等外部适配器接入；
- `WithEventHook` 选项注入，与 TraceHook 同位置调用。

### 安全

- 升级 golang.org/x/text 至 v0.39.0，修复 GO-2026-5970。

### 质量

- 根包与子包覆盖率保持 100%；race / vet / staticcheck / fuzz /
  govulncheck 全绿。

## [v0.4.3] - 2026-08-10

### 变更

- go 指令与 CI/Release 工作流统一为 Go 1.26.5；
- README Go 版本徽章同步更新。

## [v0.4.2] - 2026-08-10

### 变更

- 家族统一 Go 1.21：全部 go.mod 与 CI/Release 工作流版本号对齐 1.21；
- testx 依赖升级 v1.2.1。

## [v0.4.1] - 2026-08-10

### 修复

- examples 全部示例模块 go.mod 与最新依赖对齐（go mod tidy），
  修复 main CI 示例构建失败。

## [v0.4.0] - 2026-08-10

### 变更

- migrate 示例命令行接入 `clix v1.2.0`：数据库路径改为全局 flag，
  统一帮助与退出码；
- 示例模块依赖同步升级（confx/logx/errx/validx）。

### 质量

- 示例构建 / vet / staticcheck 全绿。

## [v0.3.0] - 2026-08-10

### 变更

- 家族测试底座接入：根包与全部子包测试改用语义等价的
  testx 断言（含 Require* 致命断言）；
- 测试依赖新增 `testx v1.2.0`，errx 同步升级 v1.4.0。

### 质量

- 根包语句覆盖率 100%；race / vet / staticcheck 全绿。

## [v0.2.4] - 2026-08-10

### 新增

- `TraceHook` 链路追踪钩子（零依赖接口 + `WithTraceHook`）：
  Exec / Query / QueryRow 自动埋点（db.system / db.operation /
  db.statement 属性），由 tracex 等外部适配器接入；
- 追踪埋点测试，根包与子包覆盖率保持 100%。

本项目遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [v0.2.1] - 2026-08-09

### 变更

- 错误统一收尾:`Close` 失败包装为 `DBX_CLOSE_FAILED`;
  `WithTx` 回调返回的普通错误包装为 `DBX_TX_CALLBACK_FAILED`
  (`KindBusiness`,保留原始错误链,已是 errx 的错误保持原语义);
  `IsNotFound` 同时识别 `QueryRow` 原生 `sql.ErrNoRows`。

## [v0.2.0] - 2026-08-09

### 破坏性变更

- `Tx.Exec` 返回值从 `error` 改为 `(sql.Result, error)`,与 `DB.Exec` 对齐,
  支持事务内 `RowsAffected` 条件更新/乐观锁(issue #2)。

### 新增

- `IsDuplicate` / `CodeDuplicate`(`errx.KindConflict`):跨 MySQL / SQLite /
  PostgreSQL 统一识别唯一约束/重复键冲突,应用于 `Exec` / `Tx.Exec` /
  `BatchExec`,保留原始错误链(issue #3);
- `dbx/sqlite` 新增 `WithPragma` / `WithDBOptions` 打开选项:
  连接级 PRAGMA 按顺序合并进 DSN `_pragma` 参数,连接池每个连接生效(issue #4);
- CI 接入 apidiff 检查(v0.1.0 → HEAD,informational 不设门禁)。

### 关闭

- issue #1(`Raw()` 底层访问器):不做向下兼容窗口,能力走 dbx API。

## [v0.1.2] - 2026-08-09

### 变更

- CI 新增 `bench` job:每次运行输出性能基准日志并上传 artifact
  (`bench-log`),只记录基线、不设硬性门禁。

## [v0.1.1] - 2026-08-09

### 变更

- 建立性能基准(`docs/performance.md`):`Raw` 构建 0 分配达标;
  `Select` 构建从 26 降至 15 次分配(去指针化、切片预分配、
  占位符转换快速路径、渲染器预扩容);`One` / `List` 扫描记录基线,
  `List` 分配随行数线性(≈3.2 分配/行)。

## [v0.1.0] - 2026-08-09

### 规划与决策

- 完成产品需求(PRD)、架构设计、API 草案、迭代计划与架构决策记录(ADR);
- 确定定位:基于 `database/sql` 的薄数据访问层,支持 MySQL / SQLite / PostgreSQL;
- D1–D8 全部确认(均采用推荐项),API 草案冻结为 v0.1.0 基线。

### 新增

- P0 项目骨架:go.mod、CI 工作流、错误码注册(DBX_*)与 `IsNotFound`;
- P1 连接与查询执行:`DB` / `Open` / `OpenConfig`、连接池参数、`Ping` / `Close`,
  `Raw` 查询值与 `Exec` / `Query` / `QueryRow` 三件套;
- P2 扫描:`One[T]` / `List[T]` / `OneWith[T]` / `ListWith[T]`(包级泛型函数),
  `db` tag、嵌入结构体、NULL 归零、字段元信息缓存与 `RowMapper`,
  无数据统一返回 `DBX_NOT_FOUND`,`FuzzScan` 覆盖转换路径;
- P3 动态构造:`SelectQuery`(WHERE/AND/OR、IN、LIKE、范围、IS NULL、
  排序白名单、分页)、方言占位符转换与标识符引用,`FuzzBuilder` 保证
  任意输入不产生可注入 SQL;同步修正签名:`Query.SQL()` 返回错误、
  `QueryRow` 返回 `(*sql.Row, error)`、`Dialect.LimitOffset` 带起始序号;
- P4 方言与驱动子包:`mysql` / `sqlite` / `pg` 各自导入唯一第三方驱动
  (go-sql-driver/mysql、modernc.org/sqlite、jackc/pgx/v5 stdlib)并提供
  `Open` 快捷入口,核心包保持零第三方驱动;CI 增加 MySQL 8.4 与
  PostgreSQL 16 服务容器集成测试,SQLite 在进程内运行,同一套基础场景
  在三种方言上执行;fuzz 目标扩展为 errors / scan / builder 三个;
- P5 事务与批量:`WithTx`(自动提交/回滚,panic 兜底,隔离级别与只读选项)、
  嵌套保存点 `Nested`(SAVEPOINT / ROLLBACK TO / RELEASE,编号递增)、
  `BatchExec`(预编译复用);`Tx` 提供与 `DB` 相同的执行与扫描入口,
  新增导出 `QueryRunner` 接口,`One[T]` / `List[T]` 可同时作用于 DB 与 Tx;
- P6 可观测性:logx 对象由外部注入(`WithLogger`),包内只使用;
  新增开关 `WithLogSQL`(打印 SQL)、`WithLogArgs`(附带参数)、
  `WithMetrics`(指标钩子);`Exec` / `Query` / `QueryRow` / `BatchExec`
  统一埋点:查询/错误/慢查询计数、耗时观测、慢查询日志(默认阈值 100ms,
  SQL 截断 512 字符);事务内执行同样埋点;
- P7 迁移与 confx 接入:`dbx/migrate` 按文件名顺序执行 `*.sql`
  (支持多条语句拆分,跳过字符串/注释中的分号),版本表三方言兼容,
  每个迁移文件单个事务执行、失败整体回滚、重复执行幂等;
  `dbx/confx` 可选子包把 TOML 配置解析为 `dbx.Config`(时长字符串、
  严格未知字段),并提供 `Open` 快捷入口;
  `SelectQuery` 新增 `Args` 方法,支持跨方言参数化 INSERT / UPDATE;
- P8 示例与文档:examples/basic / dynamic / tx / migrate(独立模块 +
  replace),README 收尾,API 基线 `docs/api-v0.1.0.md`,CHANGELOG 定版。

### 质量

- 全模块(核心 + 5 个子包 + 示例)语句覆盖率 100%,
  `go test -race ./...`、`go vet`、`staticcheck` 全绿;
- fuzz:errors / scan / builder 三个目标;
- CI:三平台 × Go 1.26,MySQL / PostgreSQL 服务容器集成测试;
- 注:apidiff 检查在 v0.1.0 发布后接入(当前 apidiff 模块模式不可用)。
