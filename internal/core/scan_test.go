package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	testx "github.com/lcylpzls/testx"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcylpzls/errx"
)

type scanUser struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Email     *string   `db:"email"`
	Age       int       `db:"age"`
	Score     float64   `db:"score"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	Raw       []byte    `db:"raw"`
	Ignored   string    `db:"-"`
	nick      string
}

type ScanBase struct {
	BaseID int64 `db:"base_id"`
}

type scanEmbed struct {
	ScanBase
	Extra string `db:"extra"`
}

type scanEmbedPtr struct {
	*ScanBase
	Extra string `db:"extra"`
}

type hiddenBase struct {
	HiddenID int64 `db:"hidden_id"`
}

type scanHiddenEmbedPtr struct {
	*hiddenBase
	Extra string `db:"extra"`
}

type scanUntagged struct {
	Name string
	Age  int
}

type scanUnsupported struct {
	Data chan int `db:"data"`
}

// errScanner 是测试用 sql.Scanner,只接受字符串,用于覆盖 Scanner 失败分支。
type errScanner struct {
	v string
}

func (s *errScanner) Scan(src any) error {
	sv, ok := src.(string)
	if !ok {
		return errors.New("只接受字符串")
	}
	s.v = sv
	return nil
}

type mappedUser struct {
	ID   int64
	Name string
}

// staticRow 是 Row 的静态实现,用于直接测试 scanStruct。
type staticRow struct {
	columns []string
	values  []any
	colsErr error
	scanErr error
}

func (r *staticRow) Columns() ([]string, error) {
	return r.columns, r.colsErr
}

func (r *staticRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		p, ok := d.(*any)
		if !ok {
			return fmt.Errorf("dbx：静态行目标必须是 *any")
		}
		if i < len(r.values) {
			*p = r.values[i]
		} else {
			*p = nil
		}
	}
	return nil
}

func scanMapper(row Row) (mappedUser, error) {
	var m mappedUser
	if err := row.Scan(&m.ID, &m.Name); err != nil {
		return m, err
	}
	return m, nil
}

func TestScanStruct(t *testing.T) {
	created := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	row := &staticRow{
		columns: []string{"id", "name", "email", "age", "score", "active", "created_at", "raw", "extra_col"},
		values: []any{
			int64(1), "张三", nil, int64(30), float64(9.5), true, created, []byte("x"), "忽略",
		},
	}
	var out scanUser
	if err := scanStruct(row, &out); err != nil {
		t.Fatalf("scanStruct 失败：%v", err)
	}
	if out.ID != 1 || out.Name != "张三" || out.Age != 30 || out.Score != 9.5 || !out.Active {
		t.Errorf("基本字段不符：%+v", out)
	}
	if out.Email != nil {
		t.Errorf("NULL 指针应保持 nil：%v", *out.Email)
	}
	if !out.CreatedAt.Equal(created) || string(out.Raw) != "x" {
		t.Errorf("Scanner 字段不符：%v %q", out.CreatedAt, out.Raw)
	}
	if out.Ignored != "" || out.nick != "" {
		t.Errorf("忽略/未导出字段不应扫描：%+v", out)
	}
}

func TestScanStructCaseInsensitive(t *testing.T) {
	row := &staticRow{
		columns: []string{"NAME", "AGE"},
		values:  []any{"李四", int64(20)},
	}
	var out scanUntagged
	if err := scanStruct(row, &out); err != nil {
		t.Fatalf("scanStruct 失败：%v", err)
	}
	if out.Name != "李四" || out.Age != 20 {
		t.Errorf("大小写不敏感匹配不符：%+v", out)
	}
}

func TestScanStructMissingAndExtra(t *testing.T) {
	row := &staticRow{
		columns: []string{"id", "unknown"},
		values:  []any{int64(5), "x"},
	}
	var out scanUser
	if err := scanStruct(row, &out); err != nil {
		t.Fatalf("scanStruct 失败：%v", err)
	}
	if out.ID != 5 || out.Name != "" || out.Email != nil {
		t.Errorf("缺列/多余列处理不符：%+v", out)
	}
}

func TestScanStructEmbedded(t *testing.T) {
	row := &staticRow{
		columns: []string{"base_id", "extra"},
		values:  []any{int64(9), "ok"},
	}
	var out scanEmbed
	if err := scanStruct(row, &out); err != nil {
		t.Fatalf("scanStruct 失败：%v", err)
	}
	if out.BaseID != 9 || out.Extra != "ok" {
		t.Errorf("嵌入结构体扫描不符：%+v", out)
	}
}

func TestScanStructEmbeddedPtr(t *testing.T) {
	row := &staticRow{
		columns: []string{"base_id", "extra"},
		values:  []any{int64(9), "ok"},
	}
	var out scanEmbedPtr
	if err := scanStruct(row, &out); err != nil {
		t.Fatalf("scanStruct 失败：%v", err)
	}
	if out.BaseID != 9 || out.Extra != "ok" {
		t.Errorf("嵌入指针结构体扫描不符：%+v", out)
	}
}

func TestScanStructHiddenEmbeddedPtrSkipped(t *testing.T) {
	row := &staticRow{
		columns: []string{"hidden_id", "extra"},
		values:  []any{int64(9), "ok"},
	}
	var out scanHiddenEmbedPtr
	if err := scanStruct(row, &out); err != nil {
		t.Fatalf("scanStruct 失败：%v", err)
	}
	if out.Extra != "ok" || out.hiddenBase != nil {
		t.Errorf("未导出嵌入指针应跳过：%+v", out)
	}
}

func TestScanStructInvalidTargets(t *testing.T) {
	row := &staticRow{columns: []string{"id"}, values: []any{int64(1)}}
	if err := scanStruct(row, scanUser{}); !errx.Is(err, CodeBadArgument) {
		t.Errorf("非指针目标错误码不符：%v", err)
	}
	if err := scanStruct(row, (*scanUser)(nil)); !errx.Is(err, CodeBadArgument) {
		t.Errorf("nil 指针目标错误码不符：%v", err)
	}
	var n int
	if err := scanStruct(row, &n); !errx.Is(err, CodeBadArgument) {
		t.Errorf("非结构体目标错误码不符：%v", err)
	}
}

func TestScanStructColumnError(t *testing.T) {
	row := &staticRow{columns: []string{"id"}, colsErr: errors.New("列信息失败")}
	var out scanUser
	if err := scanStruct(row, &out); !errx.Is(err, CodeQueryFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("列信息失败错误码/分类不符：%v", err)
	}
}

func TestScanStructScanError(t *testing.T) {
	row := &staticRow{columns: []string{"id"}, scanErr: errors.New("扫描失败")}
	var out scanUser
	if err := scanStruct(row, &out); !errx.Is(err, CodeScanFailed) {
		t.Errorf("扫描失败错误码不符：%v", err)
	}
}

func TestScanStructUnsupportedField(t *testing.T) {
	row := &staticRow{columns: []string{"data"}, values: []any{int64(1)}}
	var out scanUnsupported
	err := scanStruct(row, &out)
	if !errx.Is(err, CodeScanFailed) || !strings.Contains(err.Error(), "data") {
		t.Errorf("不支持类型错误不符：%v", err)
	}
}

func TestAssignValue(t *testing.T) {
	created := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	keep := "keep"
	cases := []struct {
		name    string
		field   any
		value   any
		want    any
		wantErr bool
	}{
		{name: "string", field: new(string), value: "x", want: "x"},
		{name: "string from bytes", field: new(string), value: []byte("y"), want: "y"},
		{name: "string mismatch", field: new(string), value: int64(1), want: "", wantErr: true},
		{name: "bool", field: new(bool), value: true, want: true},
		{name: "bool mismatch", field: new(bool), value: "true", want: false, wantErr: true},
		{name: "int", field: new(int64), value: int64(7), want: int64(7)},
		{name: "int from int", field: new(int64), value: int(7), want: int64(7)},
		{name: "int mismatch", field: new(int64), value: "7", want: int64(0), wantErr: true},
		{name: "uint", field: new(uint64), value: uint64(7), want: uint64(7)},
		{name: "uint from int64", field: new(uint64), value: int64(7), want: uint64(7)},
		{name: "uint negative", field: new(uint64), value: int64(-1), want: uint64(0), wantErr: true},
		{name: "uint mismatch", field: new(uint64), value: "7", want: uint64(0), wantErr: true},
		{name: "float", field: new(float64), value: float64(1.5), want: 1.5},
		{name: "float from int64", field: new(float64), value: int64(2), want: 2.0},
		{name: "float mismatch", field: new(float64), value: "1.5", want: 0.0, wantErr: true},
		{name: "nil scalar keeps value", field: &keep, value: nil, want: "keep"},
		{name: "nil pointer", field: new(*string), value: nil, want: (*string)(nil)},
		{name: "pointer allocate", field: new(*int64), value: int64(9), want: int64Ptr(9)},
		{name: "pointer mismatch", field: new(*int64), value: "9", want: (*int64)(nil), wantErr: true},
		{name: "scanner success", field: new(errScanner), value: "abc", want: errScanner{v: "abc"}},
		{name: "scanner error", field: new(errScanner), value: int64(1), want: errScanner{}, wantErr: true},
		{name: "time", field: new(time.Time), value: created, want: created},
		{name: "time from string", field: new(time.Time), value: "2026-08-09T12:00:00Z", want: created},
		{name: "time from bytes", field: new(time.Time), value: []byte("2026-08-09"), want: time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)},
		{name: "time bad string", field: new(time.Time), value: "not-a-time", want: time.Time{}, wantErr: true},
		{name: "time mismatch", field: new(time.Time), value: int64(1), want: time.Time{}, wantErr: true},
		{name: "bytes", field: new([]byte), value: []byte("z"), want: []byte("z")},
		{name: "bytes from string", field: new([]byte), value: "z", want: []byte("z")},
		{name: "bytes mismatch", field: new([]byte), value: int64(1), want: []byte(nil), wantErr: true},
		{name: "slice unsupported", field: new([]string), value: int64(1), want: []string(nil), wantErr: true},
		{name: "unsupported kind", field: new(chan int), value: int64(1), want: (chan int)(nil), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assignValue(reflect.ValueOf(tc.field).Elem(), tc.value, "col")
			if tc.wantErr {
				if err == nil || !errx.Is(err, CodeScanFailed) {
					t.Errorf("应返回 CodeScanFailed：%v", err)
				}
				return
			}
			testx.RequireNoError(t, err)

			if got := reflect.ValueOf(tc.field).Elem().Interface(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("赋值结果不符：got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestScanMetaCache(t *testing.T) {
	typ := reflect.TypeOf(scanUser{})
	meta1 := scanMeta(typ)
	meta2 := scanMeta(typ)
	if len(meta1) != len(meta2) {
		t.Errorf("缓存结果不一致：%d vs %d", len(meta1), len(meta2))
	}
	cols := map[string]bool{}
	for _, m := range meta1 {
		cols[m.column] = true
	}
	for _, col := range []string{"id", "name", "email", "age", "score", "active", "created_at", "raw"} {
		if !cols[col] {
			t.Errorf("缺少字段列 %q", col)
		}
	}
	if cols["Ignored"] || cols["nick"] {
		t.Error("忽略/未导出字段不应进入元信息")
	}
}

func TestOne(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id", "name"},
		rows:    [][]driver.Value{{int64(1), "张三"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	u, err := One[scanUser](context.Background(), db, Raw(`SELECT id, name FROM users WHERE id = $1`, 1))
	testx.RequireNoError(t, err)

	if u.ID != 1 || u.Name != "张三" {
		t.Errorf("One 结果不符：%+v", u)
	}
}

func TestOneNotFound(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = One[scanUser](context.Background(), db, Raw(`SELECT id FROM users WHERE id = $1`, 1))
	if !errx.Is(err, CodeNotFound) || !IsNotFound(err) || errx.KindOf(err) != errx.KindNotFound {
		t.Errorf("无数据错误码/分类不符：%v", err)
	}
	testx.ErrorIs(t, err, sql.ErrNoRows)

}

func TestOneQueryFailure(t *testing.T) {
	fake.set(fakeConfig{queryErr: errors.New("查询失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = One[scanUser](context.Background(), db, Raw(`SELECT id FROM users`))
	if !errx.Is(err, CodeQueryFailed) {
		t.Errorf("查询失败错误码不符：%v", err)
	}
}

func TestOneRowsError(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}, rowsErr: errors.New("迭代失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = One[scanUser](context.Background(), db, Raw(`SELECT id FROM users`))
	if !errx.Is(err, CodeQueryFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("迭代失败错误码/分类不符：%v", err)
	}
}

func TestOneTypeMismatch(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"age"},
		rows:    [][]driver.Value{{"abc"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = One[scanUser](context.Background(), db, Raw(`SELECT age FROM users`))
	if !errx.Is(err, CodeScanFailed) || !strings.Contains(err.Error(), "age") {
		t.Errorf("类型不匹配错误不符：%v", err)
	}
}

func TestList(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id", "name"},
		rows:    [][]driver.Value{{int64(1), "张三"}, {int64(2), "李四"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	users, err := List[scanUser](context.Background(), db, Raw(`SELECT id, name FROM users`))
	testx.RequireNoError(t, err)

	if len(users) != 2 || users[0].ID != 1 || users[1].Name != "李四" {
		t.Errorf("List 结果不符：%+v", users)
	}
}

func TestListEmpty(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	users, err := List[scanUser](context.Background(), db, Raw(`SELECT id FROM users`))
	testx.RequireNoError(t, err)

	if users == nil || len(users) != 0 {
		t.Errorf("空结果应返回非 nil 空切片：%#v", users)
	}
}

func TestListQueryFailure(t *testing.T) {
	fake.set(fakeConfig{queryErr: errors.New("查询失败")})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = List[scanUser](context.Background(), db, Raw(`SELECT id FROM users`))
	if !errx.Is(err, CodeQueryFailed) {
		t.Errorf("查询失败错误码不符：%v", err)
	}
}

func TestListRowsError(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id"},
		rows:    [][]driver.Value{{int64(1)}},
		rowsErr: errors.New("迭代失败"),
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = List[scanUser](context.Background(), db, Raw(`SELECT id FROM users`))
	if !errx.Is(err, CodeQueryFailed) || errx.KindOf(err) != errx.KindUnavailable {
		t.Errorf("迭代失败错误码/分类不符：%v", err)
	}
}

func TestListTypeMismatch(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"active"},
		rows:    [][]driver.Value{{"yes"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = List[scanUser](context.Background(), db, Raw(`SELECT active FROM users`))
	if !errx.Is(err, CodeScanFailed) {
		t.Errorf("类型不匹配错误码不符：%v", err)
	}
}

func TestOneWith(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id", "name"},
		rows:    [][]driver.Value{{int64(1), "张三"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	m, err := OneWith[mappedUser](context.Background(), db, Raw(`SELECT id, name FROM users`), scanMapper)
	testx.RequireNoError(t, err)

	if m.ID != 1 || m.Name != "张三" {
		t.Errorf("OneWith 结果不符：%+v", m)
	}
}

func TestOneWithNotFound(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	_, err = OneWith[mappedUser](context.Background(), db, Raw(`SELECT id FROM users`), scanMapper)
	if !IsNotFound(err) {
		t.Errorf("无数据应返回 DBX_NOT_FOUND：%v", err)
	}
}

func TestOneWithMapperError(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id"},
		rows:    [][]driver.Value{{int64(1)}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	bad := func(row Row) (mappedUser, error) {
		return mappedUser{}, errors.New("映射失败")
	}
	_, err = OneWith[mappedUser](context.Background(), db, Raw(`SELECT id FROM users`), bad)
	if !errx.Is(err, CodeScanFailed) {
		t.Errorf("映射失败错误码不符：%v", err)
	}
}

func TestListWith(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id", "name"},
		rows:    [][]driver.Value{{int64(1), "张三"}, {int64(2), "李四"}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	list, err := ListWith[mappedUser](context.Background(), db, Raw(`SELECT id, name FROM users`), scanMapper)
	testx.RequireNoError(t, err)

	if len(list) != 2 || list[1].Name != "李四" {
		t.Errorf("ListWith 结果不符：%+v", list)
	}
}

func TestListWithEmpty(t *testing.T) {
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	list, err := ListWith[mappedUser](context.Background(), db, Raw(`SELECT id FROM users`), scanMapper)
	testx.RequireNoError(t, err)

	if list == nil || len(list) != 0 {
		t.Errorf("空结果应返回非 nil 空切片：%#v", list)
	}
}

func TestListWithMapperError(t *testing.T) {
	fake.set(fakeConfig{
		columns: []string{"id"},
		rows:    [][]driver.Value{{int64(1)}},
	})
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)

	defer db.Close()
	bad := func(row Row) (mappedUser, error) {
		return mappedUser{}, errors.New("映射失败")
	}
	_, err = ListWith[mappedUser](context.Background(), db, Raw(`SELECT id FROM users`), bad)
	if !errx.Is(err, CodeScanFailed) {
		t.Errorf("映射失败错误码不符：%v", err)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func FuzzScan(f *testing.F) {
	f.Add([]byte("张三"), []byte("true"), []byte("42"), []byte("1.5"))
	f.Add([]byte(""), []byte("x"), []byte("abc"), []byte(""))
	f.Add([]byte{0xff}, []byte("yes"), []byte("-1"), []byte("nan"))
	f.Fuzz(func(t *testing.T, name, active, age, score []byte) {
		row := &staticRow{
			columns: []string{"name", "active", "age", "score", "email"},
			values:  []any{string(name), string(active), string(age), string(score), nil},
		}
		var out scanUser
		_ = scanStruct(row, &out)
	})
}
