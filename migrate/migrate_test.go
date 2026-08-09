package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/dbx/sqlite"
	"github.com/lcylpzls/errx"
)

func openSQLite(t *testing.T) *dbx.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migrate-test.db")
	db, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunAppliesAll(t *testing.T) {
	db := openSQLite(t)
	fsys := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(
			`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`)},
		"002_seed.sql": &fstest.MapFile{Data: []byte(
			`INSERT INTO users (id, name) VALUES (1, '张三');`)},
	}
	if err := Run(context.Background(), db, fsys); err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	row, err := db.QueryRow(context.Background(),
		dbx.Select(`SELECT name FROM users`).Where(`id = ?`, int64(1)))
	if err != nil {
		t.Fatalf("QueryRow 失败：%v", err)
	}
	var name string
	if err := row.Scan(&name); err != nil || name != "张三" {
		t.Fatalf("迁移数据不符：%q, %v", name, err)
	}
	versions, err := appliedVersions(context.Background(), db)
	if err != nil {
		t.Fatalf("读取版本失败：%v", err)
	}
	if len(versions) != 2 || !versions["001_init"] || !versions["002_seed"] {
		t.Errorf("版本记录不符：%v", versions)
	}
}

func TestRunIdempotent(t *testing.T) {
	db := openSQLite(t)
	fsys := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(
			`CREATE TABLE users (id INTEGER PRIMARY KEY);`)},
	}
	if err := Run(context.Background(), db, fsys); err != nil {
		t.Fatalf("第一次 Run 失败：%v", err)
	}
	if err := Run(context.Background(), db, fsys); err != nil {
		t.Fatalf("第二次 Run 失败：%v", err)
	}
	versions, err := appliedVersions(context.Background(), db)
	if err != nil {
		t.Fatalf("读取版本失败：%v", err)
	}
	if len(versions) != 1 {
		t.Errorf("重复执行不应重复记录：%v", versions)
	}
}

func TestRunSkipsAppliedAndAppliesNew(t *testing.T) {
	db := openSQLite(t)
	base := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(
			`CREATE TABLE users (id INTEGER PRIMARY KEY);`)},
	}
	if err := Run(context.Background(), db, base); err != nil {
		t.Fatalf("Run 失败：%v", err)
	}
	base["002_more.sql"] = &fstest.MapFile{Data: []byte(
		`CREATE TABLE logs (id INTEGER PRIMARY KEY);`)}
	if err := Run(context.Background(), db, base); err != nil {
		t.Fatalf("第二次 Run 失败：%v", err)
	}
	versions, err := appliedVersions(context.Background(), db)
	if err != nil {
		t.Fatalf("读取版本失败：%v", err)
	}
	if len(versions) != 2 || !versions["002_more"] {
		t.Errorf("应跳过已应用版本并应用新版本：%v", versions)
	}
}

func TestRunFailureRollsBack(t *testing.T) {
	db := openSQLite(t)
	fsys := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(
			`CREATE TABLE t1 (id INTEGER PRIMARY KEY);`)},
		"002_bad.sql": &fstest.MapFile{Data: []byte(
			`CREATE TABLE t2 (id INTEGER); INSERT INTO missing_table (id) VALUES (1);`)},
	}
	err := Run(context.Background(), db, fsys)
	if err == nil || !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("失败迁移错误码不符：%v", err)
	}
	row, err := db.QueryRow(context.Background(), dbx.Select(`SELECT COUNT(*) FROM t2`))
	if err != nil {
		t.Fatalf("QueryRow 失败：%v", err)
	}
	var count int64
	if scanErr := row.Scan(&count); scanErr == nil {
		t.Fatal("失败迁移的建表不应保留")
	}
	versions, err := appliedVersions(context.Background(), db)
	if err != nil {
		t.Fatalf("读取版本失败：%v", err)
	}
	if len(versions) != 1 || !versions["001_init"] {
		t.Errorf("失败迁移不应记录版本：%v", versions)
	}
}

func TestRunEmpty(t *testing.T) {
	db := openSQLite(t)
	fsys := fstest.MapFS{
		"notes.txt": &fstest.MapFile{Data: []byte("忽略")},
		"dir.sql/":  &fstest.MapFile{},
	}
	if err := Run(context.Background(), db, fsys); err != nil {
		t.Fatalf("空迁移 Run 失败：%v", err)
	}
	versions, err := appliedVersions(context.Background(), db)
	if err != nil {
		t.Fatalf("读取版本失败：%v", err)
	}
	if len(versions) != 0 {
		t.Errorf("空迁移不应有版本：%v", versions)
	}
}

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"multiple", "SELECT 1; SELECT 2", []string{"SELECT 1", "SELECT 2"}},
		{"single quotes", `SELECT 'a;b'; SELECT 2`, []string{"SELECT 'a;b'", "SELECT 2"}},
		{"double quotes", `SELECT "a;b"; SELECT 2`, []string{`SELECT "a;b"`, "SELECT 2"}},
		{"backtick", "SELECT `a;b`; SELECT 2", []string{"SELECT `a;b`", "SELECT 2"}},
		{"line comment", "-- 注释; SELECT 1", []string{"-- 注释; SELECT 1"}},
		{"line comment newline", "-- 注释\nSELECT 1; SELECT 2", []string{"-- 注释\nSELECT 1", "SELECT 2"}},
		{"block comment", "/* 注释; */ SELECT 1; SELECT 2", []string{"/* 注释; */ SELECT 1", "SELECT 2"}},
		{"trailing semicolon", "SELECT 1;", []string{"SELECT 1"}},
		{"no semicolon", "SELECT 1", []string{"SELECT 1"}},
		{"empty", "  ;  ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitStatements(tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("语句数不符：got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("语句 %d 不符：got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// --- 错误分支:使用测试假驱动 ---

var migFake = &migFakeDriver{}

func init() {
	sql.Register("dbxmigratetest", migFake)
}

type migFakeConfig struct {
	execErr   error
	queryErr  error
	beginErr  error
	insertErr error
	versions  []string
}

type migFakeDriver struct {
	mu  sync.Mutex
	cfg migFakeConfig
}

func (d *migFakeDriver) set(cfg migFakeConfig) {
	d.mu.Lock()
	d.cfg = cfg
	d.mu.Unlock()
}

func (d *migFakeDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	cfg := d.cfg
	d.mu.Unlock()
	return &migFakeConn{cfg: cfg}, nil
}

type migFakeConn struct {
	cfg migFakeConfig
}

func (c *migFakeConn) Prepare(query string) (driver.Stmt, error) {
	return &migFakeStmt{cfg: c.cfg, query: query}, nil
}

func (c *migFakeConn) Close() error {
	return nil
}

func (c *migFakeConn) Begin() (driver.Tx, error) {
	return &migFakeTx{}, nil
}

func (c *migFakeConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.cfg.beginErr != nil {
		return nil, c.cfg.beginErr
	}
	return &migFakeTx{}, nil
}

func (c *migFakeConn) Ping(ctx context.Context) error {
	return nil
}

var _ driver.Pinger = (*migFakeConn)(nil)
var _ driver.ConnBeginTx = (*migFakeConn)(nil)

type migFakeStmt struct {
	cfg   migFakeConfig
	query string
}

func (s *migFakeStmt) Close() error {
	return nil
}

func (s *migFakeStmt) NumInput() int {
	return -1
}

func (s *migFakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	if strings.Contains(s.query, "INSERT INTO schema_migrations") && s.cfg.insertErr != nil {
		return nil, s.cfg.insertErr
	}
	if s.cfg.execErr != nil {
		return nil, s.cfg.execErr
	}
	return migFakeResult{}, nil
}

func (s *migFakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.cfg.queryErr != nil {
		return nil, s.cfg.queryErr
	}
	return &migFakeRows{versions: s.cfg.versions}, nil
}

type migFakeResult struct{}

func (migFakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (migFakeResult) RowsAffected() (int64, error) {
	return 0, nil
}

type migFakeRows struct {
	versions []string
	index    int
}

func (r *migFakeRows) Columns() []string {
	return []string{"version"}
}

func (r *migFakeRows) Close() error {
	return nil
}

func (r *migFakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.versions) {
		return io.EOF
	}
	dest[0] = r.versions[r.index]
	r.index++
	return nil
}

type migFakeTx struct{}

func (migFakeTx) Commit() error   { return nil }
func (migFakeTx) Rollback() error { return nil }

func openMigFake(t *testing.T) *dbx.DB {
	t.Helper()
	db, err := dbx.Open(context.Background(), "dbxmigratetest", "x")
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunGlobError(t *testing.T) {
	migFake.set(migFakeConfig{})
	db := openMigFake(t)
	err := Run(context.Background(), db, errFS{})
	if !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("Glob 失败错误码不符：%v", err)
	}
}

func TestRunReadFileError(t *testing.T) {
	migFake.set(migFakeConfig{})
	db := openMigFake(t)
	entries, _ := fs.ReadDir(fstest.MapFS{
		"001_x.sql": &fstest.MapFile{Data: []byte("SELECT 1")},
	}, ".")
	err := Run(context.Background(), db, dirOnlyFS{entries: entries})
	if !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("ReadFile 失败错误码不符：%v", err)
	}
}

func TestRunEnsureTableFailed(t *testing.T) {
	migFake.set(migFakeConfig{execErr: errors.New("建表失败")})
	db := openMigFake(t)
	err := Run(context.Background(), db, fstest.MapFS{})
	if !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("建表失败错误码不符：%v", err)
	}
}

func TestRunAppliedQueryFailed(t *testing.T) {
	migFake.set(migFakeConfig{queryErr: errors.New("查询失败")})
	db := openMigFake(t)
	err := Run(context.Background(), db, fstest.MapFS{})
	if !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("查询版本失败错误码不符：%v", err)
	}
}

func TestRunInsertVersionFailed(t *testing.T) {
	migFake.set(migFakeConfig{insertErr: errors.New("记录失败")})
	db := openMigFake(t)
	fsys := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte("SELECT 1")},
	}
	err := Run(context.Background(), db, fsys)
	if !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("记录版本失败错误码不符：%v", err)
	}
}

func TestRunBeginFailed(t *testing.T) {
	migFake.set(migFakeConfig{beginErr: errors.New("开启事务失败")})
	db := openMigFake(t)
	fsys := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte("SELECT 1")},
	}
	err := Run(context.Background(), db, fsys)
	if !errx.Is(err, dbx.CodeMigrationFailed) {
		t.Fatalf("开启事务失败错误码不符：%v", err)
	}
}

// errFS 使 fs.Glob 失败。
type errFS struct{}

func (errFS) Open(name string) (fs.File, error) {
	return nil, errors.New("目录不可读")
}

// dirOnlyFS 只允许列出目录,读取文件时失败。
type dirOnlyFS struct {
	entries []fs.DirEntry
}

func (f dirOnlyFS) Open(name string) (fs.File, error) {
	if name == "." {
		return &listDir{entries: f.entries}, nil
	}
	return nil, errors.New("文件不可读")
}

type listDir struct {
	entries []fs.DirEntry
}

func (d *listDir) ReadDir(n int) ([]fs.DirEntry, error) {
	return d.entries, nil
}

func (d *listDir) Read(p []byte) (int, error) {
	return 0, fs.ErrInvalid
}

func (d *listDir) Stat() (fs.FileInfo, error) {
	return nil, fs.ErrInvalid
}

func (d *listDir) Close() error {
	return nil
}
