package mariadb

import (
	"os"
	"path/filepath"
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
	if New().DBType() != engine.TypeMariaDB {
		t.Errorf("DBType() = %q, want %q", New().DBType(), engine.TypeMariaDB)
	}
}

func TestVersions_ReturnsNonEmpty(t *testing.T) {
	versions := New().Versions()
	if len(versions) == 0 {
		t.Fatal("Versions() returned empty list")
	}
	for _, v := range versions {
		if v.Type != engine.TypeMariaDB {
			t.Errorf("version type = %q, want %q", v.Type, engine.TypeMariaDB)
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
	tmp := t.TempDir()
	sbinDir := filepath.Join(tmp, "opt/klyra-mariadb/sbin")
	if err := os.MkdirAll(sbinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho '/opt/klyra-mariadb/sbin/mariadbd  Ver 10.11.6-MariaDB Distrib 10.11.6-MariaDB, for debian-linux-gnu (x86_64)'\n"
	if err := os.WriteFile(filepath.Join(sbinDir, "mariadbd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP", tmp)

	// Verify binary is found in snap context
	bin := findBinary("mariadbd")
	if bin == "" {
		t.Fatal("findBinary returned empty with mock binary in snap dir")
	}
	// Verify version detection works and identifies MariaDB
	ver := detectMariaDBVersion(bin)
	if ver == "" {
		t.Fatal("detectMariaDBVersion returned empty for mock MariaDB binary")
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
		LogFile:  filepath.Join(tmp, "logs", "mariadb.log"),
		PIDFile:  filepath.Join(tmp, "mariadb.pid"),
		ConfFile: filepath.Join(tmp, "mariadb.conf"),
		Port:     3306,
		User:     "test",
	}
	if err := New().Create(inst); err == nil {
		t.Error("Create() expected error when mariadbd not found in snap")
	}
}

func TestCheckStatus_Stopped(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		PIDFile: filepath.Join(tmp, "mariadb.pid"),
		Port:    59931,
	}
	if status := New().CheckStatus(inst); status != engine.StatusStopped {
		t.Errorf("CheckStatus() = %q, want %q", status, engine.StatusStopped)
	}
}

func TestDelete_NoDataDir(t *testing.T) {
	inst := &engine.Instance{
		DataDir:  "/tmp/klyradb_mariadb_test_nonexistent_98765",
		LogFile:  "/tmp/klyradb_mariadb_test_nonexistent_98765.log",
		PIDFile:  "/tmp/klyradb_mariadb_test_nonexistent_98765.pid",
		ConfFile: "/tmp/klyradb_mariadb_test_nonexistent_98765.conf",
		Port:     59932,
	}
	if err := New().Delete(inst); err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

func TestFindBinary_SnapContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	if got := findBinary("mariadbd"); got != "" {
		t.Errorf("findBinary in empty snap = %q, want empty", got)
	}

	sbinDir := filepath.Join(tmp, "opt/klyra-mariadb/sbin")
	if err := os.MkdirAll(sbinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sbinDir, "mariadbd"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findBinary("mariadbd"); got == "" {
		t.Error("findBinary in snap with binary = empty, want path")
	}
}

func TestWriteMyConf(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		DataDir:  filepath.Join(tmp, "data"),
		LogFile:  filepath.Join(tmp, "mariadb.log"),
		PIDFile:  filepath.Join(tmp, "mariadb.pid"),
		ConfFile: filepath.Join(tmp, "mariadb.conf"),
		Port:     3308,
		User:     "testuser",
	}
	if err := os.MkdirAll(inst.DataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeMyConf(inst); err != nil {
		t.Fatalf("writeMyConf() error: %v", err)
	}
	conf, err := os.ReadFile(inst.ConfFile)
	if err != nil {
		t.Fatalf("conf not written: %v", err)
	}
	content := string(conf)
	for _, want := range []string{"port = 3308", "user = testuser", "bind-address = 127.0.0.1", "[mariadbd]"} {
		if !strings.Contains(content, want) {
			t.Errorf("conf missing %q", want)
		}
	}
}

func TestDetectMariaDBVersion(t *testing.T) {
	tmp := t.TempDir()
	// Mock binary with valid MariaDB version output
	script := "#!/bin/sh\necho '/sbin/mariadbd  Ver 10.11.6-MariaDB Distrib 10.11.6-MariaDB, for debian-linux-gnu (x86_64)'\n"
	bin := filepath.Join(tmp, "mariadbd")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ver := detectMariaDBVersion(bin)
	if ver == "" {
		t.Error("detectMariaDBVersion returned empty for valid MariaDB output")
	}
	if !strings.HasPrefix(ver, "10.11") {
		t.Errorf("detectMariaDBVersion = %q, want 10.11.x", ver)
	}
}

func TestDetectMariaDBVersion_NotMariaDB(t *testing.T) {
	tmp := t.TempDir()
	// MySQL binary — should return empty
	script := "#!/bin/sh\necho '/usr/sbin/mysqld  Ver 8.0.36 Distrib 8.0.36, for Linux (x86_64)'\n"
	bin := filepath.Join(tmp, "mysqld")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ver := detectMariaDBVersion(bin)
	if ver != "" {
		t.Errorf("detectMariaDBVersion for MySQL binary = %q, want empty", ver)
	}
}
