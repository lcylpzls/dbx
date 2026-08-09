package dbx

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"github.com/lcylpzls/errx"
)

func TestSelectBase(t *testing.T) {
	sqlText, args, err := Select(`SELECT * FROM users`).SQL()
	if err != nil || sqlText != `SELECT * FROM users` || len(args) != 0 {
		t.Errorf("Select 基础结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestSelectEmptyBase(t *testing.T) {
	if _, _, err := Select("  ").SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("空 SQL 主体错误码不符：%v", err)
	}
}

func TestSelectWhereAndOr(t *testing.T) {
	q := Select(`SELECT * FROM users`).
		Where(`status = ?`, 1).
		And(`name LIKE ?`, "%张%").
		Or(`age > ?`, 18)
	sqlText, args, err := q.SQL()
	want := `SELECT * FROM users WHERE status = ? AND name LIKE ? OR age > ?`
	if err != nil || sqlText != want || !reflect.DeepEqual(args, []any{1, "%张%", 18}) {
		t.Errorf("条件拼接结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestSelectWhereAfterCondition(t *testing.T) {
	q := Select(`SELECT * FROM users`).Where(`a = ?`, 1).Where(`b = ?`, 2)
	sqlText, _, err := q.SQL()
	if err != nil || !strings.Contains(sqlText, "WHERE a = ? AND b = ?") {
		t.Errorf("Where 后续条件应以 AND 连接：%q %v", sqlText, err)
	}
}

func TestSelectIn(t *testing.T) {
	q := Select(`SELECT * FROM users`).Where(`status = ?`, 1).In(`id`, 1, 2, 3)
	sqlText, args, err := q.SQL()
	want := `SELECT * FROM users WHERE status = ? AND id IN (?, ?, ?)`
	if err != nil || sqlText != want || !reflect.DeepEqual(args, []any{1, 1, 2, 3}) {
		t.Errorf("IN 拼接结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestSelectInEmpty(t *testing.T) {
	if _, _, err := Select(`SELECT * FROM users`).In(`id`).SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("空 IN 错误码不符：%v", err)
	}
}

func TestSelectLikeBetweenIsNull(t *testing.T) {
	q := Select(`SELECT * FROM users`).
		Like(`name`, "%张%").
		Between(`age`, 18, 60).
		IsNull(`deleted_at`)
	sqlText, args, err := q.SQL()
	want := `SELECT * FROM users WHERE name LIKE ? AND age BETWEEN ? AND ? AND deleted_at IS NULL`
	if err != nil || sqlText != want || !reflect.DeepEqual(args, []any{"%张%", 18, 60}) {
		t.Errorf("LIKE/BETWEEN/IS NULL 结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestSelectOrderBy(t *testing.T) {
	q := Select(`SELECT * FROM users`).OrderBy(`created_at`, true).OrderBy(`id`, false)
	sqlText, _, err := q.SQL()
	want := "SELECT * FROM users ORDER BY `created_at` DESC, `id` ASC"
	if err != nil || sqlText != want {
		t.Errorf("排序结果不符：%q %v", sqlText, err)
	}
}

func TestSelectOrderByInvalidColumn(t *testing.T) {
	if _, _, err := Select(`SELECT * FROM users`).OrderBy(`created_at; DROP`, true).SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("非法排序列错误码不符：%v", err)
	}
}

func TestSelectPage(t *testing.T) {
	q := Select(`SELECT * FROM users`).Where(`status = ?`, 1).Page(2, 20)
	sqlText, args, err := q.SQL()
	want := `SELECT * FROM users WHERE status = ? LIMIT ? OFFSET ?`
	if err != nil || sqlText != want || !reflect.DeepEqual(args, []any{1, int64(20), int64(20)}) {
		t.Errorf("分页结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestSelectPageInvalid(t *testing.T) {
	if _, _, err := Select(`SELECT 1`).Page(0, 10).SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("页号 0 错误码不符：%v", err)
	}
	if _, _, err := Select(`SELECT 1`).Page(1, 0).SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("每页 0 错误码不符：%v", err)
	}
}

func TestSelectArgs(t *testing.T) {
	q := Select(`INSERT INTO users (id, name) VALUES (?, ?)`).
		Args(int64(1), "张三").
		Where(`status = ?`, 1)
	sqlText, args, err := q.SQL()
	want := `INSERT INTO users (id, name) VALUES (?, ?) WHERE status = ?`
	if err != nil || sqlText != want || !reflect.DeepEqual(args, []any{int64(1), "张三", 1}) {
		t.Errorf("Args 拼接结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestSelectLimitOffsetInvalid(t *testing.T) {
	if _, _, err := Select(`SELECT 1`).LimitOffset(-1, 0).SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("负数 limit 错误码不符：%v", err)
	}
	if _, _, err := Select(`SELECT 1`).LimitOffset(1, -1).SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("负数 offset 错误码不符：%v", err)
	}
}

func TestSelectEmptyCondition(t *testing.T) {
	if _, _, err := Select(`SELECT 1`).Where("  ").SQL(); !errx.Is(err, CodeBadArgument) {
		t.Errorf("空条件错误码不符：%v", err)
	}
}

func TestSelectPostgresPlaceholders(t *testing.T) {
	q := Select(`SELECT * FROM users`).
		Where(`status = ?`, 1).
		And(`name LIKE ?`, "x").
		OrderBy(`created_at`, true).
		Page(2, 20)
	sqlText, args, err := q.render(pgDialect{})
	want := `SELECT * FROM users WHERE status = $1 AND name LIKE $2 ORDER BY "created_at" DESC LIMIT $3 OFFSET $4`
	if err != nil || sqlText != want || !reflect.DeepEqual(args, []any{1, "x", int64(20), int64(20)}) {
		t.Errorf("PostgreSQL 渲染结果不符：%q %v %v", sqlText, args, err)
	}
}

func TestConvertPlaceholdersRemaining(t *testing.T) {
	got := convertPlaceholders("a = ? AND b = ?", pgDialect{}, 1)
	if got != "a = $1 AND b = ?" {
		t.Errorf("占位符转换不符：%q", got)
	}
}

func TestDBRenderSelectQuery(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x")
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	if _, err := db.Query(context.Background(), Select(`SELECT id FROM users`).Where(`id = ?`, 1)); err != nil {
		t.Errorf("Query 渲染失败：%v", err)
	}
	if _, err := db.Exec(context.Background(), Select(`UPDATE users SET name = ?`).Where(`id = ?`, 1)); err != nil {
		t.Errorf("Exec 渲染失败：%v", err)
	}
}

func TestDBRenderError(t *testing.T) {
	db, err := Open(context.Background(), "dbxtest", "x")
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	bad := Select(`SELECT 1`).OrderBy(`x; DROP`, true)
	if _, err := db.Exec(context.Background(), bad); !errx.Is(err, CodeBadArgument) {
		t.Errorf("Exec 构造错误码不符：%v", err)
	}
	if _, err := db.Query(context.Background(), bad); !errx.Is(err, CodeBadArgument) {
		t.Errorf("Query 构造错误码不符：%v", err)
	}
	if row, err := db.QueryRow(context.Background(), bad); row != nil || !errx.Is(err, CodeBadArgument) {
		t.Errorf("QueryRow 构造错误码不符：%v", err)
	}
}

func TestDBQueryRowSelect(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id"},
		rows:    [][]driver.Value{{int64(7)}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	row, err := db.QueryRow(context.Background(), Select(`SELECT id FROM users`).Where(`id = ?`, 7))
	if err != nil {
		t.Fatalf("QueryRow 失败：%v", err)
	}
	var id int64
	if err := row.Scan(&id); err != nil || id != 7 {
		t.Errorf("QueryRow 结果不符：%d, %v", id, err)
	}
}

func FuzzBuilder(f *testing.F) {
	f.Add("name", "%张%", "1", "2")
	f.Add("users.id", "x", "0", "0")
	f.Add("id; DROP TABLE users", "y", "-1", "-2")
	f.Fuzz(func(t *testing.T, col, pattern, lo, hi string) {
		q := Select(`SELECT * FROM users`).
			Where(`status = ?`, 1).
			And(`name LIKE ?`, pattern).
			In(col, 1, 2).
			Like(col, pattern).
			Between(col, lo, hi).
			IsNull(col).
			OrderBy(col, true).
			Page(1, 10)
		_, _, _ = q.SQL()
		_, _, _ = q.render(pgDialect{})
		q2 := Select(`SELECT * FROM users`).Or(`a = ?`, 1).LimitOffset(1, 0)
		_, _, _ = q2.SQL()
	})
}
