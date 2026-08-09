package dbx

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lcylpzls/errx"
)

// Tx 是数据库事务,持有 *sql.Tx 与方言。
type Tx struct {
	sqlTx   *sql.Tx
	dialect Dialect
	cfg     Config
	next    *int // 保存点计数器,由根事务持有
}

// TxOption 是事务选项。
type TxOption func(*sql.TxOptions)

// WithIsolation 设置事务隔离级别。
func WithIsolation(level sql.IsolationLevel) TxOption {
	return func(o *sql.TxOptions) { o.Isolation = level }
}

// WithReadOnly 设置只读事务。
func WithReadOnly(readOnly bool) TxOption {
	return func(o *sql.TxOptions) { o.ReadOnly = readOnly }
}

// WithTx 开启事务并执行回调,回调返回错误或 panic 时自动回滚。
func (db *DB) WithTx(ctx context.Context, fn func(*Tx) error, opts ...TxOption) error {
	txOpts := &sql.TxOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(txOpts)
		}
	}
	sqlTx, err := db.sqlDB.BeginTx(ctx, txOpts)
	if err != nil {
		return errx.Wrap(err, errx.KindUnavailable, CodeTxBeginFailed, "开启事务失败")
	}
	next := 0
	tx := &Tx{sqlTx: sqlTx, dialect: db.dialect, cfg: db.cfg, next: &next}
	committed := false
	rolledBack := false
	defer func() {
		if !committed && !rolledBack {
			_ = sqlTx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		rolledBack = true
		if rbErr := sqlTx.Rollback(); rbErr != nil {
			return errx.Wrap(rbErr, errx.KindUnavailable, CodeTxRollbackFailed, "回滚事务失败")
		}
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return errx.Wrap(err, errx.KindUnavailable, CodeTxCommitFailed, "提交事务失败")
	}
	committed = true
	return nil
}

// Exec 在事务内执行 SQL,返回影响行数等信息。
func (tx *Tx) Exec(ctx context.Context, q Query) (sql.Result, error) {
	sqlText, args, err := renderQuery(q, tx.dialect)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	res, err := tx.sqlTx.ExecContext(ctx, sqlText, args...)
	observe(tx.cfg, "exec", sqlText, args, start, err)
	if err != nil {
		return nil, wrapExecError(err)
	}
	return res, nil
}

// Query 在事务内执行查询。
func (tx *Tx) Query(ctx context.Context, q Query) (*sql.Rows, error) {
	sqlText, args, err := renderQuery(q, tx.dialect)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	rows, err := tx.sqlTx.QueryContext(ctx, sqlText, args...)
	observe(tx.cfg, "query", sqlText, args, start, err)
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeQueryFailed, "查询失败")
	}
	return rows, nil
}

// QueryRow 在事务内执行单行查询,构造失败直接返回错误,其余错误延迟到 Scan。
func (tx *Tx) QueryRow(ctx context.Context, q Query) (*sql.Row, error) {
	sqlText, args, err := renderQuery(q, tx.dialect)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	row := tx.sqlTx.QueryRowContext(ctx, sqlText, args...)
	observe(tx.cfg, "query_row", sqlText, args, start, nil)
	return row, nil
}

// Nested 在事务内开启保存点嵌套事务,
// 回调返回错误或 panic 时回滚到保存点并释放。
func (tx *Tx) Nested(ctx context.Context, fn func(*Tx) error) (err error) {
	name := tx.savepointName()
	if _, err := tx.sqlTx.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
		return errx.Wrap(err, errx.KindUnavailable, CodeTxBeginFailed, "创建保存点失败")
	}
	child := &Tx{sqlTx: tx.sqlTx, dialect: tx.dialect, cfg: tx.cfg, next: tx.next}
	released := false
	defer func() {
		if !released {
			_, _ = tx.sqlTx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
			_, _ = tx.sqlTx.ExecContext(ctx, "RELEASE SAVEPOINT "+name)
		}
	}()
	if err := fn(child); err != nil {
		if _, rbErr := tx.sqlTx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name); rbErr != nil {
			return errx.Wrap(rbErr, errx.KindUnavailable, CodeTxRollbackFailed, "回滚保存点失败")
		}
		if _, relErr := tx.sqlTx.ExecContext(ctx, "RELEASE SAVEPOINT "+name); relErr != nil {
			return errx.Wrap(relErr, errx.KindUnavailable, CodeTxRollbackFailed, "释放保存点失败")
		}
		released = true
		return err
	}
	if _, relErr := tx.sqlTx.ExecContext(ctx, "RELEASE SAVEPOINT "+name); relErr != nil {
		return errx.Wrap(relErr, errx.KindUnavailable, CodeTxRollbackFailed, "释放保存点失败")
	}
	released = true
	return nil
}

// savepointName 生成并递增保存点名称。
func (tx *Tx) savepointName() string {
	n := *tx.next
	*tx.next = n + 1
	return fmt.Sprintf("dbx_%d", n)
}

// BatchExec 使用预编译语句批量执行固定 SQL。
func (db *DB) BatchExec(ctx context.Context, sqlText string, args [][]any) error {
	return batchExec(ctx, db.sqlDB, db.cfg, sqlText, args)
}

// BatchExec 在事务内使用预编译语句批量执行固定 SQL。
func (tx *Tx) BatchExec(ctx context.Context, sqlText string, args [][]any) error {
	return batchExec(ctx, tx.sqlTx, tx.cfg, sqlText, args)
}

// preparer 是预编译语句的最小接口。
type preparer interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

// batchExec 预编译一次并逐组执行参数。
func batchExec(ctx context.Context, prep preparer, cfg Config, sqlText string, args [][]any) error {
	start := time.Now()
	stmt, err := prep.PrepareContext(ctx, sqlText)
	if err != nil {
		observe(cfg, "batch", sqlText, args, start, err)
		return errx.Wrap(err, errx.KindUnavailable, CodeExecFailed, "预编译语句失败")
	}
	defer stmt.Close()
	for _, argSet := range args {
		if _, err := stmt.ExecContext(ctx, argSet...); err != nil {
			observe(cfg, "batch", sqlText, args, start, err)
			return wrapExecError(err)
		}
	}
	observe(cfg, "batch", sqlText, args, start, nil)
	return nil
}
