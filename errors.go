// Package dbx 提供基于 database/sql 的薄数据访问层:
// 统一连接、事务、扫描、动态查询构造与可观测性,
// 支持 MySQL、SQLite、PostgreSQL。
// 对外错误统一使用 errx 结构化错误,错误码为 DBX_*。
package dbx

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/lcylpzls/errx"
)

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
	// CodeTxCallbackFailed 事务回调失败(已回滚)。
	CodeTxCallbackFailed errx.Code = "DBX_TX_CALLBACK_FAILED"
	// CodeCloseFailed 关闭数据库连接失败。
	CodeCloseFailed errx.Code = "DBX_CLOSE_FAILED"
	// CodeDuplicate 唯一约束/重复键冲突。
	CodeDuplicate errx.Code = "DBX_DUPLICATE"
	// CodeMigrationFailed 迁移执行失败。
	CodeMigrationFailed errx.Code = "DBX_MIGRATION_FAILED"
)

func init() {
	errx.RegisterCode(CodeOpenFailed, "打开数据库连接失败")
	errx.RegisterCodeKind(CodeOpenFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeDriverNotRegistered, "数据库驱动/方言未注册")
	errx.RegisterCodeKind(CodeDriverNotRegistered, errx.KindInvalid)
	errx.RegisterCode(CodeBadArgument, "参数非法")
	errx.RegisterCodeKind(CodeBadArgument, errx.KindInvalid)
	errx.RegisterCode(CodeExecFailed, "Exec 执行失败")
	errx.RegisterCodeKind(CodeExecFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeQueryFailed, "查询失败")
	errx.RegisterCodeKind(CodeQueryFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeScanFailed, "扫描或类型转换失败")
	errx.RegisterCodeKind(CodeScanFailed, errx.KindInvalid)
	errx.RegisterCode(CodeNotFound, "查询无结果")
	errx.RegisterCodeKind(CodeNotFound, errx.KindNotFound)
	errx.RegisterCode(CodeTxBeginFailed, "开启事务失败")
	errx.RegisterCodeKind(CodeTxBeginFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeTxCommitFailed, "提交事务失败")
	errx.RegisterCodeKind(CodeTxCommitFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeTxRollbackFailed, "回滚事务失败")
	errx.RegisterCodeKind(CodeTxRollbackFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeTxCallbackFailed, "事务回调失败,已回滚")
	errx.RegisterCodeKind(CodeTxCallbackFailed, errx.KindBusiness)
	errx.RegisterCode(CodeCloseFailed, "关闭数据库连接失败")
	errx.RegisterCodeKind(CodeCloseFailed, errx.KindUnavailable)
	errx.RegisterCode(CodeDuplicate, "唯一约束或重复键冲突")
	errx.RegisterCodeKind(CodeDuplicate, errx.KindConflict)
	errx.RegisterCode(CodeMigrationFailed, "迁移执行失败")
	errx.RegisterCodeKind(CodeMigrationFailed, errx.KindUnavailable)
}

// IsNotFound 判断错误是否为“查询无结果”。
// 同时识别已包装的 DBX_NOT_FOUND 与 QueryRow 原生返回的 sql.ErrNoRows。
func IsNotFound(err error) bool {
	return errx.Is(err, CodeNotFound) || errors.Is(err, sql.ErrNoRows)
}

// IsDuplicate 判断错误是否为唯一约束/重复键冲突(DBX_DUPLICATE)。
// 支持错误链,未包装错误或 nil 返回 false。
func IsDuplicate(err error) bool {
	return errx.Is(err, CodeDuplicate)
}

// duplicatePatterns 是驱动无关的重复键错误文本特征。
var duplicatePatterns = []string{
	"Duplicate entry",                                // MySQL 1062
	"UNIQUE constraint failed",                       // SQLite
	"duplicate key value violates unique constraint", // PostgreSQL 23505
}

// wrapExecError 将执行错误分类包装:重复键冲突映射为 DBX_DUPLICATE,
// 其余错误保持 DBX_EXEC_FAILED。
func wrapExecError(err error) error {
	for _, pattern := range duplicatePatterns {
		if strings.Contains(err.Error(), pattern) {
			return errx.WrapCode(err, CodeDuplicate, "唯一约束或重复键冲突")
		}
	}
	return errx.WrapCode(err, CodeExecFailed, "Exec 执行失败")
}
