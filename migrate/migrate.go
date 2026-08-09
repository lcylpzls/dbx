// Package migrate 提供轻量迁移:
// 按文件名顺序执行 fsys 中的 *.sql,并记录到 schema_migrations 版本表。
// v0.1 仅支持 up-only。
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/lcylpzls/dbx"
	"github.com/lcylpzls/errx"
)

// migrationRow 是版本表的一行。
type migrationRow struct {
	Version string `db:"version"`
}

// createVersionTableSQL 是版本表 DDL,三种方言兼容。
const createVersionTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// Run 按文件名顺序执行 fsys 中的 *.sql,并记录到 schema_migrations。
// 已存在的版本跳过;每个迁移文件在一个事务内执行,失败时该文件整体回滚,
// 已成功应用的文件保持已记录状态,再次运行自动跳过。
func Run(ctx context.Context, db *dbx.DB, fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return errx.Wrap(err, errx.KindUnavailable, dbx.CodeMigrationFailed, "读取迁移文件列表失败")
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if err := ensureVersionTable(ctx, db); err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}
	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return errx.Wrap(err, errx.KindUnavailable, dbx.CodeMigrationFailed, "读取迁移文件失败")
		}
		if err := applyMigration(ctx, db, version, string(data)); err != nil {
			return err
		}
	}
	return nil
}

// ensureVersionTable 创建版本表(幂等)。
func ensureVersionTable(ctx context.Context, db *dbx.DB) error {
	if _, err := db.Exec(ctx, dbx.Raw(createVersionTableSQL)); err != nil {
		return errx.Wrap(err, errx.KindUnavailable, dbx.CodeMigrationFailed, "创建版本表失败")
	}
	return nil
}

// appliedVersions 返回已应用版本的集合。
func appliedVersions(ctx context.Context, db *dbx.DB) (map[string]bool, error) {
	rows, err := dbx.List[migrationRow](ctx, db, dbx.Select(`SELECT version FROM schema_migrations`))
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, dbx.CodeMigrationFailed, "读取已应用版本失败")
	}
	applied := make(map[string]bool, len(rows))
	for _, row := range rows {
		applied[row.Version] = true
	}
	return applied, nil
}

// applyMigration 在单个事务内执行迁移文件并记录版本。
func applyMigration(ctx context.Context, db *dbx.DB, version, content string) error {
	if err := db.WithTx(ctx, func(tx *dbx.Tx) error {
		for _, stmt := range splitStatements(content) {
			if _, err := tx.Exec(ctx, dbx.Raw(stmt)); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, dbx.Select(`INSERT INTO schema_migrations (version) VALUES (?)`).Args(version))
		return err
	}); err != nil {
		return errx.Wrap(err, errx.KindUnavailable, dbx.CodeMigrationFailed,
			fmt.Sprintf("应用迁移 %s 失败", version))
	}
	return nil
}
