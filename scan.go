package dbx

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/lcylpzls/errx"
)

// Row 是单行结果的最小视图,可由 *sql.Rows 或自定义行实现。
type Row interface {
	Columns() ([]string, error)
	Scan(dest ...any) error
}

// RowMapper 将一行原始列值映射为自定义结果。
type RowMapper[T any] func(row Row) (T, error)

// One 扫描单条记录到结构体;无数据返回 DBX_NOT_FOUND。
func One[T any](ctx context.Context, runner QueryRunner, q Query) (T, error) {
	return one[T](ctx, runner, q, nil)
}

// OneWith 使用自定义映射扫描单条记录;无数据返回 DBX_NOT_FOUND。
func OneWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) (T, error) {
	return one[T](ctx, runner, q, mapper)
}

// List 扫描多条记录到结构体切片;无数据返回空切片而非 nil。
func List[T any](ctx context.Context, runner QueryRunner, q Query) ([]T, error) {
	return list[T](ctx, runner, q, nil)
}

// ListWith 使用自定义映射扫描多条记录;无数据返回空切片而非 nil。
func ListWith[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) ([]T, error) {
	return list[T](ctx, runner, q, mapper)
}

// QueryRunner 是可执行查询的最小接口,DB 与 Tx 都实现它。
type QueryRunner interface {
	Query(ctx context.Context, q Query) (*sql.Rows, error)
}

func one[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) (T, error) {
	var zero T
	rows, err := runner.Query(ctx, q)
	if err != nil {
		return zero, err
	}
	defer rows.Close()
	return scanOne[T](rows, mapper)
}

func list[T any](ctx context.Context, runner QueryRunner, q Query, mapper RowMapper[T]) ([]T, error) {
	rows, err := runner.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanList[T](rows, mapper)
}

// scanOne 扫描单行;无数据时优先返回结果集错误,否则返回 DBX_NOT_FOUND。
func scanOne[T any](rows *sql.Rows, mapper RowMapper[T]) (T, error) {
	var zero T
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, errx.WrapCode(err, CodeQueryFailed, "读取查询结果失败")
		}
		return zero, errx.WrapCode(sql.ErrNoRows, CodeNotFound, "查询无结果")
	}
	if mapper != nil {
		v, err := mapper(rows)
		if err != nil {
			return zero, errx.WrapCode(err, CodeScanFailed, "自定义映射失败")
		}
		return v, nil
	}
	var out T
	if err := scanStruct(rows, &out); err != nil {
		return zero, err
	}
	return out, nil
}

// scanList 扫描多行;无数据返回空切片而非 nil。
func scanList[T any](rows *sql.Rows, mapper RowMapper[T]) ([]T, error) {
	var out []T
	for rows.Next() {
		if mapper != nil {
			v, err := mapper(rows)
			if err != nil {
				return nil, errx.WrapCode(err, CodeScanFailed, "自定义映射失败")
			}
			out = append(out, v)
		} else {
			var item T
			if err := scanStruct(rows, &item); err != nil {
				return nil, err
			}
			out = append(out, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errx.WrapCode(err, CodeQueryFailed, "读取查询结果失败")
	}
	if out == nil {
		out = []T{}
	}
	return out, nil
}

// scanStruct 按 db tag 将一行列值绑定到结构体。
// 列多出/缺少均可;NULL 按目标类型归零;类型不匹配返回 DBX_SCAN_FAILED。
func scanStruct(row Row, out any) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errx.NewCode(CodeBadArgument, "扫描目标必须是非空指针")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errx.NewCode(CodeBadArgument, "扫描目标必须是结构体")
	}
	columns, err := row.Columns()
	if err != nil {
		return errx.WrapCode(err, CodeQueryFailed, "读取列信息失败")
	}
	meta := scanMeta(rv.Type())
	slots := make([]scanSlot, len(columns))
	dest := make([]any, len(columns))
	for i, col := range columns {
		slots[i].field = matchField(meta, col)
		dest[i] = &slots[i].value
	}
	if err := row.Scan(dest...); err != nil {
		return errx.WrapCode(err, CodeScanFailed, "扫描行数据失败")
	}
	for i, col := range columns {
		if slots[i].field == nil {
			continue
		}
		field := fieldByIndexAlloc(rv, slots[i].field.index)
		if err := assignValue(field, slots[i].value, col); err != nil {
			return err
		}
	}
	return nil
}

// scanSlot 是单列扫描槽位,合并临时值与字段元信息以减少分配。
type scanSlot struct {
	field *fieldMeta
	value any
}

// fieldMeta 描述结构体字段的扫描目标。
type fieldMeta struct {
	index  []int
	column string
	fold   bool // 无 db tag 时按字段名与列名不区分大小写匹配
}

// scanMetaCache 缓存类型到字段元信息的映射,避免热路径重复反射。
var scanMetaCache sync.Map

// scanMeta 返回结构体类型的字段元信息(带缓存)。
func scanMeta(t reflect.Type) []fieldMeta {
	if cached, ok := scanMetaCache.Load(t); ok {
		return cached.([]fieldMeta)
	}
	meta := collectMeta(t, nil)
	actual, _ := scanMetaCache.LoadOrStore(t, meta)
	return actual.([]fieldMeta)
}

// collectMeta 递归收集结构体字段,嵌入结构体按字段路径展开。
func collectMeta(t reflect.Type, prefix []int) []fieldMeta {
	var out []fieldMeta
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		tag := f.Tag.Get("db")
		if tag == "-" {
			continue
		}
		index := append(append([]int(nil), prefix...), i)
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				// 未导出的嵌入指针无法通过反射分配,直接跳过。
				if f.Type.Kind() == reflect.Ptr && f.PkgPath != "" {
					continue
				}
				out = append(out, collectMeta(ft, index)...)
				continue
			}
		}
		out = append(out, fieldMeta{
			index:  index,
			column: fieldColumn(f, tag),
			fold:   tag == "",
		})
	}
	return out
}

// fieldColumn 返回字段的 db 列名:有 tag 用 tag,无 tag 用字段名。
func fieldColumn(f reflect.StructField, tag string) string {
	if tag != "" {
		return tag
	}
	return f.Name
}

// matchField 按列名匹配字段:带 tag 精确匹配,无 tag 不区分大小写。
func matchField(meta []fieldMeta, column string) *fieldMeta {
	for i := range meta {
		if meta[i].column == column {
			return &meta[i]
		}
	}
	for i := range meta {
		if meta[i].fold && strings.EqualFold(meta[i].column, column) {
			return &meta[i]
		}
	}
	return nil
}

// fieldByIndexAlloc 按字段路径取值,沿途自动分配为 nil 的嵌入指针。
func fieldByIndexAlloc(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

// assignValue 将驱动值转换并设置到字段;NULL 按目标类型归零。
func assignValue(field reflect.Value, v any, column string) error {
	if v == nil {
		if field.Kind() == reflect.Ptr {
			field.Set(reflect.Zero(field.Type()))
		}
		return nil
	}
	if field.Kind() == reflect.Ptr {
		pv := reflect.New(field.Type().Elem())
		if err := assignValue(pv.Elem(), v, column); err != nil {
			return err
		}
		field.Set(pv)
		return nil
	}
	if sc, ok := field.Addr().Interface().(sql.Scanner); ok {
		if err := sc.Scan(v); err != nil {
			return errx.NewCodef(CodeScanFailed,
				"列 %s 的值 %T 无法扫描到 %s：%v", column, v, field.Type(), err)
		}
		return nil
	}
	if field.Type() == reflect.TypeOf(time.Time{}) {
		tm, err := parseTimeValue(v)
		if err != nil {
			return scanTypeError(column, field.Type(), v)
		}
		field.Set(reflect.ValueOf(tm))
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		switch sv := v.(type) {
		case string:
			field.SetString(sv)
		case []byte:
			field.SetString(string(sv))
		default:
			return scanTypeError(column, field.Type(), v)
		}
	case reflect.Bool:
		if b, ok := v.(bool); ok {
			field.SetBool(b)
		} else {
			return scanTypeError(column, field.Type(), v)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch n := v.(type) {
		case int64:
			field.SetInt(n)
		case int:
			field.SetInt(int64(n))
		default:
			return scanTypeError(column, field.Type(), v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch n := v.(type) {
		case uint64:
			field.SetUint(n)
		case int64:
			if n < 0 {
				return scanTypeError(column, field.Type(), v)
			}
			field.SetUint(uint64(n))
		default:
			return scanTypeError(column, field.Type(), v)
		}
	case reflect.Float32, reflect.Float64:
		switch f := v.(type) {
		case float64:
			field.SetFloat(f)
		case int64:
			field.SetFloat(float64(f))
		default:
			return scanTypeError(column, field.Type(), v)
		}
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.Uint8 {
			switch bv := v.(type) {
			case []byte:
				field.SetBytes(bv)
			case string:
				field.SetBytes([]byte(bv))
			default:
				return scanTypeError(column, field.Type(), v)
			}
			return nil
		}
		return errx.NewCodef(CodeScanFailed,
			"列 %s 不支持扫描到类型 %s", column, field.Type())
	default:
		return errx.NewCodef(CodeScanFailed,
			"列 %s 不支持扫描到类型 %s", column, field.Type())
	}
	return nil
}

// scanTypeError 构造类型不匹配错误,包含列名与目标类型。
func scanTypeError(column string, typ reflect.Type, v any) error {
	return errx.NewCodef(CodeScanFailed,
		"列 %s 的值 %T 无法扫描到 %s", column, v, typ)
}

// parseTimeValue 将驱动值转换为 time.Time。
func parseTimeValue(v any) (time.Time, error) {
	switch tv := v.(type) {
	case time.Time:
		return tv, nil
	case string:
		return parseTimeString(tv)
	case []byte:
		return parseTimeString(string(tv))
	default:
		return time.Time{}, errx.NewCodef(CodeScanFailed, "不支持的时间值 %T", v)
	}
}

// parseTimeString 按常见数据库时间格式解析字符串。
func parseTimeString(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if tm, err := time.Parse(layout, s); err == nil {
			return tm, nil
		}
	}
	return time.Time{}, errx.NewCodef(CodeScanFailed, "无法解析时间字符串 %q", s)
}
