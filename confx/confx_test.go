package confx

import (
	"context"
	testx "github.com/lcylpzls/testx"
	"os"
	"path/filepath"
	"testing"
	"time"

	confxlib "github.com/lcylpzls/confx"
	"github.com/lcylpzls/dbx"
	_ "github.com/lcylpzls/dbx/sqlite"
	"github.com/lcylpzls/errx"
)

const validTOML = `
driver = "sqlite"
dsn = "test.db"
max_open_conns = 5
max_idle_conns = 2
conn_max_lifetime = "30s"
conn_max_idle_time = "10s"
slow_query_threshold = "50ms"
log_sql = true
log_args = true
`

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写配置文件失败：%v", err)
	}
	return path
}

func TestLoadFile(t *testing.T) {
	cfg, err := LoadFile(writeTOML(t, validTOML))
	testx.RequireNoError(t, err)

	if cfg.Driver != "sqlite" || cfg.DSN != "test.db" ||
		cfg.MaxOpenConns != 5 || cfg.MaxIdleConns != 2 ||
		cfg.ConnMaxLifetime != 30*time.Second ||
		cfg.ConnMaxIdleTime != 10*time.Second ||
		cfg.SlowQueryThreshold != 50*time.Millisecond ||
		!cfg.LogSQL || !cfg.LogArgs {
		t.Errorf("配置解析不符：%+v", cfg)
	}
}

func TestLoadFileMissingDuration(t *testing.T) {
	cfg, err := LoadFile(writeTOML(t, `driver = "sqlite"
dsn = "test.db"`))
	testx.RequireNoError(t, err)

	if cfg.ConnMaxLifetime != 0 || cfg.ConnMaxIdleTime != 0 || cfg.SlowQueryThreshold != 0 {
		t.Errorf("缺失时长应为 0：%+v", cfg)
	}
}

func TestLoadFileUnknownKey(t *testing.T) {
	_, err := LoadFile(writeTOML(t, `driver = "sqlite"
dsn = "test.db"
unknown = 1`))
	if err == nil || !errx.Is(err, confxlib.CodeUnknownKey) {
		t.Errorf("未声明字段应返回错误：%v", err)
	}
}

func TestLoadFileMissingFile(t *testing.T) {
	_, err := LoadFile(filepath.Join(t.TempDir(), "不存在.toml"))
	testx.RequireError(t, err)

}

func TestLoadFileInvalidDuration(t *testing.T) {
	cases := []string{
		`conn_max_lifetime = "abc"`,
		`conn_max_idle_time = "abc"`,
		`slow_query_threshold = "abc"`,
	}
	for _, extra := range cases {
		_, err := LoadFile(writeTOML(t, `driver = "sqlite"
dsn = "test.db"
`+extra))
		if !errx.Is(err, dbx.CodeBadArgument) {
			t.Errorf("非法时长 %q 错误码不符：%v", extra, err)
		}
	}
}

func TestOpen(t *testing.T) {
	dsn := filepath.ToSlash(filepath.Join(t.TempDir(), "open.db"))
	path := writeTOML(t, `driver = "sqlite"
dsn = "`+dsn+`"`)
	db, err := Open(context.Background(), path)
	testx.RequireNoError(t, err)

	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatalf("Ping 失败：%v", err)
	}
}

func TestOpenInvalidDSN(t *testing.T) {
	dsn := filepath.ToSlash(filepath.Join(t.TempDir(), "不存在目录", "x.db"))
	path := writeTOML(t, `driver = "sqlite"
dsn = "`+dsn+`"`)
	db, err := Open(context.Background(), path)
	if err == nil {
		db.Close()
		t.Fatal("无效 DSN 应返回错误")
	}
}

func TestOpenLoadFileError(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "不存在.toml"))
	if err == nil {
		db.Close()
		t.Fatal("缺失配置文件应返回错误")
	}
}
