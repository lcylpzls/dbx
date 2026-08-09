// Package mysql 提供 MySQL 驱动与方言接入。
// 导入本包即可注册 go-sql-driver/mysql,并通过 Open 快捷入口打开连接。
package mysql

import (
	"context"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lcylpzls/dbx"
)

// Open 打开 MySQL 数据库连接。
func Open(ctx context.Context, dsn string, opts ...dbx.Option) (*dbx.DB, error) {
	return dbx.Open(ctx, "mysql", dsn, opts...)
}
