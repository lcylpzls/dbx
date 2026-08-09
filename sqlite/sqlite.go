// Package sqlite 提供 SQLite 驱动与方言接入。
// 导入本包即可注册 modernc.org/sqlite(纯 Go,无 CGO)。
package sqlite

import (
	"context"

	"github.com/lcylpzls/dbx"
	_ "modernc.org/sqlite"
)

// Open 打开 SQLite 数据库连接。
func Open(ctx context.Context, dsn string, opts ...dbx.Option) (*dbx.DB, error) {
	return dbx.Open(ctx, "sqlite", dsn, opts...)
}
