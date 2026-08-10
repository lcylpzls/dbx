package dbx

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/lcylpzls/testx"
)

func TestEventHook(t *testing.T) {
	hook := &fakeQueryEventHook{}
	db, err := Open(context.Background(), "dbxtest", "x", WithEventHook(hook))
	testx.RequireNoError(t, err)
	defer db.Close()

	_, err = db.Exec(context.Background(), Raw("INSERT INTO t (v) VALUES (1)"))
	testx.RequireNoError(t, err)
	_, err = db.Query(context.Background(), Raw("SELECT v FROM t"))
	testx.RequireNoError(t, err)
	_, _ = db.QueryRow(context.Background(), Raw("SELECT v FROM t"))
	fake.set(fakeConfig{execErr: errors.New("执行失败")})
	defer fake.set(fakeConfig{})
	_, _ = db.Exec(context.Background(), Raw("INSERT INTO t (v) VALUES (2)"))

	events := hook.snapshot()
	testx.RequireLen(t, events, 4)
	var ops []string
	for _, e := range events {
		ops = append(ops, e.Operation)
	}
	testx.RequireEqual(t, strings.Join(ops, ","), "exec,query,query_row,exec")
	testx.RequireNotEmpty(t, events[0].System)
	testx.RequireNotEmpty(t, events[0].Statement)
	testx.RequireNotNil(t, events[3].Err)
}

func TestEventHookWithTrace(t *testing.T) {
	hook := &fakeQueryEventHook{}
	trace := &fakeTraceHook{}
	db, err := Open(context.Background(), "dbxtest", "x",
		WithTraceHook(trace),
		WithEventHook(hook),
	)
	testx.RequireNoError(t, err)
	defer db.Close()
	_, _ = db.Exec(context.Background(), Raw("SELECT 1"))
	testx.RequireLen(t, hook.snapshot(), 1)
	testx.RequireLen(t, trace.snapshot(), 1)
}

func TestNoEventHook(t *testing.T) {
	db, err := Open(context.Background(), "dbxtest", "x")
	testx.RequireNoError(t, err)
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
