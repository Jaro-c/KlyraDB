package pg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"klyradb/internal/engine"
	"klyradb/internal/versions"
)

var pgFallback = []string{"18.3", "17.9", "16.13"}

func pgMajors() []string { return versions.FetchLatest("postgresql", 3, pgFallback) }

type PGEngine struct{}

func New() *PGEngine { return &PGEngine{} }

func (e *PGEngine) DBType() engine.DBType { return engine.TypePostgres }

func (e *PGEngine) Versions() []engine.Version {
	var candidates []string
	if snap := engine.SnapDir(); snap != "" {
		// Snap context: only bundled binaries, no host fallback
		candidates = []string{filepath.Join(snap, "usr/lib/postgresql")}
	} else {
		// Native: KlyraDB engines dir takes priority, then system paths
		candidates = []string{
			filepath.Join(engine.EnginesDir(), "pg"),
			"/usr/lib/postgresql",
			"/opt/postgresql",
		}
	}
	found := map[string]string{}
	for _, base := range candidates {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			bin := filepath.Join(base, ent.Name(), "bin")
			if _, err := os.Stat(filepath.Join(bin, "pg_ctl")); err == nil {
				found[ent.Name()] = bin
			}
		}
	}
	majors := pgMajors()
	out := make([]engine.Version, 0, len(majors))
	for _, m := range majors {
		// m is a patch version like "18.3"; directory is named by major "18"
		dirKey := versions.MajorKey(m)
		v := engine.Version{Type: engine.TypePostgres, Major: dirKey, Label: "PostgreSQL " + m, LatestPatch: m}
		if bin, ok := found[dirKey]; ok {
			v.BinPath = bin
			v.Installed = true
			v.InstalledVersion = detectPGVersion(bin)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Major > out[j].Major })
	return out
}

func (e *PGEngine) Create(inst *engine.Instance) error {
	bin, err := e.binFor(inst.Version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(inst.LogFile), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(inst.DataDir, 0o700); err != nil {
		return err
	}
	cmd := exec.Command(
		filepath.Join(bin, "initdb"),
		"-D", inst.DataDir,
		"-U", inst.User,
		"--auth=trust",
		"--encoding=UTF8",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("initdb: %s", out)
	}
	return nil
}

func (e *PGEngine) Start(inst *engine.Instance) error {
	bin, err := e.binFor(inst.Version)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		filepath.Join(bin, "pg_ctl"),
		"-D", inst.DataDir,
		"-l", inst.LogFile,
		"-o", fmt.Sprintf("-p %d -k %s", inst.Port, inst.DataDir),
		"start",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_ctl start: %s", out)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if engine.PortOpen(inst.Port) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("postgres did not start on port %d within 15s", inst.Port)
}

func (e *PGEngine) Stop(inst *engine.Instance) error {
	bin, err := e.binFor(inst.Version)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		filepath.Join(bin, "pg_ctl"),
		"-D", inst.DataDir,
		"-m", "fast",
		"stop",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_ctl stop: %s", out)
	}
	return nil
}

func (e *PGEngine) Delete(inst *engine.Instance) error {
	_ = e.Stop(inst)
	_ = os.RemoveAll(inst.DataDir)
	_ = os.Remove(inst.LogFile)
	return nil
}

// CheckStatus — pg stores its PID in <DataDir>/postmaster.pid.
// Validates both PID alive in /proc AND TCP port responding.
func (e *PGEngine) CheckStatus(inst *engine.Instance) engine.Status {
	pidFile := filepath.Join(inst.DataDir, "postmaster.pid")
	_, pidOK := engine.CheckPID(pidFile)
	portOK := engine.PortOpen(inst.Port)
	if pidOK || portOK {
		return engine.StatusRunning
	}
	return engine.StatusStopped
}

func detectPGVersion(binDir string) string {
	out, err := exec.Command(filepath.Join(binDir, "postgres"), "--version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) > 0 {
		return fields[len(fields)-1]
	}
	return ""
}

func (e *PGEngine) binFor(version string) (string, error) {
	for _, v := range e.Versions() {
		if v.Major == version && v.Installed {
			return v.BinPath, nil
		}
	}
	if engine.SnapDir() != "" {
		return "", fmt.Errorf("PostgreSQL %s not found in snap bundle", version)
	}
	return "", fmt.Errorf("PostgreSQL %s not installed — run: sudo apt install postgresql-%s", version, version)
}
