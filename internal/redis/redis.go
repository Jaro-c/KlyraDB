package redis

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

var redisFallback = []string{"8.0", "7.4", "7.2"}

func redisMajors() []string { return versions.FetchLatest("redis", 3, redisFallback) }

type RedisEngine struct{}

func New() *RedisEngine { return &RedisEngine{} }

func (e *RedisEngine) DBType() engine.DBType { return engine.TypeRedis }

func (e *RedisEngine) Versions() []engine.Version {
	bin := findBinary("redis-server")
	majors := redisMajors()
	out := make([]engine.Version, 0, len(majors))

	installedVer := ""
	installedBin := ""
	if bin != "" {
		installedVer = detectRedisVersion(bin)
		installedBin = filepath.Dir(bin)
	}

	for _, m := range majors {
		v := engine.Version{Type: engine.TypeRedis, Major: m, Label: "Redis " + m}
		if installedVer != "" && strings.HasPrefix(installedVer, m) {
			v.Installed = true
			v.BinPath = installedBin
		}
		out = append(out, v)
	}
	return out
}

func (e *RedisEngine) Create(inst *engine.Instance) error {
	if findBinary("redis-server") == "" {
		if engine.SnapDir() != "" {
			return fmt.Errorf("redis-server not found in snap bundle")
		}
		return fmt.Errorf("redis-server not found — install: sudo apt install redis-server")
	}
	for _, dir := range []string{inst.DataDir, filepath.Dir(inst.LogFile), filepath.Dir(inst.PIDFile), filepath.Dir(inst.ConfFile)} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
	}
	return writeRedisConf(inst)
}

func (e *RedisEngine) Start(inst *engine.Instance) error {
	bin := findBinary("redis-server")
	if bin == "" {
		return fmt.Errorf("redis-server not found")
	}
	cmd := exec.Command(bin, inst.ConfFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("redis-server start: %s", out)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if engine.PortOpen(inst.Port) {
			return nil
		}
	}
	return fmt.Errorf("redis did not start on port %d within 15s", inst.Port)
}

func (e *RedisEngine) Stop(inst *engine.Instance) error {
	cli := findBinary("redis-cli")
	if cli != "" {
		cmd := exec.Command(cli, "-p", fmt.Sprintf("%d", inst.Port), "shutdown", "nosave")
		if cmd.Run() == nil {
			return nil
		}
	}
	return killFromPIDFile(inst.PIDFile)
}

func (e *RedisEngine) Delete(inst *engine.Instance) error {
	_ = e.Stop(inst)
	_ = os.RemoveAll(inst.DataDir)
	_ = os.Remove(inst.LogFile)
	_ = os.Remove(inst.PIDFile)
	_ = os.Remove(inst.ConfFile)
	return nil
}

func (e *RedisEngine) CheckStatus(inst *engine.Instance) engine.Status {
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
		// Snap context: only bundled binaries, never fall back to host system
		for _, dir := range []string{"usr/bin", "usr/local/bin"} {
			if p := filepath.Join(snap, dir, name); fileExists(p) {
				return p
			}
		}
		return ""
	}
	// Native: KlyraDB engines dir takes priority over system paths
	engDir := engine.EnginesDir()
	for _, p := range []string{
		filepath.Join(engDir, "redis", "bin", name),
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

func detectRedisVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return ""
	}
	// "Redis server v=7.4.0 sha=..."
	for _, field := range strings.Fields(string(out)) {
		if strings.HasPrefix(field, "v=") {
			return strings.TrimPrefix(field, "v=")
		}
	}
	return ""
}

func writeRedisConf(inst *engine.Instance) error {
	content := fmt.Sprintf(`port %d
bind 127.0.0.1
daemonize yes
pidfile %s
logfile %s
dir %s
save ""
`, inst.Port, inst.PIDFile, inst.LogFile, inst.DataDir)
	return os.WriteFile(inst.ConfFile, []byte(content), 0o644)
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
