package core

import (
	"fmt"
	"time"

	"github.com/lcylpzls/logx"
)

// defaultSlowThreshold 是默认慢查询阈值。
const defaultSlowThreshold = 100 * time.Millisecond

// maxSQLLength 是 SQL 日志截断长度(按字符计)。
const maxSQLLength = 512

// observe 统一记录一次执行的指标、SQL 日志与慢查询日志。
func observe(cfg Config, op, sqlText string, args any, start time.Time, err error) {
	duration := time.Since(start)
	threshold := cfg.SlowQueryThreshold
	if threshold <= 0 {
		threshold = defaultSlowThreshold
	}
	isSlow := duration >= threshold

	if cfg.Metrics != nil {
		cfg.Metrics.IncCounter("dbx.queries", []string{op})
		cfg.Metrics.ObserveDuration("dbx.duration", duration.Seconds(), []string{op})
		if err != nil {
			cfg.Metrics.IncCounter("dbx.errors", []string{op})
		}
		if isSlow {
			cfg.Metrics.IncCounter("dbx.slow_queries", []string{op})
		}
	}
	if cfg.Logger == nil {
		return
	}
	if cfg.LogSQL {
		fields := []logx.Field{
			logx.String("op", op),
			logx.String("sql", truncateSQL(sqlText)),
		}
		if cfg.LogArgs {
			fields = append(fields, logx.String("args", formatArgs(args)))
		}
		cfg.Logger.Debug("执行 SQL", logx.Fields(fields...))
	}
	if isSlow {
		fields := []logx.Field{
			logx.String("op", op),
			logx.String("sql", truncateSQL(sqlText)),
			logx.String("duration", duration.String()),
		}
		if cfg.LogArgs {
			fields = append(fields, logx.String("args", formatArgs(args)))
		}
		cfg.Logger.Warn("慢查询", logx.Fields(fields...))
	}
}

// truncateSQL 将 SQL 截断到 maxSQLLength 字符,超出部分以省略号结尾。
func truncateSQL(sqlText string) string {
	if len(sqlText) <= maxSQLLength {
		return sqlText
	}
	runes := []rune(sqlText)
	if len(runes) <= maxSQLLength {
		return sqlText
	}
	return string(runes[:maxSQLLength]) + "..."
}

// formatArgs 格式化 SQL 参数用于日志输出。
func formatArgs(args any) string {
	return fmt.Sprintf("%v", args)
}
