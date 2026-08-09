// Package sqlite 提供 SQLite 驱动与方言接入。
// 导入本包即可注册 modernc.org/sqlite(纯 Go,无 CGO)。
package sqlite

import (
	"context"
	"net/url"
	"strings"

	"github.com/lcylpzls/dbx"
	_ "modernc.org/sqlite"
)

// Option 是 sqlite 子包专属的打开选项。
type Option func(*options)

type options struct {
	pragmas []string // DSN 查询参数片段,形如 _pragma=name(value)
	dbxOpts []dbx.Option
}

// WithPragma 添加一个连接级 PRAGMA,如 WithPragma("journal_mode", "WAL")。
// 通过 DSN 的 _pragma 参数实现,连接池中的每个连接都会生效;
// 可多次调用,顺序即参数顺序。
func WithPragma(name, value string) Option {
	return func(o *options) {
		o.pragmas = append(o.pragmas, "_pragma="+url.QueryEscape(name+"("+value+")"))
	}
}

// WithDBOptions 透传 dbx 通用打开选项。
func WithDBOptions(opts ...dbx.Option) Option {
	return func(o *options) {
		o.dbxOpts = append(o.dbxOpts, opts...)
	}
}

// Open 打开 SQLite 数据库连接。
func Open(ctx context.Context, dsn string, opts ...Option) (*dbx.DB, error) {
	cfg := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return dbx.Open(ctx, "sqlite", mergeDSN(dsn, cfg.pragmas), cfg.dbxOpts...)
}

// mergeDSN 将 PRAGMA 查询参数合并进 DSN。
func mergeDSN(dsn string, params []string) string {
	if len(params) == 0 {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + strings.Join(params, "&")
}
