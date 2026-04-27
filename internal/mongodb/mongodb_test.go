package mongodb

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
	if New().DBType() != engine.TypeMongoDB {
		t.Errorf("DBType() = %q, want %q", New().DBType(), engine.TypeMongoDB)
	}
}

func TestVersions_ReturnsNonEmpty(t *testing.T) {
	versions := New().Versions()
	if len(versions) == 0 {
		t.Fatal("Versions() returned empty list")
	}
	for _, v := range versions {
		if v.Type != engine.TypeMongoDB {
			t.Errorf("version type = %q, want %q", v.Type, engine.TypeMongoDB)
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
	binDir := filepath.Join(tmp, "opt/klyra-mongodb/bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho 'db version v8.0.5'\necho 'Build Info: ...'\n"
	if err := os.WriteFile(filepath.Join(binDir, "mongod"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP", tmp)

	versions := New().Versions()
	installed := 0
	for _, v := range versions {
		if v.Installed {
			installed++
		}
	}
	if installed == 0 {
		t.Error("expected installed version with mock mongod binary")
	}
}

func TestCreate_SnapBinaryNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	inst := &engine.Instance{
		DataDir:  filepath.Join(tmp, "data"),
		LogFile:  filepath.Join(tmp, "logs", "mongo.log"),
		PIDFile:  filepath.Join(tmp, "mongo.pid"),
		ConfFile: filepath.Join(tmp, "mongo.conf"),
		Port:     27017,
		User:     "test",
	}
	if err := New().Create(inst); err == nil {
		t.Error("Create() expected error when mongod not found in snap")
	}
}

func TestCreate_WritesConf(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "opt/klyra-mongodb/bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "mongod"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNAP", tmp)

	inst := &engine.Instance{
		DataDir:  filepath.Join(tmp, "data"),
		LogFile:  filepath.Join(tmp, "logs", "mongo.log"),
		PIDFile:  filepath.Join(tmp, "mongo.pid"),
		ConfFile: filepath.Join(tmp, "mongo.conf"),
		Port:     27018,
		User:     "test",
	}
	if err := New().Create(inst); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	conf, err := os.ReadFile(inst.ConfFile)
	if err != nil {
		t.Fatalf("conf not written: %v", err)
	}
	content := string(conf)
	for _, want := range []string{"port: 27018", "bindIp: 127.0.0.1", "fork: true"} {
		if !strings.Contains(content, want) {
			t.Errorf("conf missing %q", want)
		}
	}
}

func TestCheckStatus_Stopped(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		PIDFile: filepath.Join(tmp, "mongo.pid"),
		Port:    59941,
	}
	if status := New().CheckStatus(inst); status != engine.StatusStopped {
		t.Errorf("CheckStatus() = %q, want %q", status, engine.StatusStopped)
	}
}

func TestDelete_NoDataDir(t *testing.T) {
	inst := &engine.Instance{
		DataDir:  "/tmp/klyradb_mongo_test_nonexistent_98765",
		LogFile:  "/tmp/klyradb_mongo_test_nonexistent_98765.log",
		PIDFile:  "/tmp/klyradb_mongo_test_nonexistent_98765.pid",
		ConfFile: "/tmp/klyradb_mongo_test_nonexistent_98765.conf",
		Port:     59942,
	}
	if err := New().Delete(inst); err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

func TestFindBinary_SnapContext(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	if got := findBinary("mongod"); got != "" {
		t.Errorf("findBinary in empty snap = %q, want empty", got)
	}

	binDir := filepath.Join(tmp, "opt/klyra-mongodb/bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "mongod"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findBinary("mongod"); got == "" {
		t.Error("findBinary in snap with binary = empty, want path")
	}
}

func TestDetectVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script mock binaries not supported on Windows")
	}
	tmp := t.TempDir()
	script := "#!/bin/sh\necho 'db version v8.0.5'\necho 'Build Info: ...'\n"
	bin := filepath.Join(tmp, "mongod")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ver := detectVersion(bin)
	if ver == "" {
		t.Error("detectVersion returned empty for valid output")
	}
	if !strings.HasPrefix(ver, "8.0") {
		t.Errorf("detectVersion = %q, want 8.0", ver)
	}
}
