package dbx

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lcylpzls/errx"
)

// Dialect 定义方言层的最小能力,只覆盖无法用同一份 SQL 表达的最小差异。
type Dialect interface {
	// Name 返回方言名,如 "mysql" / "sqlite" / "postgres"。
	Name() string
	// Placeholder 返回第 index 个参数占位符(index 从 0 开始)。
	Placeholder(index int) string
	// QuoteIdent 白名单校验标识符并加引号,非法输入返回 DBX_BAD_ARGUMENT。
	QuoteIdent(name string) (string, error)
	// LimitOffset 生成分页片段并返回追加参数;
	// start 是分页参数在全部参数中的起始序号(供 PostgreSQL 计算 $n)。
	LimitOffset(start int, limit, offset int64) (string, []any)
}

// identPattern 是标识符白名单:字母/下划线开头,仅含字母、数字、下划线与点。
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// quoteIdent 白名单校验标识符,并按指定引号包裹各段。
func quoteIdent(name string, quote byte) (string, error) {
	if !identPattern.MatchString(name) {
		return "", errx.Newf(errx.KindInvalid, CodeBadArgument, "非法标识符 %q", name)
	}
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = string(quote) + part + string(quote)
	}
	return strings.Join(parts, "."), nil
}

// mysqlDialect 是 MySQL 方言。
type mysqlDialect struct{}

func (mysqlDialect) Name() string {
	return "mysql"
}

func (mysqlDialect) Placeholder(index int) string {
	return "?"
}

func (mysqlDialect) QuoteIdent(name string) (string, error) {
	return quoteIdent(name, '`')
}

func (mysqlDialect) LimitOffset(start int, limit, offset int64) (string, []any) {
	return "LIMIT ? OFFSET ?", []any{limit, offset}
}

// sqliteDialect 是 SQLite 方言。
type sqliteDialect struct{}

func (sqliteDialect) Name() string {
	return "sqlite"
}

func (sqliteDialect) Placeholder(index int) string {
	return "?"
}

func (sqliteDialect) QuoteIdent(name string) (string, error) {
	return quoteIdent(name, '"')
}

func (sqliteDialect) LimitOffset(start int, limit, offset int64) (string, []any) {
	return "LIMIT ? OFFSET ?", []any{limit, offset}
}

// pgDialect 是 PostgreSQL 方言。
type pgDialect struct{}

func (pgDialect) Name() string {
	return "postgres"
}

func (pgDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index+1)
}

func (pgDialect) QuoteIdent(name string) (string, error) {
	return quoteIdent(name, '"')
}

func (pgDialect) LimitOffset(start int, limit, offset int64) (string, []any) {
	return fmt.Sprintf("LIMIT $%d OFFSET $%d", start+1, start+2), []any{limit, offset}
}

// genericDialect 是未注册方言/测试驱动使用的回退方言,行为与 MySQL 一致。
type genericDialect struct{}

func (genericDialect) Name() string {
	return "generic"
}

func (genericDialect) Placeholder(index int) string {
	return "?"
}

func (genericDialect) QuoteIdent(name string) (string, error) {
	return quoteIdent(name, '`')
}

func (genericDialect) LimitOffset(start int, limit, offset int64) (string, []any) {
	return "LIMIT ? OFFSET ?", []any{limit, offset}
}

// dialects 是方言注册表,方言子包在 init 中注册。
var dialects = map[string]Dialect{}

func init() {
	RegisterDialect("mysql", mysqlDialect{})
	RegisterDialect("sqlite", sqliteDialect{})
	RegisterDialect("postgres", pgDialect{})
}

// RegisterDialect 注册方言,供 mysql / sqlite / pg 子包在 init 中调用。
func RegisterDialect(name string, d Dialect) {
	dialects[name] = d
}

// dialectFor 返回指定名称的方言,未注册时回退 genericDialect。
func dialectFor(name string) Dialect {
	if d, ok := dialects[name]; ok {
		return d
	}
	return genericDialect{}
}
