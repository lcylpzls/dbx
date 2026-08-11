package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
)

// fake 是全局测试驱动,通过 set 控制各场景行为。
var fake = &fakeDriver{}

func init() {
	sql.Register("dbxtest", fake)
	sql.Register("sqlite", fake)
}

type fakeConfig struct {
	openErr              error
	pingErr              error
	execErr              error
	queryErr             error
	rowsErr              error
	beginTxErr           error
	commitErr            error
	rollbackErr          error
	prepareErr           error
	savepointBeginErr    error
	savepointRollbackErr error
	savepointReleaseErr  error
	closeErr             error
	columns              []string
	rows                 [][]driver.Value
	insertID             int64
	affected             int64
}

type fakeDriver struct {
	mu     sync.Mutex
	cfg    fakeConfig
	txOpts sql.TxOptions
}

func (d *fakeDriver) set(cfg fakeConfig) {
	d.mu.Lock()
	d.cfg = cfg
	d.mu.Unlock()
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	cfg := d.cfg
	d.mu.Unlock()
	if cfg.openErr != nil {
		return nil, cfg.openErr
	}
	return &fakeConn{cfg: cfg}, nil
}

func (d *fakeDriver) recordTxOptions(opts sql.TxOptions) {
	d.mu.Lock()
	d.txOpts = opts
	d.mu.Unlock()
}

func (d *fakeDriver) lastTxOptions() sql.TxOptions {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.txOpts
}

type fakeConn struct {
	cfg fakeConfig
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	if c.cfg.prepareErr != nil {
		return nil, c.cfg.prepareErr
	}
	return &fakeStmt{cfg: c.cfg, query: query}, nil
}

func (c *fakeConn) Close() error {
	return c.cfg.closeErr
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	return &fakeTx{cfg: c.cfg}, nil
}

func (c *fakeConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	fake.recordTxOptions(sql.TxOptions{
		Isolation: sql.IsolationLevel(opts.Isolation),
		ReadOnly:  opts.ReadOnly,
	})
	if c.cfg.beginTxErr != nil {
		return nil, c.cfg.beginTxErr
	}
	return &fakeTx{cfg: c.cfg}, nil
}

func (c *fakeConn) Ping(ctx context.Context) error {
	return c.cfg.pingErr
}

var _ driver.Pinger = (*fakeConn)(nil)
var _ driver.ConnBeginTx = (*fakeConn)(nil)

type fakeStmt struct {
	cfg   fakeConfig
	query string
}

func (s *fakeStmt) Close() error {
	return nil
}

func (s *fakeStmt) NumInput() int {
	return -1
}

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	switch {
	case strings.Contains(s.query, "SAVEPOINT ") && s.cfg.savepointBeginErr != nil:
		return nil, s.cfg.savepointBeginErr
	case strings.Contains(s.query, "ROLLBACK TO SAVEPOINT") && s.cfg.savepointRollbackErr != nil:
		return nil, s.cfg.savepointRollbackErr
	case strings.Contains(s.query, "RELEASE SAVEPOINT") && s.cfg.savepointReleaseErr != nil:
		return nil, s.cfg.savepointReleaseErr
	case s.cfg.execErr != nil:
		return nil, s.cfg.execErr
	}
	return fakeResult{insertID: s.cfg.insertID, affected: s.cfg.affected}, nil
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.cfg.queryErr != nil {
		return nil, s.cfg.queryErr
	}
	return &fakeRows{columns: s.cfg.columns, rows: s.cfg.rows, rowsErr: s.cfg.rowsErr}, nil
}

type fakeTx struct {
	cfg fakeConfig
}

func (t *fakeTx) Commit() error {
	return t.cfg.commitErr
}

func (t *fakeTx) Rollback() error {
	return t.cfg.rollbackErr
}

type fakeResult struct {
	insertID int64
	affected int64
}

func (r fakeResult) LastInsertId() (int64, error) {
	return r.insertID, nil
}

func (r fakeResult) RowsAffected() (int64, error) {
	return r.affected, nil
}

type fakeRows struct {
	columns []string
	rows    [][]driver.Value
	rowsErr error
	index   int
	errSent bool
}

func (r *fakeRows) Columns() []string {
	return r.columns
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		if r.rowsErr != nil && !r.errSent {
			r.errSent = true
			return r.rowsErr
		}
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
