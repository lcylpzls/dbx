package core

import (
	"context"
	"database/sql/driver"
	testx "github.com/lcylpzls/testx"
	"testing"
)

// benchUser 是基准测试使用的 5 字段扫描结构体。
type benchUser struct {
	ID     int64   `db:"id"`
	Name   string  `db:"name"`
	Age    int     `db:"age"`
	Score  float64 `db:"score"`
	Active bool    `db:"active"`
}

func BenchmarkRaw_Build(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q := Raw(`SELECT 1`)
		if _, _, err := q.SQL(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRaw_BuildWithArgs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q := Raw(`SELECT id, name FROM users WHERE id = ?`, int64(i), "张三")
		if _, _, err := q.SQL(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelect_Build(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q := Select(`SELECT id, name FROM users`).
			Where(`age >= ?`, int64(i)).
			And(`status = ?`, 1).
			Like(`name`, "%张%").
			OrderBy(`created_at`, true).
			Page(1, 20)
		if _, _, err := q.SQL(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOne_Scan5Fields(b *testing.B) {
	fake.set(fakeConfig{
		columns: []string{"id", "name", "age", "score", "active"},
		rows:    [][]driver.Value{{int64(1), "张三", int64(20), float64(9.5), true}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(b, err)

	defer db.Close()
	ctx := context.Background()
	q := Raw(`SELECT id, name, age, score, active FROM users WHERE id = ?`, int64(1))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := One[benchUser](ctx, db, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkList_Scan50Rows(b *testing.B) {
	benchmarkList(b, 50)
}

func BenchmarkList_Scan100Rows(b *testing.B) {
	benchmarkList(b, 100)
}

func benchmarkList(b *testing.B, n int) {
	rows := make([][]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, []driver.Value{int64(i), "张三", int64(20), float64(9.5), true})
	}
	fake.set(fakeConfig{
		columns: []string{"id", "name", "age", "score", "active"},
		rows:    rows,
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(b, err)

	defer db.Close()
	ctx := context.Background()
	q := Raw(`SELECT id, name, age, score, active FROM users`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		users, err := List[benchUser](ctx, db, q)
		testx.RequireNoError(b, err)

		if len(users) != n {
			b.Fatalf("行数不符：%d", len(users))
		}
	}
}
