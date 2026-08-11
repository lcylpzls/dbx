package dbx_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/lcylpzls/dbx"
)

// TestPublicAPI 黑盒冒烟测试：覆盖根包全部转发函数、类型别名与常量。
func TestPublicAPI(t *testing.T) {
	opts := []dbx.Option{
		dbx.WithMaxOpenConns(10),
		dbx.WithMaxIdleConns(5),
		dbx.WithConnMaxLifetime(time.Minute),
		dbx.WithConnMaxIdleTime(time.Minute),
		dbx.WithSlowQueryThreshold(time.Second),
		dbx.WithLogger(nil),
		dbx.WithLogSQL(true),
		dbx.WithLogArgs(true),
		dbx.WithMetrics(nil),
		dbx.WithTraceHook(nil),
		dbx.WithEventHook(nil),
	}
	if len(opts) == 0 {
		t.Fatal("Option 构造失败")
	}

	_, _ = dbx.Open(context.Background(), "sqlite", ":memory:", opts...)
	_, _ = dbx.OpenConfig(context.Background(), dbx.Config{}, opts...)
	_ = dbx.IsNotFound(nil)
	_ = dbx.IsDuplicate(nil)
	_ = dbx.Raw("SELECT 1", "a")
	_ = dbx.Select("SELECT * FROM t")
	dbx.RegisterDialect("smoke", nil)
	_, _ = dbx.One[int](context.Background(), fakeRunner{}, dbx.Raw("SELECT 1"))
	_, _ = dbx.OneWith[int](context.Background(), fakeRunner{}, dbx.Raw("SELECT 1"), func(dbx.Row) (int, error) { return 0, nil })
	_, _ = dbx.List[int](context.Background(), fakeRunner{}, dbx.Raw("SELECT 1"))
	_, _ = dbx.ListWith[int](context.Background(), fakeRunner{}, dbx.Raw("SELECT 1"), func(dbx.Row) (int, error) { return 0, nil })

	var _ dbx.TxOption = dbx.WithIsolation(sql.LevelSerializable)
	_ = dbx.WithReadOnly(true)

	_ = dbx.CodeOpenFailed
	_ = dbx.CodeDriverNotRegistered
	_ = dbx.CodeBadArgument
	_ = dbx.CodeExecFailed
	_ = dbx.CodeQueryFailed
	_ = dbx.CodeScanFailed
	_ = dbx.CodeNotFound
	_ = dbx.CodeTxBeginFailed
	_ = dbx.CodeTxCommitFailed
	_ = dbx.CodeTxRollbackFailed
	_ = dbx.CodeTxCallbackFailed
	_ = dbx.CodeCloseFailed
	_ = dbx.CodeDuplicate
	_ = dbx.CodeMigrationFailed

	var _ dbx.Tx
	var _ dbx.TraceAttr
	var _ dbx.TraceHook
	var _ dbx.Row
	var _ dbx.RowMapper[int]
	var _ dbx.QueryRunner
	var _ dbx.Query
	var _ dbx.SelectQuery
	var _ dbx.Config
	var _ dbx.Option
	var _ dbx.DB
	var _ dbx.Dialect
	var _ dbx.Metrics
	var _ dbx.QueryEvent
	var _ dbx.EventHook
}

// fakeRunner 是冒烟测试用的查询执行器（恒返回错误）。
type fakeRunner struct{}

func (fakeRunner) Query(context.Context, dbx.Query) (*sql.Rows, error) {
	return nil, errors.New("冒烟错误")
}
