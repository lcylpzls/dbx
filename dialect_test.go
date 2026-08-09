package dbx

import (
	"reflect"
	"testing"

	"github.com/lcylpzls/errx"
)

func TestDialectName(t *testing.T) {
	cases := []struct {
		d    Dialect
		want string
	}{
		{mysqlDialect{}, "mysql"},
		{sqliteDialect{}, "sqlite"},
		{pgDialect{}, "postgres"},
		{genericDialect{}, "generic"},
	}
	for _, tc := range cases {
		if got := tc.d.Name(); got != tc.want {
			t.Errorf("方言名不符：got %q, want %q", got, tc.want)
		}
	}
}

func TestDialectPlaceholder(t *testing.T) {
	for _, d := range []Dialect{mysqlDialect{}, sqliteDialect{}, genericDialect{}} {
		if d.Placeholder(0) != "?" || d.Placeholder(2) != "?" {
			t.Errorf("%s 占位符应恒为 ?", d.Name())
		}
	}
	if (pgDialect{}).Placeholder(0) != "$1" || (pgDialect{}).Placeholder(2) != "$3" {
		t.Error("PostgreSQL 占位符应按序号生成")
	}
}

func TestDialectQuoteIdent(t *testing.T) {
	cases := []struct {
		d    Dialect
		name string
		want string
	}{
		{mysqlDialect{}, "users", "`users`"},
		{mysqlDialect{}, "users.id", "`users`.`id`"},
		{sqliteDialect{}, "users", `"users"`},
		{pgDialect{}, "users.id", `"users"."id"`},
		{genericDialect{}, "users", "`users`"},
	}
	for _, tc := range cases {
		got, err := tc.d.QuoteIdent(tc.name)
		if err != nil || got != tc.want {
			t.Errorf("%s.QuoteIdent(%q) = %q, %v, want %q", tc.d.Name(), tc.name, got, err, tc.want)
		}
	}
	invalid := []string{"", "1abc", "a b", "a;b", "a-b"}
	for _, name := range invalid {
		for _, d := range []Dialect{mysqlDialect{}, pgDialect{}} {
			if _, err := d.QuoteIdent(name); !errx.Is(err, CodeBadArgument) {
				t.Errorf("%s.QuoteIdent(%q) 应返回 CodeBadArgument：%v", d.Name(), name, err)
			}
		}
	}
}

func TestDialectLimitOffset(t *testing.T) {
	cases := []struct {
		d        Dialect
		wantSQL  string
		wantArgs []any
	}{
		{mysqlDialect{}, "LIMIT ? OFFSET ?", []any{int64(10), int64(20)}},
		{sqliteDialect{}, "LIMIT ? OFFSET ?", []any{int64(10), int64(20)}},
		{pgDialect{}, "LIMIT $4 OFFSET $5", []any{int64(10), int64(20)}},
		{genericDialect{}, "LIMIT ? OFFSET ?", []any{int64(10), int64(20)}},
	}
	for _, tc := range cases {
		sqlText, args := tc.d.LimitOffset(3, 10, 20)
		if sqlText != tc.wantSQL || !reflect.DeepEqual(args, tc.wantArgs) {
			t.Errorf("%s.LimitOffset 结果不符：%q %v", tc.d.Name(), sqlText, args)
		}
	}
}

func TestDialectFor(t *testing.T) {
	if dialectFor("sqlite").Name() != "sqlite" {
		t.Error("已注册方言应命中注册表")
	}
	if dialectFor("dbxtest").Name() != "generic" {
		t.Error("未注册方言应回退 generic")
	}
}

func TestRegisterDialect(t *testing.T) {
	RegisterDialect("test-dialect", mysqlDialect{})
	if dialectFor("test-dialect").Name() != "mysql" {
		t.Error("RegisterDialect 后应能解析")
	}
}
