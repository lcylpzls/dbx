package migrate

import "strings"

// splitStatements 按分号拆分 SQL 语句,
// 跳过单引号字符串、双引号标识符、反引号标识符与注释中的分号。
func splitStatements(src string) []string {
	var stmts []string
	var cur strings.Builder
	runes := []rune(src)
	inSingle := false
	inDouble := false
	inBacktick := false
	lineComment := false
	blockComment := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case lineComment:
			cur.WriteRune(r)
			if r == '\n' {
				lineComment = false
			}
		case blockComment:
			cur.WriteRune(r)
			if r == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				cur.WriteRune(runes[i+1])
				i++
				blockComment = false
			}
		case inSingle:
			cur.WriteRune(r)
			if r == '\'' {
				inSingle = false
			}
		case inDouble:
			cur.WriteRune(r)
			if r == '"' {
				inDouble = false
			}
		case inBacktick:
			cur.WriteRune(r)
			if r == '`' {
				inBacktick = false
			}
		case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
			lineComment = true
			cur.WriteRune(r)
			cur.WriteRune(runes[i+1])
			i++
		case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
			blockComment = true
			cur.WriteRune(r)
			cur.WriteRune(runes[i+1])
			i++
		case r == '\'':
			inSingle = true
			cur.WriteRune(r)
		case r == '"':
			inDouble = true
			cur.WriteRune(r)
		case r == '`':
			inBacktick = true
			cur.WriteRune(r)
		case r == ';':
			if stmt := strings.TrimSpace(cur.String()); stmt != "" {
				stmts = append(stmts, stmt)
			}
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if stmt := strings.TrimSpace(cur.String()); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}
