package dbx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	testx "github.com/lcylpzls/testx"
	"testing"

	"github.com/lcylpzls/errx"
)

func TestWithTxCommit(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		_, err := tx.Exec(context.Background(), Raw(`UPDATE users SET name = $1 WHERE id = $2`, "x", 1))
		return err
	})
	testx.RequireNoError(t, err)

}

func TestWithTxOptions(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error { return nil },
		WithIsolation(sql.LevelSerializable),
		WithReadOnly(true),
		nil)
	testx.RequireNoError(t, err)

	opts := fake.lastTxOptions()
	if opts.Isolation != sql.LevelSerializable || !opts.ReadOnly {
		t.Errorf("事务选项未生效：%+v", opts)
	}
}

func TestWithTxBeginFailed(t *testing.T) {
	fake.set(fakeConfig{beginTxErr: errors.New("开启失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error { return nil })
	if !errx.Is(err, CodeTxBeginFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("开启事务失败错误码/分类不符：%v", err)
	}
}

func TestWithTxRollbackOnError(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	wantErr := errors.New("业务错误")
	err = db.WithTx(context.Background(), func(tx *Tx) error { return wantErr })
	testx.ErrorIs(t, err, wantErr)

}

func TestWithTxCallbackFailed(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	wantErr := errors.New("业务错误")
	err = db.WithTx(context.Background(), func(tx *Tx) error { return wantErr })
	if !errx.Is(err, CodeTxCallbackFailed) || errx.KindOf(err) != errx.KindBusiness {
		t.Errorf("普通回调错误应包装为回调失败：%v", err)
	}
	testx.ErrorIs(t, err, wantErr)

}

func TestWithTxRollbackFailed(t *testing.T) {
	fake.set(fakeConfig{rollbackErr: errors.New("回滚失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error { return errors.New("业务错误") })
	if !errx.Is(err, CodeTxRollbackFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("回滚失败错误码/分类不符：%v", err)
	}
}

func TestWithTxCommitFailed(t *testing.T) {
	fake.set(fakeConfig{commitErr: errors.New("提交失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error { return nil })
	if !errx.Is(err, CodeTxCommitFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("提交失败错误码/分类不符：%v", err)
	}
}

func TestWithTxPanic(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = db.WithTx(context.Background(), func(tx *Tx) error { panic("boom") })
	}()
	testx.RequireTrue(t, panicked)

}

func TestTxExecFailure(t *testing.T) {
	fake.set(fakeConfig{execErr: errors.New("执行失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		_, err := tx.Exec(context.Background(), Raw(`UPDATE users SET name = $1`, "x"))
		return err
	})
	if !errx.Is(err, CodeExecFailed) {
		t.Errorf("Tx.Exec 失败错误码不符：%v", err)
	}
}

func TestTxExecResult(t *testing.T) {
	fake.set(fakeConfig{affected: 3, insertID: 7})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		res, err := tx.Exec(context.Background(),
			Raw(`UPDATE users SET name = ? WHERE id = ?`, "x", 1))
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil || n != 3 {
			return errors.New("RowsAffected 不符")
		}
		return nil
	})
	testx.RequireNoError(t, err)

}

func TestTxExecDuplicate(t *testing.T) {
	fake.set(fakeConfig{execErr: errors.New("UNIQUE constraint failed: users.mac")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		_, err := tx.Exec(context.Background(), Raw(`INSERT INTO users (mac) VALUES (?)`, "x"))
		return err
	})
	if !IsDuplicate(err) {
		t.Errorf("事务内重复键错误码不符：%v", err)
	}
}

func TestTxExecRenderError(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		_, err := tx.Exec(context.Background(), Select(`UPDATE users SET name = ?`).OrderBy(`x; DROP`, true))
		return err
	})
	if !errx.Is(err, CodeBadArgument) {
		t.Errorf("Tx.Exec 构造错误码不符：%v", err)
	}
}

func TestTxQueryAndQueryRow(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id"},
		rows:    [][]driver.Value{{int64(7)}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		rows, err := tx.Query(context.Background(), Raw(`SELECT id FROM users`))
		if err != nil {
			return err
		}
		cols, err := rows.Columns()
		if err != nil || len(cols) != 1 {
			return errors.New("列信息不符")
		}
		rows.Close()
		row, err := tx.QueryRow(context.Background(), Raw(`SELECT id FROM users`))
		if err != nil {
			return err
		}
		var id int64
		if err := row.Scan(&id); err != nil || id != 7 {
			return errors.New("QueryRow 结果不符")
		}
		return nil
	})
	testx.RequireNoError(t, err)

}

func TestTxQueryFailure(t *testing.T) {
	fake.set(fakeConfig{queryErr: errors.New("查询失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		_, err := tx.Query(context.Background(), Raw(`SELECT id FROM users`))
		return err
	})
	if !errx.Is(err, CodeQueryFailed) {
		t.Errorf("Tx.Query 失败错误码不符：%v", err)
	}
}

func TestTxQueryRenderError(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		rows, err := tx.Query(context.Background(), Select(`SELECT 1`).OrderBy(`x; DROP`, true))
		if rows != nil {
			rows.Close()
		}
		return err
	})
	if !errx.Is(err, CodeBadArgument) {
		t.Errorf("Tx.Query 构造错误码不符：%v", err)
	}
}

func TestTxQueryRowRenderError(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		row, err := tx.QueryRow(context.Background(), Select(`SELECT 1`).OrderBy(`x; DROP`, true))
		if row != nil {
			return errors.New("应返回 nil row")
		}
		return err
	})
	if !errx.Is(err, CodeBadArgument) {
		t.Errorf("Tx.QueryRow 构造错误码不符：%v", err)
	}
}

func TestTxOneList(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id", "name"},
		rows:    [][]driver.Value{{int64(1), "张三"}, {int64(2), "李四"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		u, err := One[scanUser](context.Background(), tx, Raw(`SELECT id, name FROM users WHERE id = $1`, 1))
		if err != nil || u.ID != 1 {
			return errors.New("Tx One 结果不符")
		}
		users, err := List[scanUser](context.Background(), tx, Raw(`SELECT id, name FROM users`))
		if err != nil || len(users) != 2 {
			return errors.New("Tx List 结果不符")
		}
		return nil
	})
	testx.RequireNoError(t, err)

}

func TestNestedCommit(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		if _, err := tx.Exec(context.Background(), Raw(`UPDATE users SET name = $1`, "x")); err != nil {
			return err
		}
		return tx.Nested(context.Background(), func(child *Tx) error {
			_, err := child.Exec(context.Background(), Raw(`UPDATE users SET name = $1`, "y"))
			return err
		})
	})
	testx.RequireNoError(t, err)

}

func TestNestedRollback(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	wantErr := errors.New("内层错误")
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		nestedErr := tx.Nested(context.Background(), func(child *Tx) error { return wantErr })
		if !errors.Is(nestedErr, wantErr) {
			return errors.New("Nested 应返回原始错误")
		}
		return nil
	})
	testx.RequireNoError(t, err)

}

func TestNestedBeginFailed(t *testing.T) {
	fake.set(fakeConfig{savepointBeginErr: errors.New("保存点失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		return tx.Nested(context.Background(), func(child *Tx) error { return nil })
	})
	if !errx.Is(err, CodeTxBeginFailed) {
		t.Errorf("保存点创建失败错误码不符：%v", err)
	}
}

func TestNestedRollbackToFailed(t *testing.T) {
	fake.set(fakeConfig{savepointRollbackErr: errors.New("回滚保存点失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		return tx.Nested(context.Background(), func(child *Tx) error { return errors.New("内层错误") })
	})
	if !errx.Is(err, CodeTxRollbackFailed) {
		t.Errorf("保存点回滚失败错误码不符：%v", err)
	}
}

func TestNestedReleaseAfterRollbackFailed(t *testing.T) {
	fake.set(fakeConfig{savepointReleaseErr: errors.New("释放保存点失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		return tx.Nested(context.Background(), func(child *Tx) error { return errors.New("内层错误") })
	})
	if !errx.Is(err, CodeTxRollbackFailed) {
		t.Errorf("回滚后释放失败错误码不符：%v", err)
	}
}

func TestNestedReleaseFailed(t *testing.T) {
	fake.set(fakeConfig{savepointReleaseErr: errors.New("释放保存点失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		return tx.Nested(context.Background(), func(child *Tx) error { return nil })
	})
	if !errx.Is(err, CodeTxRollbackFailed) {
		t.Errorf("成功路径释放失败错误码不符：%v", err)
	}
}

func TestNestedPanic(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = db.WithTx(context.Background(), func(tx *Tx) error {
			return tx.Nested(context.Background(), func(child *Tx) error { panic("boom") })
		})
	}()
	testx.RequireTrue(t, panicked)

}

func TestNestedDeep(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		return tx.Nested(context.Background(), func(inner *Tx) error {
			return inner.Nested(context.Background(), func(leaf *Tx) error {
				_, err := leaf.Exec(context.Background(), Raw(`SELECT 1`))
				return err
			})
		})
	})
	testx.RequireNoError(t, err)

}

func TestBatchExecDB(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.BatchExec(context.Background(),
		`INSERT INTO users (id, name) VALUES (?, ?)`,
		[][]any{{int64(1), "a"}, {int64(2), "b"}})
	testx.RequireNoError(t, err)

}

func TestBatchExecEmpty(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	if err := db.BatchExec(context.Background(), `INSERT INTO users (id) VALUES (?)`, nil); err != nil {
		t.Fatalf("空参数 BatchExec 失败：%v", err)
	}
}

func TestBatchExecPrepareFailed(t *testing.T) {
	fake.set(fakeConfig{prepareErr: errors.New("预编译失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.BatchExec(context.Background(), `INSERT INTO users (id) VALUES (?)`, [][]any{{int64(1)}})
	if !errx.Is(err, CodeExecFailed) {
		t.Errorf("预编译失败错误码不符：%v", err)
	}
}

func TestBatchExecExecFailed(t *testing.T) {
	fake.set(fakeConfig{execErr: errors.New("执行失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.BatchExec(context.Background(), `INSERT INTO users (id) VALUES (?)`, [][]any{{int64(1)}})
	if !errx.Is(err, CodeExecFailed) {
		t.Errorf("批量执行失败错误码不符：%v", err)
	}
}

func TestBatchExecDuplicate(t *testing.T) {
	fake.set(fakeConfig{execErr: errors.New("duplicate key value violates unique constraint")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.BatchExec(context.Background(), `INSERT INTO users (id) VALUES (?)`, [][]any{{int64(1)}})
	if !IsDuplicate(err) {
		t.Errorf("批量重复键错误码不符：%v", err)
	}
}

func TestBatchExecTx(t *testing.T) {
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	err = db.WithTx(context.Background(), func(tx *Tx) error {
		return tx.BatchExec(context.Background(),
			`INSERT INTO users (id, name) VALUES (?, ?)`,
			[][]any{{int64(1), "a"}})
	})
	testx.RequireNoError(t, err)

}
