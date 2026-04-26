package mysql

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"klyradb/internal/engine"
)

// supportedMajors — MySQL versions with active support as of 2026-04.
// 8.4 is LTS (active until 2032). 8.0 EOL late 2026, included for compatibility.
var supportedMajors = []string{"8.4", "8.0"}

type MySQLEngine struct{}

func New() *MySQLEngine { return &MySQLEngine{} }

func (e *MySQLEngine) DBType() engine.DBType { return engine.TypeMySQL }

func (e *MySQLEngine) Versions() []engine.Version {
	mysqld := findBinary("mysqld")
	out := make([]engine.Version, 0, len(supportedMajors))

	installedVer := ""
	installedBin := ""
	if mysqld != "" {
		ver := detectMySQLVersion(mysqld)
		// Exclude MariaDB — it also uses mysqld binary on some systems
		if ver != "" && !strings.Contains(ver, "MariaDB") {
			installedVer = ver
			installedBin = filepath.Dir(mysqld)
		}
	}

	for _, m := range supportedMajors {
		v := engine.Version{Type: engine.TypeMySQL, Major: m, Label: "MySQL " + m}
		if installedVer != "" && strings.HasPrefix(installedVer, m) {
			v.Installed = true
			v.BinPath = installedBin
		}
		out = append(out, v)
	}
	return out
}

func (e *MySQLEngine) Create(inst *engine.Instance) error {
	mysqld := findBinary("mysqld")
	if mysqld == "" {
		return fmt.Errorf("mysqld not found — install: sudo apt install mysql-server")
	}
	for _, dir := range []string{inst.DataDir, filepath.Dir(inst.LogFile), filepath.Dir(inst.PIDFile), filepath.Dir(inst.ConfFile)} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
	}
	if err := os.Chmod(inst.DataDir, 0o700); err != nil {
		return err
	}
	cmd := exec.Command(mysqld,
		"--initialize-insecure",
		"--user="+inst.User,
		"--datadir="+inst.DataDir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mysql initialize: %s", out)
	}
	return writeMyConf(inst)
}

func (e *MySQLEngine) Start(inst *engine.Instance) error {
	// Try mysqld_safe first (recommended approach)
	safe := findBinary("mysqld_safe")
	if safe != "" {
		cmd := exec.Command(safe, "--defaults-file="+inst.ConfFile)
		if err := cmd.Start(); err == nil {
			return waitReady(inst)
		}
	}
	// Fallback: start mysqld with --daemonize
	mysqld := findBinary("mysqld")
	if mysqld == "" {
		return fmt.Errorf("mysqld not found")
	}
	cmd := exec.Command(mysqld, "--defaults-file="+inst.ConfFile, "--daemonize")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mysqld start: %s", out)
	}
	return waitReady(inst)
}

func (e *MySQLEngine) Stop(inst *engine.Instance) error {
	admin := findBinary("mysqladmin")
	if admin != "" {
		sockFile := filepath.Join(inst.DataDir, "mysql.sock")
		cmd := exec.Command(admin, "-u", "root", "--socket="+sockFile, "shutdown")
		if cmd.Run() == nil {
			return nil
		}
	}
	// Fallback: SIGTERM to PID
	return killFromPIDFile(inst.PIDFile)
}

func (e *MySQLEngine) Delete(inst *engine.Instance) error {
	_ = e.Stop(inst)
	_ = os.RemoveAll(inst.DataDir)
	_ = os.Remove(inst.LogFile)
	_ = os.Remove(inst.PIDFile)
	_ = os.Remove(inst.ConfFile)
	return nil
}

func (e *MySQLEngine) CheckStatus(inst *engine.Instance) engine.Status {
	_, pidOK := engine.CheckPID(inst.PIDFile)
	portOK := engine.PortOpen(inst.Port)
	if pidOK || portOK {
		return engine.StatusRunning
	}
	return engine.StatusStopped
}

// ---- helpers ----

func findBinary(name string) string {
	paths := []string{
		"/usr/sbin/" + name,
		"/usr/bin/" + name,
		"/usr/local/sbin/" + name,
		"/usr/local/bin/" + name,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func detectMySQLVersion(mysqldPath string) string {
	out, err := exec.Command(mysqldPath, "--version").Output()
	if err != nil {
		return ""
	}
	s := string(out)
	fields := strings.Fields(s)
	for i, f := range fields {
		if strings.EqualFold(f, "ver") && i+1 < len(fields) {
			return strings.Split(fields[i+1], "-")[0]
		}
	}
	return ""
}

func writeMyConf(inst *engine.Instance) error {
	sockFile := filepath.Join(inst.DataDir, "mysql.sock")
	content := fmt.Sprintf(`[mysqld]
datadir = %s
socket = %s
port = %d
pid-file = %s
log-error = %s
user = %s
bind-address = 127.0.0.1
`, inst.DataDir, sockFile, inst.Port, inst.PIDFile, inst.LogFile, inst.User)
	return os.WriteFile(inst.ConfFile, []byte(content), 0o644)
}

func waitReady(inst *engine.Instance) error {
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if engine.PortOpen(inst.Port) {
			return nil
		}
	}
	return fmt.Errorf("mysql did not start on port %d within 45s", inst.Port)
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
