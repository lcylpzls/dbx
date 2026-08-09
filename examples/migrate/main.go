// migrate 示例:embed 迁移文件、执行与幂等。
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/dbx/migrate"
	"github.com/lcylpzls/dbx/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	ctx := context.Background()
	path := filepath.Join(os.TempDir(), "dbx-example-migrate.db")
	_ = os.Remove(path)
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		panic(err)
	}
	defer func() {
		db.Close()
		_ = os.Remove(path)
	}()

	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	if err := migrate.Run(ctx, db, sub); err != nil {
		panic(err)
	}
	// 重复执行应幂等
	if err := migrate.Run(ctx, db, sub); err != nil {
		panic(err)
	}

	var count int64
	row, err := db.QueryRow(ctx, dbx.Select(`SELECT COUNT(*) FROM users`))
	if err != nil {
		panic(err)
	}
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
	fmt.Printf("迁移后用户数：%d\n", count)
}
