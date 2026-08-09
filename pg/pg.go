// Package pg 提供 PostgreSQL 驱动与方言接入。
// 导入本包即可注册 jackc/pgx/v5 的 database/sql 驱动。
package pg

import (
	"context"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lcylpzls/dbx"
)

// Open 打开 PostgreSQL 数据库连接。
func Open(ctx context.Context, dsn string, opts ...dbx.Option) (*dbx.DB, error) {
	return dbx.Open(ctx, "postgres", dsn, opts...)
}
