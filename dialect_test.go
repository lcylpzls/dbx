package dbx

import (
	"testing"

	"github.com/lcylpzls/testx"
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
		testx.RequireEqual(t, tc.d.Name(), tc.want)
	}
}

func TestDialectPlaceholder(t *testing.T) {
	for _, d := range []Dialect{mysqlDialect{}, sqliteDialect{}, genericDialect{}} {
		testx.RequireEqual(t, d.Placeholder(0), "?")
		testx.RequireEqual(t, d.Placeholder(2), "?")
	}
	testx.RequireEqual(t, (pgDialect{}).Placeholder(0), "$1")
	testx.RequireEqual(t, (pgDialect{}).Placeholder(2), "$3")
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
		testx.RequireNoError(t, err)
		testx.RequireEqual(t, got, tc.want)
	}
	invalid := []string{"", "1abc", "a b", "a;b", "a-b"}
	for _, name := range invalid {
		for _, d := range []Dialect{mysqlDialect{}, pgDialect{}} {
			_, err := d.QuoteIdent(name)
			testx.RequireErrCode(t, err, CodeBadArgument)
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
		testx.RequireEqual(t, sqlText, tc.wantSQL)
		testx.RequireEqual(t, args, tc.wantArgs)
	}
}

func TestDialectFor(t *testing.T) {
	testx.RequireEqual(t, dialectFor("sqlite").Name(), "sqlite")
	testx.RequireEqual(t, dialectFor("dbxtest").Name(), "generic")
}

func TestRegisterDialect(t *testing.T) {
	RegisterDialect("test-dialect", mysqlDialect{})
	testx.RequireEqual(t, dialectFor("test-dialect").Name(), "mysql")
}
