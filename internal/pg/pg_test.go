package pg

import (
	"os"
	"path/filepath"
	"testing"

	"klyradb/internal/engine"
)

func TestNew(t *testing.T) {
	if New() == nil {
		t.Error("New() returned nil")
	}
}

func TestDBType(t *testing.T) {
	if New().DBType() != engine.TypePostgres {
		t.Errorf("DBType() = %q, want %q", New().DBType(), engine.TypePostgres)
	}
}

func TestVersions_ReturnsNonEmpty(t *testing.T) {
	e := New()
	versions := e.Versions()
	if len(versions) == 0 {
		t.Fatal("Versions() returned empty list")
	}
	for _, v := range versions {
		if v.Type != engine.TypePostgres {
			t.Errorf("version type = %q, want %q", v.Type, engine.TypePostgres)
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

	versions := New().Versions()
	for _, v := range versions {
		if v.Installed {
			t.Errorf("version %s: Installed=true in empty snap dir", v.Major)
		}
	}
}

func TestVersions_SnapWithMockBinaries(t *testing.T) {
	tmp := t.TempDir()
	for _, major := range []string{"16", "17"} {
		binDir := filepath.Join(tmp, "usr/lib/postgresql", major, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(binDir, "pg_ctl"), []byte("#!/bin/sh"), 0o755); err != nil {
			t.Fatal(err)
		}
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
		t.Error("expected installed versions with mock pg_ctl binaries")
	}
}

func TestCreate_SnapBinaryNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SNAP", tmp)

	inst := &engine.Instance{
		DataDir: filepath.Join(tmp, "data"),
		LogFile: filepath.Join(tmp, "logs", "pg.log"),
		Version: "17",
		User:    "test",
		Port:    5432,
	}
	if err := New().Create(inst); err == nil {
		t.Error("Create() expected error when binary not found in snap")
	}
}

func TestCheckStatus_Stopped_NoPIDNoPort(t *testing.T) {
	tmp := t.TempDir()
	inst := &engine.Instance{
		DataDir: filepath.Join(tmp, "data"),
		PIDFile: filepath.Join(tmp, "pg.pid"),
		Port:    59901,
	}
	if status := New().CheckStatus(inst); status != engine.StatusStopped {
		t.Errorf("CheckStatus() = %q, want %q", status, engine.StatusStopped)
	}
}

func TestCheckStatus_StoppedStalePID(t *testing.T) {
	tmp := t.TempDir()
	// Write a PID that doesn't exist in /proc
	pidFile := filepath.Join(tmp, "data", "postmaster.pid")
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidFile, []byte("9999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst := &engine.Instance{
		DataDir: filepath.Join(tmp, "data"),
		Port:    59902,
	}
	if status := New().CheckStatus(inst); status != engine.StatusStopped {
		t.Errorf("CheckStatus() with stale PID = %q, want %q", status, engine.StatusStopped)
	}
}

func TestDelete_NoDataDir(t *testing.T) {
	inst := &engine.Instance{
		DataDir: "/tmp/klyradb_pg_test_nonexistent_98765",
		LogFile: "/tmp/klyradb_pg_test_nonexistent_98765.log",
		PIDFile: "/tmp/klyradb_pg_test_nonexistent_98765.pid",
		Port:    59903,
	}
	if err := New().Delete(inst); err != nil {
		t.Errorf("Delete() unexpected error: %v", err)
	}
}

func TestVersions_SortedDescending(t *testing.T) {
	tmp := t.TempDir()
	for _, major := range []string{"16", "17", "18"} {
		binDir := filepath.Join(tmp, "usr/lib/postgresql", major, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(binDir, "pg_ctl"), []byte("#!/bin/sh"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SNAP", tmp)

	versions := New().Versions()
	for i := 1; i < len(versions); i++ {
		if versions[i-1].Major < versions[i].Major {
			t.Errorf("Versions() not sorted desc: %s before %s", versions[i-1].Major, versions[i].Major)
		}
	}
}
