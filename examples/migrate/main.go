// migrate 示例:embed 迁移文件、执行与幂等。
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lcylpzls/clix"
	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/dbx/migrate"
	"github.com/lcylpzls/dbx/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	app, err := clix.New("dbx-migrate", "0.3.0",
		clix.WithDescription("dbx 迁移示例（embed 迁移文件、执行与幂等）"),
		clix.WithIO(os.Stdout, os.Stderr),
		clix.WithGlobalFlags(
			clix.StringFlag("db", "SQLite 数据库文件路径").
				Default(filepath.Join(os.TempDir(), "dbx-example-migrate.db")),
		),
		clix.WithRootAction(run),
	)
	if err != nil {
		panic(err)
	}
	os.Exit(app.Execute(context.Background(), os.Args[1:]))
}

// run 执行迁移并验证幂等（clix 根 Action）。
func run(ctx context.Context, c *clix.Context) error {
	path := c.GlobalString("db")
	_ = os.Remove(path)
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		return err
	}
	defer func() {
		db.Close()
		_ = os.Remove(path)
	}()

	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	if err := migrate.Run(ctx, db, sub); err != nil {
		return err
	}
	// 重复执行应幂等
	if err := migrate.Run(ctx, db, sub); err != nil {
		return err
	}

	var count int64
	row, err := db.QueryRow(ctx, dbx.Select(`SELECT COUNT(*) FROM users`))
	if err != nil {
		return err
	}
	if err := row.Scan(&count); err != nil {
		return err
	}
	fmt.Printf("迁移后用户数：%d\n", count)
	return nil
}
