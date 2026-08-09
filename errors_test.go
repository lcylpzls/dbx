package dbx

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lcylpzls/errx"
)

func TestCodesRegistered(t *testing.T) {
	codes := map[errx.Code]string{
		CodeOpenFailed:          "打开数据库连接失败",
		CodeDriverNotRegistered: "数据库驱动/方言未注册",
		CodeBadArgument:         "参数非法",
		CodeExecFailed:          "Exec 执行失败",
		CodeQueryFailed:         "查询失败",
		CodeScanFailed:          "扫描或类型转换失败",
		CodeNotFound:            "查询无结果",
		CodeTxBeginFailed:       "开启事务失败",
		CodeTxCommitFailed:      "提交事务失败",
		CodeTxRollbackFailed:    "回滚事务失败",
		CodeMigrationFailed:     "迁移执行失败",
	}
	for code, desc := range codes {
		if got := errx.Describe(code); got != desc {
			t.Errorf("错误码 %s 说明不符:got %q, want %q", code, got, desc)
		}
	}
}

func TestIsNotFound(t *testing.T) {
	base := errx.Newf(errx.KindInvalid, CodeNotFound, "无结果")
	if !IsNotFound(base) {
		t.Error("DBX_NOT_FOUND 错误应判定为未找到")
	}
	wrapped := fmt.Errorf("包装:%w", base)
	if !IsNotFound(wrapped) {
		t.Error("错误链中的 DBX_NOT_FOUND 应判定为未找到")
	}
	other := errx.New(errx.KindInvalid, CodeQueryFailed, "查询失败")
	if IsNotFound(other) {
		t.Error("其他错误码不应判定为未找到")
	}
	if IsNotFound(nil) {
		t.Error("nil 不应判定为未找到")
	}
	var e *errx.Error
	if !errors.As(base, &e) {
		t.Error("errors.As 应命中 *errx.Error")
	}
}

func FuzzIsNotFound(f *testing.F) {
	f.Add("DBX_NOT_FOUND", "无结果")
	f.Add("DBX_QUERY_FAILED", "查询失败")
	f.Add("", "")
	f.Fuzz(func(t *testing.T, code, msg string) {
		e := errx.New(errx.KindInvalid, errx.Code(code), msg)
		_ = IsNotFound(e)
		_ = errx.KindOf(e)
	})
}
