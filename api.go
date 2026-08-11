package dbx

import (
	"context"
	"database/sql"
	"time"

	"github.com/lcylpzls/dbx/internal/core"
	"github.com/lcylpzls/logx"
)

const (
	CodeOpenFailed          = core.CodeOpenFailed
	CodeDriverNotRegistered = core.CodeDriverNotRegistered
	CodeBadArgument         = core.CodeBadArgument
	CodeExecFailed          = core.CodeExecFailed
	CodeQueryFailed         = core.CodeQueryFailed
	CodeScanFailed          = core.CodeScanFailed
	CodeNotFound            = core.CodeNotFound
	CodeTxBeginFailed       = core.CodeTxBeginFailed
	CodeTxCommitFailed      = core.CodeTxCommitFailed
	CodeTxRollbackFailed    = core.CodeTxRollbackFailed
	CodeTxCallbackFailed    = core.CodeTxCallbackFailed
	CodeCloseFailed         = core.CodeCloseFailed
	CodeDuplicate           = core.CodeDuplicate
	CodeMigrationFailed     = core.CodeMigrationFailed
)

type (
	Tx               = core.Tx
	TxOption         = core.TxOption
	TraceAttr        = core.TraceAttr
	TraceHook        = core.TraceHook
	Row              = core.Row
	RowMapper[T any] = core.RowMapper[T]
	QueryRunner      = core.QueryRunner
	Query            = core.Query
	SelectQuery      = core.SelectQuery
	Config           = core.Config
	Option           = core.Option
	DB               = core.DB
	Dialect          = core.Dialect
	Metrics          = core.Metrics
	QueryEvent       = core.QueryEvent
	EventHook        = core.EventHook
)

func WithIsolation(level sql.IsolationLevel) TxOption { return core.WithIsolation(level) }
func WithReadOnly(readOnly bool) TxOption             { return core.WithReadOnly(readOnly) }
func WithMaxOpenConns(n int) Option                   { return core.WithMaxOpenConns(n) }
func WithMaxIdleConns(n int) Option                   { return core.WithMaxIdleConns(n) }
func WithConnMaxLifetime(d time.Duration) Option      { return core.WithConnMaxLifetime(d) }
func WithConnMaxIdleTime(d time.Duration) Option      { return core.WithConnMaxIdleTime(d) }
func WithSlowQueryThreshold(d time.Duration) Option   { return core.WithSlowQueryThreshold(d) }
func WithLogger(l logx.Logger) Option                 { return core.WithLogger(l) }
func WithLogSQL(enabled bool) Option                  { return core.WithLogSQL(enabled) }
func WithLogArgs(enabled bool) Option                 { return core.WithLogArgs(enabled) }
func WithMetrics(m Metrics) Option                    { return core.WithMetrics(m) }
func WithTraceHook(h TraceHook) Option                { return core.WithTraceHook(h) }
func WithEventHook(h EventHook) Option                { return core.WithEventHook(h) }
func Open(ctx context.Context, dialect, dsn string, opts ...Option) (*DB, error) {
	return core.Open(ctx, dialect, dsn, opts...)
}
func OpenConfig(ctx context.Context, cfg Config, opts ...Option) (*DB, error) {
	return core.OpenConfig(ctx, cfg, opts...)
}
func IsNotFound(err error) bool              { return core.IsNotFound(err) }
func IsDuplicate(err error) bool             { return core.IsDuplicate(err) }
func Raw(sql string, args ...any) Query      { return core.Raw(sql, args...) }
func Select(sql string) *SelectQuery         { return core.Select(sql) }
func RegisterDialect(name string, d Dialect) { core.RegisterDialect(name, d) }
func One[T any](ctx context.Context, runner QueryRunner, q Query) (T, error) {
	return core.One[T](ctx, runner, q)
}
func OneWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) (T, error) {
	return core.OneWith[T](ctx, runner, q, mapper)
}
func List[T any](ctx context.Context, runner QueryRunner, q Query) ([]T, error) {
	return core.List[T](ctx, runner, q)
}
func ListWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) ([]T, error) {
	return core.ListWith[T](ctx, runner, q, mapper)
}
