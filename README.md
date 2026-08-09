# dbx

基于 `database/sql` 的薄数据访问层:统一、安全、可读、可观测,支持 MySQL、SQLite、PostgreSQL。

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.26-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

> 当前状态:**v0.1.0 定版**。规划与实现全部完成,进入发布流程。

## 定位

dbx **不是 ORM**,不解决「帮你写 SQL」的问题;它解决的是原生 SQL 开发中每个项目都要重复的部分:

- 连接管理、连接池配置与优雅关闭;
- 统一的事务边界与嵌套保存点;
- 结构体扫描(泛型为主,反射兜底);
- 动态查询的安全构造(参数占位 + 标识符白名单);
- 三种方言差异的最小化处理(占位符、分页);
- 与 logx / errx 打通的日志、错误与可观测性;
- 内置轻量迁移与 confx TOML 配置接入。

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/dbx/sqlite"
)

type User struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func main() {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, "app.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// 固定 SQL:可审查、可粘贴
	u, err := dbx.One[User](ctx, db, dbx.Raw(`SELECT id, name FROM users WHERE id = $1`, 42))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", u)

	// 动态查询:条件安全构造
	q := dbx.Select(`SELECT id, name FROM users`).
		Where(`name LIKE ?`, "%张%").
		OrderBy(`id`, false).
		Page(1, 20)
	users, err := dbx.List[User](ctx, db, q)
	if err != nil {
		panic(err)
	}
	_ = users
}
```

## 功能清单

- 🔌 连接:`Open` / `OpenConfig`,连接池参数、`Ping`、`Close`;
- 📝 查询执行:`Exec` / `Query` / `QueryRow`,参数一律占位符;
- 🔍 扫描:`One[T]` / `List[T]`(DB 与 Tx 通用),`db` tag、NULL 归零、字段元信息缓存、`RowMapper`;
- 🧱 动态构造:`Select` 条件(AND/OR/IN/LIKE/范围/IS NULL)、排序白名单、分页,占位符按方言转换;
- 🔁 事务:`WithTx` 自动提交/回滚(panic 兜底)、隔离级别与只读、嵌套保存点、
  `Tx.Exec` 返回 `sql.Result` 支持 `RowsAffected` 条件更新、`BatchExec`;
- 🪵 可观测:慢查询日志、SQL 打印开关、指标钩子,logx 由外部注入;
- 🏷️ 错误:`DBX_*` 错误码,`IsNotFound` / `IsDuplicate`(跨方言重复键)判定助手;
- 🗃️ 迁移:`dbx/migrate` 版本表、embed.FS、失败回滚;`dbx/confx` TOML 配置接入。

`dbx/sqlite` 支持连接级 PRAGMA 选项(`WithPragma("journal_mode", "WAL")` 等),
通过 DSN `_pragma` 参数保证连接池每个连接都生效。

## 方言子包

| 子包 | 驱动 | 说明 |
| --- | --- | --- |
| `dbx/mysql` | go-sql-driver/mysql | MySQL 5.7+ / 8.x |
| `dbx/sqlite` | modernc.org/sqlite | 纯 Go,无 CGO,支持内存库 |
| `dbx/pg` | jackc/pgx/v5 stdlib | PostgreSQL 12+,占位符 `$1..$n` |

## 示例

| 示例 | 说明 |
| --- | --- |
| [basic](examples/basic) | 连接、建表、`One` / `List` 扫描 |
| [dynamic](examples/dynamic) | 动态查询构造(条件、LIKE、排序、分页) |
| [tx](examples/tx) | 事务、嵌套保存点与整体回滚 |
| [migrate](examples/migrate) | embed 迁移与幂等执行 |

```sh
cd examples/basic && go run .
```

## 文档

- [docs/README.md](docs/README.md) — 文档索引与阅读顺序
- [docs/PRD.md](docs/PRD.md) — 产品需求
- [docs/architecture.md](docs/architecture.md) — 架构设计
- [docs/api-design.md](docs/api-design.md) — v0.1.0 API 设计
- [docs/api-v0.1.0.md](docs/api-v0.1.0.md) — API 基线
- [docs/iteration-plan.md](docs/iteration-plan.md) — 迭代计划与质量门槛
- [docs/decisions.md](docs/decisions.md) — 架构决策记录(ADR)

## License

MIT © [lcylpzls](https://github.com/lcylpzls)
