package dbx

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcylpzls/logx"
)

type logEntry struct {
	msg    string
	fields logx.FieldGroup
}

// fakeLogger 是 logx.Logger 的内存实现,用于断言日志输出。
type fakeLogger struct {
	mu    sync.Mutex
	debug []logEntry
	warn  []logEntry
}

func (l *fakeLogger) IsDebugEnabled() bool { return true }

func (l *fakeLogger) Debug(msg string, fields logx.FieldGroup) {
	l.mu.Lock()
	l.debug = append(l.debug, logEntry{msg: msg, fields: fields})
	l.mu.Unlock()
}

func (l *fakeLogger) Info(msg string, fields logx.FieldGroup) {}

func (l *fakeLogger) Warn(msg string, fields logx.FieldGroup) {
	l.mu.Lock()
	l.warn = append(l.warn, logEntry{msg: msg, fields: fields})
	l.mu.Unlock()
}

func (l *fakeLogger) Error(msg string, fields logx.FieldGroup) {}
func (l *fakeLogger) Panic(msg string, fields logx.FieldGroup) {}
func (l *fakeLogger) Fatal(msg string, fields logx.FieldGroup) {}
func (l *fakeLogger) Debugf(format string, args ...any)        {}
func (l *fakeLogger) Infof(format string, args ...any)         {}
func (l *fakeLogger) Warnf(format string, args ...any)         {}
func (l *fakeLogger) Errorf(format string, args ...any)        {}
func (l *fakeLogger) Panicf(format string, args ...any)        {}
func (l *fakeLogger) Fatalf(format string, args ...any)        {}
func (l *fakeLogger) WithContext(ctx context.Context) logx.Logger {
	return l
}
func (l *fakeLogger) WithField(key string, val any) logx.Logger {
	return l
}
func (l *fakeLogger) Sync() error  { return nil }
func (l *fakeLogger) Close() error { return nil }
func (l *fakeLogger) SafeExit(exitFunc func()) {
	exitFunc()
}

func (l *fakeLogger) debugCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.debug)
}

func (l *fakeLogger) warnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.warn)
}

func (l *fakeLogger) debugKeys(i int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return entryKeys(l.debug[i])
}

func (l *fakeLogger) warnKeys(i int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return entryKeys(l.warn[i])
}

func entryKeys(e logEntry) []string {
	keys := make([]string, 0, e.fields.Len())
	for i := 0; i < e.fields.Len(); i++ {
		keys = append(keys, e.fields.At(i).Key)
	}
	return keys
}

func hasKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

// fakeMetrics 是 Metrics 的内存实现,用于断言指标计数。
type fakeMetrics struct {
	mu        sync.Mutex
	counters  map[string]int
	durations map[string][]float64
}

func newFakeMetrics() *fakeMetrics {
	return &fakeMetrics{
		counters:  map[string]int{},
		durations: map[string][]float64{},
	}
}

func (m *fakeMetrics) key(name string, labels []string) string {
	return name + "/" + strings.Join(labels, ",")
}

func (m *fakeMetrics) IncCounter(name string, labels ...string) {
	m.mu.Lock()
	m.counters[m.key(name, labels)]++
	m.mu.Unlock()
}

func (m *fakeMetrics) ObserveDuration(name string, seconds float64, labels ...string) {
	m.mu.Lock()
	k := m.key(name, labels)
	m.durations[k] = append(m.durations[k], seconds)
	m.mu.Unlock()
}

func (m *fakeMetrics) counter(name string, labels ...string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[m.key(name, labels)]
}

func (m *fakeMetrics) durationCount(name string, labels ...string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.durations[m.key(name, labels)])
}

func TestObservabilityOptions(t *testing.T) {
	metrics := newFakeMetrics()
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x",
		WithLogger(&fakeLogger{}),
		WithLogSQL(true),
		WithLogArgs(true),
		WithMetrics(metrics))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	if !db.cfg.LogSQL || !db.cfg.LogArgs || db.cfg.Metrics != metrics {
		t.Errorf("可观测性选项未生效：%+v", db.cfg)
	}
}

func TestLogSQL(t *testing.T) {
	logger := &fakeLogger{}
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x", WithLogger(logger), WithLogSQL(true))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Exec(ctx, Raw(`UPDATE users SET name = ?`, "x")); err != nil {
		t.Fatalf("Exec 失败：%v", err)
	}
	if _, err := db.Query(ctx, Raw(`SELECT id FROM users`)); err != nil {
		t.Fatalf("Query 失败：%v", err)
	}
	if _, err := db.QueryRow(ctx, Raw(`SELECT id FROM users`)); err != nil {
		t.Fatalf("QueryRow 失败：%v", err)
	}
	if logger.debugCount() != 3 {
		t.Fatalf("应打印 3 条 SQL 日志,实际 %d", logger.debugCount())
	}
	if !hasKey(logger.debugKeys(0), "sql") || !hasKey(logger.debugKeys(0), "op") {
		t.Errorf("SQL 日志字段不符：%v", logger.debugKeys(0))
	}
	if logger.debugKeys(0)[0] != "op" {
		t.Errorf("SQL 日志应包含 op 字段：%v", logger.debugKeys(0))
	}
}

func TestLogSQLDisabled(t *testing.T) {
	logger := &fakeLogger{}
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x", WithLogger(logger))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	if _, err := db.Exec(context.Background(), Raw(`UPDATE users SET name = ?`, "x")); err != nil {
		t.Fatalf("Exec 失败：%v", err)
	}
	if logger.debugCount() != 0 {
		t.Errorf("关闭 SQL 打印后不应输出日志")
	}
}

func TestLogArgs(t *testing.T) {
	logger := &fakeLogger{}
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x",
		WithLogger(logger), WithLogSQL(true), WithLogArgs(true))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	if _, err := db.Exec(context.Background(), Raw(`UPDATE users SET name = ?`, "x")); err != nil {
		t.Fatalf("Exec 失败：%v", err)
	}
	keys := logger.debugKeys(0)
	if !hasKey(keys, "args") || len(keys) != 3 {
		t.Errorf("SQL 日志应包含 args 字段：%v", keys)
	}
}

func TestObserveSlow(t *testing.T) {
	logger := &fakeLogger{}
	metrics := newFakeMetrics()
	cfg := Config{
		Logger:             logger,
		Metrics:            metrics,
		SlowQueryThreshold: time.Nanosecond,
		LogArgs:            true,
	}
	observe(cfg, "exec", "SELECT 1", []any{1}, time.Now().Add(-time.Second), nil)
	if logger.warnCount() != 1 {
		t.Fatalf("应输出慢查询日志")
	}
	if !hasKey(logger.warnKeys(0), "duration") || !hasKey(logger.warnKeys(0), "sql") {
		t.Errorf("慢查询日志字段不符：%v", logger.warnKeys(0))
	}
	if !hasKey(logger.warnKeys(0), "args") || len(logger.warnKeys(0)) != 4 {
		t.Errorf("慢查询日志应包含 args 字段：%v", logger.warnKeys(0))
	}
	if metrics.counter("dbx.queries", "exec") != 1 ||
		metrics.counter("dbx.slow_queries", "exec") != 1 ||
		metrics.durationCount("dbx.duration", "exec") != 1 {
		t.Errorf("慢查询指标不符")
	}
}

func TestObserveNotSlow(t *testing.T) {
	logger := &fakeLogger{}
	metrics := newFakeMetrics()
	cfg := Config{
		Logger:             logger,
		Metrics:            metrics,
		SlowQueryThreshold: time.Hour,
	}
	observe(cfg, "query", "SELECT 1", nil, time.Now(), nil)
	if logger.warnCount() != 0 {
		t.Error("未超阈值不应输出慢查询日志")
	}
	if metrics.counter("dbx.slow_queries", "query") != 0 {
		t.Error("未超阈值不应记录慢查询指标")
	}
	if metrics.counter("dbx.queries", "query") != 1 {
		t.Error("应记录查询计数")
	}
}

func TestObserveDefaultThreshold(t *testing.T) {
	logger := &fakeLogger{}
	metrics := newFakeMetrics()
	cfg := Config{Logger: logger, Metrics: metrics}
	observe(cfg, "exec", "SELECT 1", nil, time.Now().Add(-time.Second), nil)
	if logger.warnCount() != 1 {
		t.Error("默认阈值下超时查询应输出慢查询日志")
	}
}

func TestObserveNoConfig(t *testing.T) {
	observe(Config{}, "exec", "SELECT 1", nil, time.Now(), nil)
	observe(Config{}, "exec", "SELECT 1", nil, time.Now(), errors.New("失败"))
}

func TestMetricsCounters(t *testing.T) {
	metrics := newFakeMetrics()
	fake.set(fakeConfig{columns: []string{"id"}})
	db, err := Open(context.Background(), "dbxtest", "x", WithMetrics(metrics))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	ctx := context.Background()
	if _, err := db.QueryRow(ctx, Raw(`SELECT id FROM users`)); err != nil {
		t.Fatalf("QueryRow 失败：%v", err)
	}
	if err := db.BatchExec(ctx, `INSERT INTO users (id) VALUES (?)`, [][]any{{int64(1)}}); err != nil {
		t.Fatalf("BatchExec 失败：%v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close 失败：%v", err)
	}

	fake.set(fakeConfig{
		execErr:  errors.New("执行失败"),
		queryErr: errors.New("查询失败"),
		columns:  []string{"id"},
	})
	db2, err := Open(context.Background(), "dbxtest", "x", WithMetrics(metrics))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db2.Close()
	if _, err := db2.Exec(ctx, Raw(`UPDATE users SET name = ?`, "x")); err == nil {
		t.Fatal("Exec 应失败")
	}
	if _, err := db2.Query(ctx, Raw(`SELECT id FROM users`)); err == nil {
		t.Fatal("Query 应失败")
	}
	if metrics.counter("dbx.queries", "exec") != 1 ||
		metrics.counter("dbx.queries", "query") != 1 ||
		metrics.counter("dbx.queries", "query_row") != 1 ||
		metrics.counter("dbx.queries", "batch") != 1 {
		t.Errorf("查询计数不符：%+v", metrics.counters)
	}
	if metrics.counter("dbx.errors", "exec") != 1 || metrics.counter("dbx.errors", "query") != 1 {
		t.Errorf("错误计数不符：%+v", metrics.counters)
	}
	if metrics.durationCount("dbx.duration", "exec") != 1 ||
		metrics.durationCount("dbx.duration", "batch") != 1 {
		t.Errorf("耗时计数不符：%+v", metrics.durations)
	}
}

func TestTxObservability(t *testing.T) {
	logger := &fakeLogger{}
	metrics := newFakeMetrics()
	fake.set(fakeConfig{})
	db, err := Open(context.Background(), "dbxtest", "x",
		WithLogger(logger), WithLogSQL(true), WithMetrics(metrics))
	if err != nil {
		t.Fatalf("Open 失败：%v", err)
	}
	defer db.Close()
	if err := db.WithTx(context.Background(), func(tx *Tx) error {
		_, err := tx.Exec(context.Background(), Raw(`UPDATE users SET name = ?`, "x"))
		return err
	}); err != nil {
		t.Fatalf("WithTx 失败：%v", err)
	}
	if logger.debugCount() != 1 {
		t.Errorf("事务内应打印 SQL 日志")
	}
	if metrics.counter("dbx.queries", "exec") != 1 {
		t.Errorf("事务内应记录查询指标")
	}
}

func TestTruncateSQL(t *testing.T) {
	short := strings.Repeat("a", 512)
	if got := truncateSQL(short); got != short {
		t.Error("短 SQL 不应截断")
	}
	long := strings.Repeat("a", 600)
	got := truncateSQL(long)
	if len(got) != 515 || !strings.HasSuffix(got, "...") {
		t.Errorf("长 SQL 截断不符：len=%d", len(got))
	}
	chinese := strings.Repeat("张", 600)
	got = truncateSQL(chinese)
	if len([]rune(got)) != 515 {
		t.Errorf("中文 SQL 截断应按字符计：%d", len([]rune(got)))
	}
	byteLong := strings.Repeat("张", 300)
	if got := truncateSQL(byteLong); got != byteLong {
		t.Error("字节超长但字符数未超时不应截断")
	}
}

var _ logx.Logger = (*fakeLogger)(nil)
