package mariadb

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"klyradb/internal/engine"
)

// supportedMajors — MariaDB LTS versions with active support as of 2026-04.
// 11.4 LTS until May 2029. 10.11 LTS until Feb 2028. 10.6 LTS until Jul 2026.
var supportedMajors = []string{"11.4", "10.11", "10.6"}

type MariaDBEngine struct{}

func New() *MariaDBEngine { return &MariaDBEngine{} }

func (e *MariaDBEngine) DBType() engine.DBType { return engine.TypeMariaDB }

func (e *MariaDBEngine) Versions() []engine.Version {
	bin := findMariaDBd()
	out := make([]engine.Version, 0, len(supportedMajors))

	installedVer := ""
	installedBin := ""
	if bin != "" {
		ver := detectMariaDBVersion(bin)
		if ver != "" {
			installedVer = ver
			installedBin = filepath.Dir(bin)
		}
	}

	for _, m := range supportedMajors {
		v := engine.Version{Type: engine.TypeMariaDB, Major: m, Label: "MariaDB " + m}
		if installedVer != "" && strings.HasPrefix(installedVer, m) {
			v.Installed = true
			v.BinPath = installedBin
		}
		out = append(out, v)
	}
	return out
}

func (e *MariaDBEngine) Create(inst *engine.Instance) error {
	bin := findMariaDBd()
	if bin == "" {
		if engine.SnapDir() != "" {
			return fmt.Errorf("mariadbd not found — MariaDB is not bundled in this snap")
		}
		return fmt.Errorf("mariadbd not found — install: sudo apt install mariadb-server")
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

	// mariadb-install-db prefers --basedir so it can find system tables
	basedir := filepath.Dir(filepath.Dir(bin)) // /usr/sbin → /usr
	initBin := findBinary("mariadb-install-db")
	if initBin == "" {
		initBin = findBinary("mysql_install_db")
	}
	if initBin == "" {
		return fmt.Errorf("mariadb-install-db not found — reinstall mariadb-server")
	}
	cmd := exec.Command(initBin,
		"--user="+inst.User,
		"--datadir="+inst.DataDir,
		"--basedir="+basedir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mariadb-install-db: %s", out)
	}
	return writeMyConf(inst)
}

func (e *MariaDBEngine) Start(inst *engine.Instance) error {
	safe := findBinary("mariadbd-safe")
	if safe == "" {
		safe = findBinary("mysqld_safe")
	}
	if safe != "" {
		cmd := exec.Command(safe, "--defaults-file="+inst.ConfFile)
		if err := cmd.Start(); err == nil {
			return waitReady(inst)
		}
	}
	// Fallback: direct daemon start
	bin := findMariaDBd()
	if bin == "" {
		return fmt.Errorf("mariadbd not found")
	}
	cmd := exec.Command(bin, "--defaults-file="+inst.ConfFile, "--daemonize")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mariadbd start: %s", out)
	}
	return waitReady(inst)
}

func (e *MariaDBEngine) Stop(inst *engine.Instance) error {
	sockFile := filepath.Join(inst.DataDir, "mariadb.sock")
	// Try mariadb-admin first, then mysqladmin
	for _, admin := range []string{"mariadb-admin", "mysqladmin"} {
		bin := findBinary(admin)
		if bin == "" {
			continue
		}
		cmd := exec.Command(bin, "-u", "root", "--socket="+sockFile, "shutdown")
		if cmd.Run() == nil {
			return nil
		}
	}
	return killFromPIDFile(inst.PIDFile)
}

func (e *MariaDBEngine) Delete(inst *engine.Instance) error {
	_ = e.Stop(inst)
	_ = os.RemoveAll(inst.DataDir)
	_ = os.Remove(inst.LogFile)
	_ = os.Remove(inst.PIDFile)
	_ = os.Remove(inst.ConfFile)
	return nil
}

func (e *MariaDBEngine) CheckStatus(inst *engine.Instance) engine.Status {
	_, pidOK := engine.CheckPID(inst.PIDFile)
	portOK := engine.PortOpen(inst.Port)
	if pidOK || portOK {
		return engine.StatusRunning
	}
	return engine.StatusStopped
}

// ---- helpers ----

// findMariaDBd checks both new (mariadbd) and legacy (mysqld) binary names.
// Validates mysqld is actually MariaDB by checking --version output.
func findMariaDBd() string {
	if p := findBinary("mariadbd"); p != "" {
		return p
	}
	// mysqld might be MariaDB on some systems
	if p := findBinary("mysqld"); p != "" {
		out, err := exec.Command(p, "--version").Output()
		if err == nil && strings.Contains(string(out), "MariaDB") {
			return p
		}
	}
	return ""
}

func findBinary(name string) string {
	if snap := engine.SnapDir(); snap != "" {
		for _, dir := range []string{"usr/sbin", "usr/bin"} {
			if p := filepath.Join(snap, dir, name); fileExists(p) {
				return p
			}
		}
	}
	for _, p := range []string{
		"/usr/sbin/" + name,
		"/usr/bin/" + name,
		"/usr/local/sbin/" + name,
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

func detectMariaDBVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return ""
	}
	s := string(out)
	if !strings.Contains(s, "MariaDB") {
		return ""
	}
	// "mariadbd  Ver 10.11.6-MariaDB ..."
	fields := strings.Fields(s)
	for i, f := range fields {
		if strings.EqualFold(f, "ver") && i+1 < len(fields) {
			ver := fields[i+1]
			// Strip "-MariaDB" suffix → "10.11.6"
			ver = strings.Split(ver, "-")[0]
			return ver
		}
	}
	return ""
}

func writeMyConf(inst *engine.Instance) error {
	sockFile := filepath.Join(inst.DataDir, "mariadb.sock")
	content := fmt.Sprintf(`[mariadbd]
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
	return fmt.Errorf("mariadb did not start on port %d within 45s", inst.Port)
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
