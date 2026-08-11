package core

import (
	"strings"

	"github.com/lcylpzls/errx"
)

// Query 是 dbx 中所有可执行查询的统一形态。
type Query interface {
	// SQL 返回 SQL 文本、绑定参数与构造错误。
	// 参数顺序与占位符严格对应;构造失败返回 DBX_BAD_ARGUMENT。
	SQL() (string, []any, error)
}

// rawQuery 包装固定 SQL,不解析、不校验。
type rawQuery struct {
	sql  string
	args []any
}

// SQL 返回原始 SQL 与参数。
func (q rawQuery) SQL() (string, []any, error) {
	return q.sql, q.args, nil
}

// Raw 包装固定 SQL,原样交给数据库执行。
func Raw(sql string, args ...any) Query {
	return rawQuery{sql: sql, args: args}
}

// Select 以原生 SQL 为主体,安全追加 WHERE / ORDER BY / LIMIT。
// SQL 主体必须非空;条件片段中的参数占位符统一写 "?"。
func Select(sql string) *SelectQuery {
	q := &SelectQuery{
		base:   sql,
		conds:  make([]condition, 0, 4),
		orders: make([]order, 0, 2),
	}
	if strings.TrimSpace(sql) == "" {
		q.err = errx.NewCodef(CodeBadArgument, "SQL 主体不能为空")
	}
	return q
}

// SelectQuery 是动态查询构造器,只扩展 WHERE / ORDER BY / LIMIT 三个层面。
type SelectQuery struct {
	base     string
	baseArgs []any
	conds    []condition
	orders   []order
	hasLimit bool
	limit    int64
	offset   int64
	err      error
}

// condition 是单个 WHERE 片段。
type condition struct {
	connector string // " AND " 或 " OR ",首个条件忽略
	sql       string
	args      []any
}

// order 是单个排序项。
type order struct {
	column string
	desc   bool
}

// Where 追加一个条件;非首个条件时自动使用 AND 连接。
func (q *SelectQuery) Where(cond string, args ...any) *SelectQuery {
	return q.addCond("", cond, args)
}

// And 以 AND 追加一个条件。
func (q *SelectQuery) And(cond string, args ...any) *SelectQuery {
	return q.addCond(" AND ", cond, args)
}

// Or 以 OR 追加一个条件。
func (q *SelectQuery) Or(cond string, args ...any) *SelectQuery {
	return q.addCond(" OR ", cond, args)
}

// In 追加 column IN (?, ?, ...) 条件。
func (q *SelectQuery) In(column string, values ...any) *SelectQuery {
	if len(values) == 0 {
		q.err = errx.NewCodef(CodeBadArgument, "IN 至少需要一个值")
		return q
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(values)), ", ")
	return q.addCond(" AND ", column+" IN ("+ph+")", values)
}

// Like 追加 column LIKE ? 条件,pattern 始终作为绑定参数。
func (q *SelectQuery) Like(column, pattern string) *SelectQuery {
	return q.addCond(" AND ", column+" LIKE ?", []any{pattern})
}

// Between 追加 column BETWEEN ? AND ? 条件。
func (q *SelectQuery) Between(column string, lo, hi any) *SelectQuery {
	return q.addCond(" AND ", column+" BETWEEN ? AND ?", []any{lo, hi})
}

// IsNull 追加 column IS NULL 条件。
func (q *SelectQuery) IsNull(column string) *SelectQuery {
	return q.addCond(" AND ", column+" IS NULL", nil)
}

// Args 追加 SQL 主体的绑定参数,顺序位于条件参数之前。
// 用于 INSERT / UPDATE 等需要主体参数且不含条件参数的场景。
func (q *SelectQuery) Args(args ...any) *SelectQuery {
	q.baseArgs = append(q.baseArgs, args...)
	return q
}

// OrderBy 追加排序项,列名在渲染时走白名单校验。
func (q *SelectQuery) OrderBy(column string, desc bool) *SelectQuery {
	q.orders = append(q.orders, order{column: column, desc: desc})
	return q
}

// Page 设置分页,页号从 1 开始。
func (q *SelectQuery) Page(page, size int) *SelectQuery {
	if page < 1 || size < 1 {
		q.err = errx.NewCodef(CodeBadArgument, "页号与每页大小必须大于 0")
		return q
	}
	return q.LimitOffset(int64(size), int64((page-1)*size))
}

// LimitOffset 设置 LIMIT / OFFSET,分页参数始终作为绑定参数。
func (q *SelectQuery) LimitOffset(limit, offset int64) *SelectQuery {
	if limit < 0 || offset < 0 {
		q.err = errx.NewCodef(CodeBadArgument, "limit/offset 不能为负数")
		return q
	}
	q.hasLimit = true
	q.limit = limit
	q.offset = offset
	return q
}

// SQL 按回退方言渲染查询;数据库执行时按实际方言渲染。
func (q *SelectQuery) SQL() (string, []any, error) {
	return q.render(genericDialect{})
}

// render 按指定方言渲染查询。
func (q *SelectQuery) render(d Dialect) (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	var b strings.Builder
	b.Grow(len(q.base) + 64)
	b.WriteString(q.base)
	if len(q.conds) > 0 {
		b.WriteString(" WHERE ")
		for i, c := range q.conds {
			if i > 0 {
				b.WriteString(c.connector)
			}
			b.WriteString(c.sql)
		}
	}
	if len(q.orders) > 0 {
		b.WriteString(" ORDER BY ")
		for i, o := range q.orders {
			if i > 0 {
				b.WriteString(", ")
			}
			ident, err := d.QuoteIdent(o.column)
			if err != nil {
				return "", nil, err
			}
			b.WriteString(ident)
			if o.desc {
				b.WriteString(" DESC")
			} else {
				b.WriteString(" ASC")
			}
		}
	}
	argCap := len(q.baseArgs)
	for _, c := range q.conds {
		argCap += len(c.args)
	}
	args := make([]any, 0, argCap)
	args = append(args, q.baseArgs...)
	for _, c := range q.conds {
		args = append(args, c.args...)
	}
	if q.hasLimit {
		frag, fragArgs := d.LimitOffset(len(args), q.limit, q.offset)
		b.WriteString(" ")
		b.WriteString(frag)
		args = append(args, fragArgs...)
	}
	sqlText := b.String()
	if d.Placeholder(0) != "?" {
		sqlText = convertPlaceholders(sqlText, d, len(args))
	}
	return sqlText, args, nil
}

// addCond 追加条件片段,并修正连接符。
func (q *SelectQuery) addCond(connector, cond string, args []any) *SelectQuery {
	if strings.TrimSpace(cond) == "" {
		q.err = errx.NewCodef(CodeBadArgument, "条件不能为空")
		return q
	}
	if len(q.conds) > 0 && connector == "" {
		connector = " AND "
	}
	q.conds = append(q.conds, condition{connector: connector, sql: cond, args: args})
	return q
}

// convertPlaceholders 将条件中的 "?" 按方言转换为占位符。
// 参数个数不足时剩余的 "?" 保持原样。
func convertPlaceholders(sqlText string, d Dialect, argCount int) string {
	var b strings.Builder
	idx := 0
	for i := 0; i < len(sqlText); i++ {
		if sqlText[i] == '?' && idx < argCount {
			b.WriteString(d.Placeholder(idx))
			idx++
			continue
		}
		b.WriteByte(sqlText[i])
	}
	return b.String()
}
