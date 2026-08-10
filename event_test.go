package dbx

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestEventHook(t *testing.T) {
	hook := &fakeQueryEventHook{}
	db, err := Open(context.Background(), "dbxtest", "x", WithEventHook(hook))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()

	if _, err := db.Exec(context.Background(), Raw("INSERT INTO t (v) VALUES (1)")); err != nil {
		t.Fatalf("Exec 失败：%v", err)
	}
	if _, err := db.Query(context.Background(), Raw("SELECT v FROM t")); err != nil {
		t.Fatalf("Query 失败：%v", err)
	}
	_, _ = db.QueryRow(context.Background(), Raw("SELECT v FROM t"))
	fake.set(fakeConfig{execErr: errors.New("执行失败")})
	defer fake.set(fakeConfig{})
	_, _ = db.Exec(context.Background(), Raw("INSERT INTO t (v) VALUES (2)"))

	events := hook.snapshot()
	if len(events) != 4 {
		t.Fatalf("期望 4 个事件，得到 %d：%+v", len(events), events)
	}
	var ops []string
	for _, e := range events {
		ops = append(ops, e.Operation)
	}
	if strings.Join(ops, ",") != "exec,query,query_row,exec" {
		t.Fatalf("操作序列不匹配：%v", ops)
	}
	if events[0].System == "" || events[0].Statement == "" {
		t.Fatalf("事件应携带 system 与 statement：%+v", events[0])
	}
	if events[3].Err == nil {
		t.Fatal("失败事件应携带错误")
	}
}

func TestEventHookWithTrace(t *testing.T) {
	hook := &fakeQueryEventHook{}
	trace := &fakeTraceHook{}
	db, err := Open(context.Background(), "dbxtest", "x",
		WithTraceHook(trace),
		WithEventHook(hook),
	)
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	_, _ = db.Exec(context.Background(), Raw("SELECT 1"))
	if len(hook.snapshot()) != 1 {
		t.Fatal("事件钩子应触发")
	}
	if len(trace.snapshot()) != 1 {
		t.Fatal("追踪钩子应触发")
	}
}

func TestNoEventHook(t *testing.T) {
	db, err := Open(context.Background(), "dbxtest", "x")
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	_, _ = db.Exec(context.Background(), Raw("SELECT 1"))
}

type fakeQueryEventHook struct {
	mu   sync.Mutex
	list []QueryEvent
}

func (h *fakeQueryEventHook) OnQueryEvent(_ context.Context, e QueryEvent) {
	h.mu.Lock()
	h.list = append(h.list, e)
	h.mu.Unlock()
}

func (h *fakeQueryEventHook) snapshot() []QueryEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]QueryEvent(nil), h.list...)
}

var _ = errors.New
