// Package confx 提供 dbx.Config 的 TOML 配置接入。
// 依赖 github.com/lcylpzls/confx,属于可选子包。
package confx

import (
	"context"
	"strings"
	"time"

	confxlib "github.com/lcylpzls/confx"
	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/errx"
)

// fileConfig 是 TOML 文件中的连接配置结构。
type fileConfig struct {
	Driver             string `toml:"driver"`
	DSN                string `toml:"dsn"`
	MaxOpenConns       int    `toml:"max_open_conns"`
	MaxIdleConns       int    `toml:"max_idle_conns"`
	ConnMaxLifetime    string `toml:"conn_max_lifetime"`
	ConnMaxIdleTime    string `toml:"conn_max_idle_time"`
	SlowQueryThreshold string `toml:"slow_query_threshold"`
	LogSQL             bool   `toml:"log_sql"`
	LogArgs            bool   `toml:"log_args"`
}

// LoadFile 从 TOML 文件解析 dbx.Config。
// 未声明的字段会因 confx 严格模式返回错误。
func LoadFile(path string) (dbx.Config, error) {
	cm, _ := confxlib.NewConfigManager(confxlib.Toml)
	var fc fileConfig
	if err := cm.Load(path, &fc); err != nil {
		return dbx.Config{}, err
	}
	connMaxLifetime, err := parseDuration(fc.ConnMaxLifetime)
	if err != nil {
		return dbx.Config{}, err
	}
	connMaxIdleTime, err := parseDuration(fc.ConnMaxIdleTime)
	if err != nil {
		return dbx.Config{}, err
	}
	slowQueryThreshold, err := parseDuration(fc.SlowQueryThreshold)
	if err != nil {
		return dbx.Config{}, err
	}
	return dbx.Config{
		Driver:             fc.Driver,
		DSN:                fc.DSN,
		MaxOpenConns:       fc.MaxOpenConns,
		MaxIdleConns:       fc.MaxIdleConns,
		ConnMaxLifetime:    connMaxLifetime,
		ConnMaxIdleTime:    connMaxIdleTime,
		SlowQueryThreshold: slowQueryThreshold,
		LogSQL:             fc.LogSQL,
		LogArgs:            fc.LogArgs,
	}, nil
}

// Open 从 TOML 文件加载配置并打开数据库连接。
func Open(ctx context.Context, path string, opts ...dbx.Option) (*dbx.DB, error) {
	cfg, err := LoadFile(path)
	if err != nil {
		return nil, err
	}
	return dbx.OpenConfig(ctx, cfg, opts...)
}

// parseDuration 解析可选的时长字符串,空串返回 0。
func parseDuration(s string) (time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errx.NewCodef(dbx.CodeBadArgument, "非法时长 %q", s)
	}
	return d, nil
}
