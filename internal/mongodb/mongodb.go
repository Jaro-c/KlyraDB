package mongodb

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"klyradb/internal/engine"
	"klyradb/internal/versions"
)

var mongoFallback = []string{"8.2.6", "7.3.4", "6.3.2"}

func mongoMajors() []string { return versions.FetchLatest("mongodb", 3, mongoFallback) }

type MongoEngine struct{}

func New() *MongoEngine { return &MongoEngine{} }

func (e *MongoEngine) DBType() engine.DBType { return engine.TypeMongoDB }

func (e *MongoEngine) Versions() []engine.Version {
	bin := findBinary("mongod")
	majors := mongoMajors()
	out := make([]engine.Version, 0, len(majors))

	installedVer := ""
	installedBin := ""
	if bin != "" {
		installedVer = detectVersion(bin)
		installedBin = filepath.Dir(bin)
	}

	for _, m := range majors {
		v := engine.Version{Type: engine.TypeMongoDB, Major: m, Label: "MongoDB " + m, LatestPatch: m}
		if versions.MajorMatch(installedVer, m) {
			v.Installed = true
			v.BinPath = installedBin
			v.InstalledVersion = installedVer
		}
		out = append(out, v)
	}
	return out
}

func (e *MongoEngine) Create(inst *engine.Instance) error {
	if findBinary("mongod") == "" {
		if engine.SnapDir() != "" {
			return fmt.Errorf("mongod not found in snap bundle")
		}
		return fmt.Errorf("mongod not found — install: sudo apt install mongodb-org")
	}
	for _, dir := range []string{inst.DataDir, filepath.Dir(inst.LogFile), filepath.Dir(inst.PIDFile), filepath.Dir(inst.ConfFile)} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
		}
	}
	if err := os.Chmod(inst.DataDir, 0o700); err != nil { //nolint:gosec
		return err
	}
	return writeConf(inst)
}

func (e *MongoEngine) Start(inst *engine.Instance) error {
	bin := findBinary("mongod")
	if bin == "" {
		return fmt.Errorf("mongod not found")
	}
	cmd := exec.Command(bin, "--config", inst.ConfFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mongod start: %s", out)
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		if engine.PortOpen(inst.Port) {
			return nil
		}
	}
	return fmt.Errorf("mongod did not start on port %d within 30s", inst.Port)
}

func (e *MongoEngine) Stop(inst *engine.Instance) error {
	// Try mongosh first, fall back to legacy mongo shell
	for _, shellName := range []string{"mongosh", "mongo"} {
		shell := findBinary(shellName)
		if shell == "" {
			continue
		}
		cmd := exec.Command(shell,
			"--port", fmt.Sprintf("%d", inst.Port),
			"--eval", "db.adminCommand({shutdown:1})",
			"admin",
		)
		if cmd.Run() == nil {
			return nil
		}
	}
	return killFromPIDFile(inst.PIDFile)
}

func (e *MongoEngine) Delete(inst *engine.Instance) error {
	_ = e.Stop(inst)
	_ = os.RemoveAll(inst.DataDir)
	_ = os.Remove(inst.LogFile)
	_ = os.Remove(inst.PIDFile)
	_ = os.Remove(inst.ConfFile)
	return nil
}

func (e *MongoEngine) CheckStatus(inst *engine.Instance) engine.Status {
	_, pidOK := engine.CheckPID(inst.PIDFile)
	portOK := engine.PortOpen(inst.Port)
	if pidOK || portOK {
		return engine.StatusRunning
	}
	return engine.StatusStopped
}

// ---- helpers ----

func findBinary(name string) string {
	if snap := engine.SnapDir(); snap != "" {
		for _, dir := range []string{"opt/klyra-mongodb/bin", "usr/bin"} {
			if p := filepath.Join(snap, dir, name); fileExists(p) {
				return p
			}
		}
		return ""
	}
	engDir := engine.EnginesDir()
	for _, p := range []string{
		filepath.Join(engDir, "mongodb", "bin", name),
		"/usr/bin/" + name,
		"/usr/local/bin/" + name,
	} {
		if fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func detectVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return ""
	}
	// "db version v8.0.5" on first line
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "db version v") {
			return strings.TrimPrefix(line, "db version v")
		}
	}
	return ""
}

func writeConf(inst *engine.Instance) error {
	content := fmt.Sprintf(`storage:
  dbPath: %s
systemLog:
  destination: file
  path: %s
  logAppend: true
net:
  port: %d
  bindIp: 127.0.0.1
processManagement:
  fork: true
  pidFilePath: %s
`, inst.DataDir, inst.LogFile, inst.Port, inst.PIDFile)
	return os.WriteFile(inst.ConfFile, []byte(content), 0o600)
}

func killFromPIDFile(pidFile string) error {
	pid, ok := engine.CheckPID(pidFile)
	if !ok {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	return proc.Signal(os.Interrupt)
}
