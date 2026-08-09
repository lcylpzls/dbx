// tx 示例:事务提交、嵌套保存点与整体回滚。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/dbx/sqlite"
)

func main() {
	ctx := context.Background()
	path := filepath.Join(os.TempDir(), "dbx-example-tx.db")
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
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)); err != nil {
		panic(err)
	}

	// 提交 + 嵌套保存点
	if err := db.WithTx(ctx, func(tx *dbx.Tx) error {
		if _, err := tx.Exec(ctx, dbx.Raw(
			`INSERT INTO users (id, name) VALUES (?, ?)`, int64(1), "张三")); err != nil {
			return err
		}
		return tx.Nested(ctx, func(child *dbx.Tx) error {
			_, err := child.Exec(ctx, dbx.Raw(
				`INSERT INTO users (id, name) VALUES (?, ?)`, int64(2), "李四"))
			return err
		})
	}); err != nil {
		panic(err)
	}

	// 回调返回错误 -> 整体回滚
	_ = db.WithTx(ctx, func(tx *dbx.Tx) error {
		if _, err := tx.Exec(ctx, dbx.Raw(
			`INSERT INTO users (id, name) VALUES (?, ?)`, int64(3), "王五")); err != nil {
			return err
		}
		return errors.New("模拟业务失败,触发回滚")
	})

	var count int64
	row, err := db.QueryRow(ctx, dbx.Select(`SELECT COUNT(*) FROM users`))
	if err != nil {
		panic(err)
	}
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
	fmt.Printf("事务提交 2 条,回滚 1 条,最终用户数：%d\n", count)
}
