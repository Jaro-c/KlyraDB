package mysql

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
	if New().DBType() != engine.TypeMySQL {
		t.Errorf("DBType() = %q, want %q", New().DBType(), engine.TypeMySQL)
	}
}

func TestVersions_ReturnsNonEmpty(t *testing.T) {
	versions := New().Versions()
	if len(versions) == 0 {
		t.Fatal("Versions() returned empty list")
	}
	for _, v := range versions {
		if v.Type != engine.TypeMySQL {
			t.Errorf("version type = %q, want %q", v.Type, engine.TypeMySQL)
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
	sbinDir := filepath.Join(tmp, "opt/klyra-mysql/sbin")
	if err := os.MkdirAll(sbinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho '/usr/sbin/mysqld  Ver 8.0.36 Distrib 8.0.36, for Linux (x86_64)'\n"
	if err := os.WriteFile(filepath.Join(sbinDir, "mysqld"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP", tmp)

	// Verify binary is found in snap context
	bin := findBinary("mysqld")
	if bin == "" {
		t.Fatal("findBinary returned empty with mock binary in snap dir")
	}
	// Verify version detection works and filters out MariaDB
	ver := detectMySQLVersion(bin)
	if ver == "" {
		t.Fatal("detectMySQLVersion returned empty for mock binary")
	}
	if strings.Contains(ver, "MariaDB") {
		t.Error("detectMySQLVersion should not return MariaDB version")
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
		LogFile:  filepath.Join(tmp, "logs", "mysql.log"),
		PIDFile:  filepath.Join(tmp, "mysql.pid"),
		ConfFile: filepath.Join(tmp, "mysql.conf"),
		Port:     3306,
		User:     "test",
	}
	if err := New().Create(inst); err == nil {
		t.Error("Create() expected error when mysqld not found in snap")
	}
}

func TestCheckStatus_Stopped(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		PIDFile: filepath.Join(tmp, "mysql.pid"),
		Port:    59921,
	}
	if status := New().CheckStatus(inst); status != engine.StatusStopped {
		t.Errorf("CheckStatus() = %q, want %q", status, engine.StatusStopped)
	}
}

func TestDelete_NoDataDir(t *testing.T) {
	inst := &engine.Instance{
		DataDir:  "/tmp/klyradb_mysql_test_nonexistent_98765",
		LogFile:  "/tmp/klyradb_mysql_test_nonexistent_98765.log",
		PIDFile:  "/tmp/klyradb_mysql_test_nonexistent_98765.pid",
		ConfFile: "/tmp/klyradb_mysql_test_nonexistent_98765.conf",
		Port:     59922,
	}
	if err := New().Delete(inst); err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

func TestFindBinary_SnapContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	if got := findBinary("mysqld"); got != "" {
		t.Errorf("findBinary in empty snap = %q, want empty", got)
	}

	sbinDir := filepath.Join(tmp, "opt/klyra-mysql/sbin")
	if err := os.MkdirAll(sbinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sbinDir, "mysqld"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findBinary("mysqld"); got == "" {
		t.Error("findBinary in snap with binary = empty, want path")
	}
}

func TestWriteMyConf(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		DataDir:  filepath.Join(tmp, "data"),
		LogFile:  filepath.Join(tmp, "mysql.log"),
		PIDFile:  filepath.Join(tmp, "mysql.pid"),
		ConfFile: filepath.Join(tmp, "mysql.conf"),
		Port:     3307,
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
	for _, want := range []string{"port = 3307", "user = testuser", "bind-address = 127.0.0.1"} {
		if !strings.Contains(content, want) {
			t.Errorf("conf missing %q", want)
		}
	}
}

func TestDetectMySQLVersion_NotMariaDB(t *testing.T) {
	tmp := t.TempDir()
	script := "#!/bin/sh\necho '/usr/sbin/mysqld  Ver 8.0.36 Distrib 8.0.36, for Linux (x86_64)'\n"
	bin := filepath.Join(tmp, "mysqld")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ver := detectMySQLVersion(bin)
	if ver == "" {
		t.Error("detectMySQLVersion returned empty for valid output")
	}
	if strings.Contains(ver, "MariaDB") {
		t.Error("detectMySQLVersion should not return MariaDB version")
	}
}
