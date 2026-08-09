// Package dbx 提供基于 database/sql 的薄数据访问层:
// 统一连接、事务、扫描、动态查询构造与可观测性,
// 支持 MySQL、SQLite、PostgreSQL。
// 对外错误统一使用 errx 结构化错误,错误码为 DBX_*。
package dbx

import "github.com/lcylpzls/errx"

// 错误码定义:dbx 各失败场景的错误码。
const (
	// CodeOpenFailed 打开数据库连接失败。
	CodeOpenFailed errx.Code = "DBX_OPEN_FAILED"
	// CodeDriverNotRegistered 数据库驱动/方言未注册(未导入对应子包)。
	CodeDriverNotRegistered errx.Code = "DBX_DRIVER_NOT_REGISTERED"
	// CodeBadArgument 参数非法,如标识符未通过白名单校验。
	CodeBadArgument errx.Code = "DBX_BAD_ARGUMENT"
	// CodeExecFailed Exec 执行失败。
	CodeExecFailed errx.Code = "DBX_EXEC_FAILED"
	// CodeQueryFailed 查询失败。
	CodeQueryFailed errx.Code = "DBX_QUERY_FAILED"
	// CodeScanFailed 扫描或类型转换失败。
	CodeScanFailed errx.Code = "DBX_SCAN_FAILED"
	// CodeNotFound 查询无结果。
	CodeNotFound errx.Code = "DBX_NOT_FOUND"
	// CodeTxBeginFailed 开启事务失败。
	CodeTxBeginFailed errx.Code = "DBX_TX_BEGIN_FAILED"
	// CodeTxCommitFailed 提交事务失败。
	CodeTxCommitFailed errx.Code = "DBX_TX_COMMIT_FAILED"
	// CodeTxRollbackFailed 回滚事务失败。
	CodeTxRollbackFailed errx.Code = "DBX_TX_ROLLBACK_FAILED"
	// CodeMigrationFailed 迁移执行失败。
	CodeMigrationFailed errx.Code = "DBX_MIGRATION_FAILED"
)

func init() {
	errx.RegisterCode(CodeOpenFailed, "打开数据库连接失败")
	errx.RegisterCode(CodeDriverNotRegistered, "数据库驱动/方言未注册")
	errx.RegisterCode(CodeBadArgument, "参数非法")
	errx.RegisterCode(CodeExecFailed, "Exec 执行失败")
	errx.RegisterCode(CodeQueryFailed, "查询失败")
	errx.RegisterCode(CodeScanFailed, "扫描或类型转换失败")
	errx.RegisterCode(CodeNotFound, "查询无结果")
	errx.RegisterCode(CodeTxBeginFailed, "开启事务失败")
	errx.RegisterCode(CodeTxCommitFailed, "提交事务失败")
	errx.RegisterCode(CodeTxRollbackFailed, "回滚事务失败")
	errx.RegisterCode(CodeMigrationFailed, "迁移执行失败")
}

// IsNotFound 判断错误是否为“查询无结果”(DBX_NOT_FOUND)。
// 支持 errors.As / 错误链,未包装错误或 nil 返回 false。
func IsNotFound(err error) bool {
	return errx.Is(err, CodeNotFound)
}
