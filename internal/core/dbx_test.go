package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	testx "github.com/lcylpzls/testx"
	"testing"
	"time"

	"github.com/lcylpzls/errx"
)

func TestRaw(t *testing.T) {
	q := Raw(`SELECT 1`)
	sqlText, args, err := q.SQL()
	testx.RequireNoError(t, err)

	if sqlText != `SELECT 1` || len(args) != 0 {
		t.Errorf("Raw 无参数结果不符：%q %v", sqlText, args)
	}
	q = Raw(`SELECT $1, $2`, 1, "x")
	sqlText, args, err = q.SQL()
	testx.RequireNoError(t, err)

	if sqlText != `SELECT $1, $2` || len(args) != 2 || args[0] != 1 || args[1] != "x" {
		t.Errorf("Raw 带参数结果不符：%q %v", sqlText, args)
	}
}

func TestOpenSuccess(t *testing.T) {
	fake.set(fakeConfig{})
	ctx := context.Background()
	db, err := Open(ctx, "dbxtest", "test")
	testx.RequireNoError(t, err)

	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Errorf("Ping 失败：%v", err)
	}
}

func TestOpenKnownDialect(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "sqlite", ":memory:")
	testx.RequireNoError(t, err)

	if err := db.Close(); err != nil {
		t.Errorf("Close 失败：%v", err)
	}
}

func TestOpenDriverNotRegistered(t *testing.T) {
	for _, dialect := range []string{"mysql", "nope"} {
		_, err := Open(context.Background(), dialect, "x")
		if !errx.Is(err, CodeDriverNotRegistered) || errx.KindOf(err) != errx.KindInvalid {
			t.Errorf("Open(%q) 错误码/分类不符：%v", dialect, err)
		}
	}
	_, err := OpenConfig(context.Background(), Config{Driver: "nope", DSN: "x"})
	if !errx.Is(err, CodeDriverNotRegistered) {
		t.Errorf("OpenConfig 未知方言错误码不符：%v", err)
	}
}

func TestOpenInvalidConfig(t *testing.T) {
	_, err := Open(context.Background(), "", "")
	if !errx.Is(err, CodeBadArgument) || errx.KindOf(err) != errx.KindInvalid {
		t.Errorf("Open 空配置错误码/分类不符：%v", err)
	}
}

func TestOpenPingFailure(t *testing.T) {
	fake.set(fakeConfig{pingErr: errors.New("连接失败")})
	_, err := Open(context.Background(), "dbxtest", "x")
	if !errx.Is(err, CodeOpenFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("Open Ping 失败错误码/分类不符：%v", err)
	}
}

func TestOpenSQLOpenFailure(t *testing.T) {
	_, err := open(context.Background(), "not-registered", Config{Driver: "x", DSN: "x"})
	if !errx.Is(err, CodeOpenFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("sql.Open 失败错误码/分类不符：%v", err)
	}
}

func TestOpenConfigSuccess(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := OpenConfig(context.Background(), Config{
		Driver: "dbxtest", DSN: "x", MaxOpenConns: 5,
	}, nil, WithMaxOpenConns(5))
	testx.RequireNoError(t, err)

	defer db.Close()
	if got := db.sqlDB.Stats().MaxOpenConnections; got != 5 {
		t.Errorf("MaxOpenConns 未生效：got %d, want 5", got)
	}
}

func TestValidateConfig(t *testing.T) {
	cases := []Config{
		{DSN: "x"},
		{Driver: "dbxtest"},
		{Driver: "dbxtest", DSN: "x", MaxOpenConns: -1},
		{Driver: "dbxtest", DSN: "x", MaxIdleConns: -1},
		{Driver: "dbxtest", DSN: "x", ConnMaxLifetime: -1},
		{Driver: "dbxtest", DSN: "x", ConnMaxIdleTime: -1},
		{Driver: "dbxtest", DSN: "x", SlowQueryThreshold: -1},
	}
	for i, cfg := range cases {
		_, err := OpenConfig(context.Background(), cfg)
		if !errx.Is(err, CodeBadArgument) || errx.KindOf(err) != errx.KindInvalid {
			t.Errorf("用例 %d 错误码/分类不符：%v", i, err)
		}
	}
}

func TestOptions(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x",
		WithMaxOpenConns(3),
		WithMaxIdleConns(1),
		WithConnMaxLifetime(time.Minute),
		WithConnMaxIdleTime(30*time.Second),
		WithSlowQueryThreshold(50*time.Millisecond),
		WithLogger(nil),
	)
	testx.RequireNoError(t, err)

	defer db.Close()
	if db.cfg.MaxOpenConns != 3 || db.cfg.MaxIdleConns != 1 ||
		db.cfg.ConnMaxLifetime != time.Minute || db.cfg.ConnMaxIdleTime != 30*time.Second ||
		db.cfg.SlowQueryThreshold != 50*time.Millisecond {
		t.Errorf("选项未生效：%+v", db.cfg)
	}
}

func TestNilOption(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x", nil)
	if err != nil || db == nil {
		t.Fatalf("nil 选项应被忽略：%v", err)
	}
	if err := db.Close(); err != nil {
		t.Errorf("Close 失败：%v", err)
	}
}

func TestExec(t *testing.T) {
	fake.set(fakeConfig{affected: 3, insertID: 7})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	res, err := db.Exec(context.Background(), Raw(`UPDATE users SET name = $1 WHERE id = $2`, "x", 1))
	testx.RequireNoError(t, err)

	affected, err := res.RowsAffected()
	if err != nil || affected != 3 {
		t.Errorf("RowsAffected 不符：%d, %v", affected, err)
	}
}

func TestExecFailure(t *testing.T) {
	fake.set(fakeConfig{execErr: errors.New("执行失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = db.Exec(context.Background(), Raw(`UPDATE users SET name = $1`, "x"))
	if !errx.Is(err, CodeExecFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("Exec 失败错误码/分类不符：%v", err)
	}
}

func TestExecDuplicate(t *testing.T) {
	fake.set(fakeConfig{execErr: errors.New("Duplicate entry 'x' for key 'users.email'")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = db.Exec(context.Background(), Raw(`INSERT INTO users (email) VALUES (?)`, "x"))
	if !IsDuplicate(err) || errx.KindOf(err) != errx.KindConflict {
		t.Errorf("重复键错误码/分类不符：%v", err)
	}
}

func TestQuery(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id", "name"},
		rows:    [][]driver.Value{{int64(1), "张三"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	rows, err := db.Query(context.Background(), Raw(`SELECT id, name FROM users`))
	testx.RequireNoError(t, err)

	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil || len(cols) != 2 {
		t.Fatalf("列信息不符：%v, %v", cols, err)
	}
	var id int64
	var name string
	if !rows.Next() {
		t.Fatal("应有数据行")
	}
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Scan 失败：%v", err)
	}
	if id != 1 || name != "张三" {
		t.Errorf("扫描结果不符：%d %q", id, name)
	}
	if rows.Next() {
		t.Error("不应有更多数据行")
	}
	if err := rows.Err(); err != nil {
		t.Errorf("结果集错误：%v", err)
	}
}

func TestQueryFailure(t *testing.T) {
	fake.set(fakeConfig{queryErr: errors.New("查询失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = db.Query(context.Background(), Raw(`SELECT * FROM users`))
	if !errx.Is(err, CodeQueryFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("Query 失败错误码/分类不符：%v", err)
	}
}

func TestQueryRow(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id"},
		rows:    [][]driver.Value{{int64(42)}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	var id int64
	row, err := db.QueryRow(context.Background(), Raw(`SELECT id FROM users WHERE id = $1`, 42))
	testx.RequireNoError(t, err)

	if err := row.Scan(&id); err != nil {
		t.Fatalf("QueryRow Scan 失败：%v", err)
	}
	if id != 42 {
		t.Errorf("QueryRow 结果不符：%d", id)
	}
}

func TestQueryRowNoRows(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	var id int64
	row, err := db.QueryRow(context.Background(), Raw(`SELECT id FROM users WHERE id = $1`, 1))
	testx.RequireNoError(t, err)

	if err := row.Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("无数据应返回 sql.ErrNoRows：%v", err)
	}
}

func TestClose(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	if err := db.Close(); err != nil {
		t.Errorf("Close 失败：%v", err)
	}
}

func TestCloseFailed(t *testing.T) {
	closeErr := errors.New("关闭失败")
	fake.set(fakeConfig{closeErr: closeErr})
	db, err := Open(context.Background(), "dbxtest", "x", WithMaxIdleConns(1))
	testx.RequireNoError(t, err)

	err = db.Close()
	if !errx.Is(err, CodeCloseFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("Close 失败错误码/分类不符：%v", err)
	}
	testx.ErrorIs(t, err, closeErr)

}
