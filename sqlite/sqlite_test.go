package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lcylpzls/dbx"
)

// user 是集成测试使用的扫描结构体。
type user struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

// runBasic 运行三种方言通用的基础场景。
func runBasic(t *testing.T, db *dbx.DB, placeholder func(n int) string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Exec(ctx, dbx.Raw(`DROP TABLE IF EXISTS dbx_test_users`)); err != nil {
		t.Fatalf("清理旧表失败：%v", err)
	}
	if _, err := db.Exec(ctx, dbx.Raw(
		`CREATE TABLE dbx_test_users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, age INTEGER NOT NULL)`)); err != nil {
		t.Fatalf("建表失败：%v", err)
	}
	for i, name := range []string{"张三", "李四", "王五"} {
		q := dbx.Raw(fmt.Sprintf(
			`INSERT INTO dbx_test_users (id, name, age) VALUES (%s, %s, %s)`,
			placeholder(1), placeholder(2), placeholder(3)),
			int64(i+1), name, int64(20+i))
		if _, err := db.Exec(ctx, q); err != nil {
			t.Fatalf("写入失败：%v", err)
		}
	}
	u, err := dbx.One[user](ctx, db, dbx.Select(`SELECT id, name, age FROM dbx_test_users`).Where(`id = ?`, int64(1)))
	if err != nil {
		t.Fatalf("One 失败：%v", err)
	}
	if u.ID != 1 || u.Name != "张三" || u.Age != 20 {
		t.Errorf("One 结果不符：%+v", u)
	}
	_, err = dbx.One[user](ctx, db, dbx.Select(`SELECT id, name, age FROM dbx_test_users`).Where(`id = ?`, int64(99)))
	if !dbx.IsNotFound(err) {
		t.Fatalf("无数据应返回 DBX_NOT_FOUND：%v", err)
	}
	users, err := dbx.List[user](ctx, db, dbx.Select(`SELECT id, name, age FROM dbx_test_users`).
		Where(`age >= ?`, int64(20)).
		OrderBy(`id`, true).
		Page(1, 2))
	if err != nil {
		t.Fatalf("List 失败：%v", err)
	}
	if len(users) != 2 || users[0].ID != 3 || users[1].ID != 2 {
		t.Errorf("List 分页/排序结果不符：%+v", users)
	}
	row, err := db.QueryRow(ctx, dbx.Select(`SELECT name FROM dbx_test_users`).Where(`id = ?`, int64(2)))
	if err != nil {
		t.Fatalf("QueryRow 失败：%v", err)
	}
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatalf("QueryRow Scan 失败：%v", err)
	}
	if name != "李四" {
		t.Errorf("QueryRow 结果不符：%q", name)
	}
	if err := runTxScenario(t, db, placeholder); err != nil {
		t.Fatalf("事务场景失败：%v", err)
	}
	if _, err := db.Exec(ctx, dbx.Raw(`DROP TABLE IF EXISTS dbx_test_users`)); err != nil {
		t.Fatalf("清理表失败：%v", err)
	}
}

// runTxScenario 验证真实事务:提交、嵌套保存点成功与整体回滚。
func runTxScenario(t *testing.T, db *dbx.DB, placeholder func(n int) string) error {
	t.Helper()
	ctx := context.Background()
	insert := func(id int64, name string, age int64) dbx.Query {
		return dbx.Raw(fmt.Sprintf(
			`INSERT INTO dbx_test_users (id, name, age) VALUES (%s, %s, %s)`,
			placeholder(1), placeholder(2), placeholder(3)), id, name, age)
	}
	if err := db.WithTx(ctx, func(tx *dbx.Tx) error {
		if err := tx.Exec(ctx, insert(10, "事务", 30)); err != nil {
			return err
		}
		return tx.Nested(ctx, func(child *dbx.Tx) error {
			return child.Exec(ctx, insert(11, "嵌套", 31))
		})
	}); err != nil {
		return err
	}
	if _, err := dbx.One[user](ctx, db, dbx.Select(`SELECT id, name, age FROM dbx_test_users`).Where(`id = ?`, int64(11))); err != nil {
		return err
	}
	err := db.WithTx(ctx, func(tx *dbx.Tx) error {
		if err := tx.Exec(ctx, insert(12, "回滚", 32)); err != nil {
			return err
		}
		if err := tx.Nested(ctx, func(child *dbx.Tx) error {
			return child.Exec(ctx, insert(13, "嵌套回滚", 33))
		}); err != nil {
			return err
		}
		return errors.New("触发整体回滚")
	})
	if err == nil {
		return errors.New("整体回滚未触发")
	}
	if _, err := dbx.One[user](ctx, db, dbx.Select(`SELECT id, name, age FROM dbx_test_users`).Where(`id = ?`, int64(13))); !dbx.IsNotFound(err) {
		return errors.New("回滚后嵌套写入仍存在")
	}
	if _, err := dbx.One[user](ctx, db, dbx.Select(`SELECT id, name, age FROM dbx_test_users`).Where(`id = ?`, int64(12))); !dbx.IsNotFound(err) {
		return errors.New("回滚后外层写入仍存在")
	}
	return nil
}

func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dbx-test.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	runBasic(t, db, func(n int) string { return "?" })
}
