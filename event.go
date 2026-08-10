package dbx

import "context"

// QueryEvent 描述一次 SQL 执行事件。
type QueryEvent struct {
	// System 数据库方言（mysql / postgres / sqlite）。
	System string
	// Operation 操作类型：exec / query / query_row。
	Operation string
	// Statement SQL 语句。
	Statement string
	// Err 执行结果错误；nil 表示成功。
	Err error
}

// EventHook 是可选事件钩子（默认 no-op），由 eventx 等外部适配器接入。
type EventHook interface {
	// OnQueryEvent 在 SQL 执行结束时调用。
	OnQueryEvent(ctx context.Context, e QueryEvent)
}
