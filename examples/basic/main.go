// basic 示例:连接、建表、写入与 One/List 扫描。
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
	path := filepath.Join(os.TempDir(), "dbx-example-basic.db")
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
	for i, name := range []string{"张三", "李四"} {
		if _, err := db.Exec(ctx, dbx.Raw(
			`INSERT INTO users (id, name, age) VALUES (?, ?, ?)`, int64(i+1), name, int64(20+i))); err != nil {
			panic(err)
		}
	}

	u, err := dbx.One[User](ctx, db, dbx.Select(`SELECT id, name, age FROM users`).Where(`id = ?`, int64(1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("单条查询：%+v\n", u)

	users, err := dbx.List[User](ctx, db, dbx.Select(`SELECT id, name, age FROM users`).OrderBy(`id`, true))
	if err != nil {
		panic(err)
	}
	for _, user := range users {
		fmt.Printf("用户：%d %s %d\n", user.ID, user.Name, user.Age)
	}
}
