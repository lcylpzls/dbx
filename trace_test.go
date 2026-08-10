package dbx

import (
	"context"
	testx "github.com/lcylpzls/testx"
	"sync"
	"testing"
)

// traceCall 记录一次追踪调用。
type traceCall struct {
	name  string
	attrs map[string]string
	err   error
	ended bool
}

// fakeTraceHook 内存追踪钩子。
type fakeTraceHook struct {
	mu    sync.Mutex
	calls []traceCall
}

func (h *fakeTraceHook) Start(ctx context.Context, name string, attrs ...TraceAttr) (context.Context, func(error)) {
	h.mu.Lock()
	h.calls = append(h.calls, traceCall{name: name, attrs: map[string]string{}})
	for _, a := range attrs {
		h.calls[len(h.calls)-1].attrs[a.Key] = a.Value
	}
	h.mu.Unlock()
	return ctx, func(err error) {
		h.mu.Lock()
		h.calls[len(h.calls)-1].err = err
		h.calls[len(h.calls)-1].ended = true
		h.mu.Unlock()
	}
}

func (h *fakeTraceHook) snapshot() []traceCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]traceCall, len(h.calls))
	copy(out, h.calls)
	return out
}

// TestTraceHook 覆盖 Exec/Query/QueryRow 追踪埋点。
func TestTraceHook(t *testing.T) {
	fake.set(fakeConfig{})
	hook := &fakeTraceHook{}
	db, err := Open(context.Background(), "dbxtest", "test", WithTraceHook(hook))
	testx.RequireNoError(t, err)

	defer db.Close()

	_, _ = db.Exec(context.Background(), Raw("INSERT INTO t VALUES(1)"))
	_, _ = db.Query(context.Background(), Raw("SELECT * FROM t"))
	_, _ = db.QueryRow(context.Background(), Raw("SELECT * FROM t WHERE id=1"))

	calls := hook.snapshot()
	if len(calls) != 3 {
		t.Fatalf("应调用 3 次追踪钩子，实际：%d", len(calls))
	}
	want := []struct{ name, op string }{
		{"dbx.exec", "exec"},
		{"dbx.query", "query"},
		{"dbx.query_row", "query_row"},
	}
	for i, w := range want {
		testx.RequireEqual(t, calls[i].name, w.name)

		if calls[i].attrs["db.operation"] != w.op || calls[i].attrs["db.system"] == "" ||
			calls[i].attrs["db.statement"] == "" {
			t.Fatalf("第 %d 次属性不符：%+v", i, calls[i].attrs)
		}
		if !calls[i].ended {
			t.Fatalf("第 %d 次结束回调应被调用", i)
		}
	}
}
