package redis

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"klyradb/internal/engine"
)

func TestNew(t *testing.T) {
	if New() == nil {
		t.Error("New() returned nil")
	}
}

func TestDBType(t *testing.T) {
	if New().DBType() != engine.TypeRedis {
		t.Errorf("DBType() = %q, want %q", New().DBType(), engine.TypeRedis)
	}
}

func TestVersions_ReturnsNonEmpty(t *testing.T) {
	versions := New().Versions()
	if len(versions) == 0 {
		t.Fatal("Versions() returned empty list")
	}
	for _, v := range versions {
		if v.Type != engine.TypeRedis {
			t.Errorf("version type = %q, want %q", v.Type, engine.TypeRedis)
		}
		if v.Major == "" {
			t.Error("version has empty Major")
		}
		if v.Label == "" {
			t.Error("version has empty Label")
		}
	}
}

func TestVersions_SnapNoBinaries(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	for _, v := range New().Versions() {
		if v.Installed {
			t.Errorf("version %s: Installed=true in empty snap dir", v.Major)
		}
	}
}

func TestVersions_SnapWithMockBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script mock binaries not supported on Windows")
	}
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "usr/bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho 'Redis server v=7.4.0 sha=00000000:0 malloc=libc bits=64 build=0'\n"
	if err := os.WriteFile(filepath.Join(binDir, "redis-server"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP", tmp)

	// Verify binary is found in snap context
	bin := findBinary("redis-server")
	if bin == "" {
		t.Fatal("findBinary returned empty with mock binary in snap dir")
	}
	// Verify version detection works
	ver := detectRedisVersion(bin)
	if ver == "" {
		t.Fatal("detectRedisVersion returned empty for mock binary")
	}
	// Verify Versions() doesn't panic and returns non-empty list
	versions := New().Versions()
	if len(versions) == 0 {
		t.Error("Versions() returned empty list")
	}
}

func TestCreate_SnapBinaryNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	inst := &engine.Instance{
		DataDir:  filepath.Join(tmp, "data"),
		LogFile:  filepath.Join(tmp, "logs", "redis.log"),
		PIDFile:  filepath.Join(tmp, "redis.pid"),
		ConfFile: filepath.Join(tmp, "redis.conf"),
		Port:     6379,
		User:     "test",
	}
	if err := New().Create(inst); err == nil {
		t.Error("Create() expected error when redis-server not in snap")
	}
}

func TestCreate_WritesConf(t *testing.T) {
	tmp := t.TempDir()
	// Create mock redis-server binary
	binDir := filepath.Join(tmp, "usr/bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "redis-server"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP", tmp)

	inst := &engine.Instance{
		DataDir:  filepath.Join(tmp, "data"),
		LogFile:  filepath.Join(tmp, "logs", "redis.log"),
		PIDFile:  filepath.Join(tmp, "redis.pid"),
		ConfFile: filepath.Join(tmp, "redis.conf"),
		Port:     6380,
		User:     "test",
	}
	if err := New().Create(inst); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	conf, err := os.ReadFile(inst.ConfFile)
	if err != nil {
		t.Fatalf("conf file not written: %v", err)
	}
	content := string(conf)
	if !strings.Contains(content, "port 6380") {
		t.Error("conf missing port")
	}
	if !strings.Contains(content, "bind 127.0.0.1") {
		t.Error("conf missing bind")
	}
	if !strings.Contains(content, "daemonize yes") {
		t.Error("conf missing daemonize")
	}
}

func TestCheckStatus_Stopped(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		PIDFile: filepath.Join(tmp, "redis.pid"),
		Port:    59911,
	}
	if status := New().CheckStatus(inst); status != engine.StatusStopped {
		t.Errorf("CheckStatus() = %q, want %q", status, engine.StatusStopped)
	}
}

func TestDelete_NoDataDir(t *testing.T) {
	inst := &engine.Instance{
		DataDir:  "/tmp/klyradb_redis_test_nonexistent_98765",
		LogFile:  "/tmp/klyradb_redis_test_nonexistent_98765.log",
		PIDFile:  "/tmp/klyradb_redis_test_nonexistent_98765.pid",
		ConfFile: "/tmp/klyradb_redis_test_nonexistent_98765.conf",
		Port:     59912,
	}
	if err := New().Delete(inst); err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

func TestFindBinary_SnapContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	// No binary → empty string
	if got := findBinary("redis-server"); got != "" {
		t.Errorf("findBinary in empty snap = %q, want empty", got)
	}

	// Create binary → found
	binDir := filepath.Join(tmp, "usr/bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "redis-server"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findBinary("redis-server"); got == "" {
		t.Error("findBinary in snap with binary = empty, want path")
	}
}

func TestDetectRedisVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script mock binaries not supported on Windows")
	}
	// detectRedisVersion parses "Redis server v=7.4.0 sha=..."
	// We can test by creating a mock binary in a temp dir
	tmp := t.TempDir()
	script := "#!/bin/sh\necho 'Redis server v=7.4.0 sha=00000000:0 malloc=libc bits=64 build=0'\n"
	bin := filepath.Join(tmp, "redis-server")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ver := detectRedisVersion(bin)
	if !strings.HasPrefix(ver, "7.4") {
		t.Errorf("detectRedisVersion = %q, want 7.4.x", ver)
	}
}
