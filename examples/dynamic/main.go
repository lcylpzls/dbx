// dynamic 示例:动态查询构造(条件、LIKE、排序、分页)。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/dbx/sqlite"
)

type User struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func main() {
	ctx := context.Background()
	path := filepath.Join(os.TempDir(), "dbx-example-dynamic.db")
	_ = os.Remove(path)
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		panic(err)
	}
	defer func() {
		db.Close()
		_ = os.Remove(path)
	}()

	if _, err := db.Exec(ctx, dbx.Raw(
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, age INTEGER NOT NULL)`)); err != nil {
		panic(err)
	}
	names := []string{"张三", "张伟", "李四"}
	for i, name := range names {
		if _, err := db.Exec(ctx, dbx.Raw(
			`INSERT INTO users (id, name, age) VALUES (?, ?, ?)`, int64(i+1), name, int64(20+i*5))); err != nil {
			panic(err)
		}
	}

	q := dbx.Select(`SELECT id, name, age FROM users`).
		Where(`age >= ?`, int64(20)).
		And(`name LIKE ?`, "%张%").
		OrderBy(`age`, false).
		Page(1, 10)
	sqlText, args, err := q.SQL()
	if err != nil {
		panic(err)
	}
	fmt.Printf("生成 SQL：%s\n绑定参数：%v\n", sqlText, args)

	users, err := dbx.List[User](ctx, db, q)
	if err != nil {
		panic(err)
	}
	for _, user := range users {
		fmt.Printf("命中用户：%d %s %d\n", user.ID, user.Name, user.Age)
	}
}
