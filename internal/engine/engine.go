package engine

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SnapDir returns the $SNAP directory when running inside a snap, empty otherwise.
func SnapDir() string { return os.Getenv("SNAP") }

// SnapPath prepends $SNAP to the given path when running inside a snap.
// Example: SnapPath("usr/bin/redis-server") → "/snap/klyradb/current/usr/bin/redis-server"
func SnapPath(rel string) string {
	if s := SnapDir(); s != "" {
		return filepath.Join(s, rel)
	}
	return ""
}

// BaseDir returns the data base directory.
// Inside snap: $SNAP_USER_COMMON (~/.snap/klyradb/common) — survives updates, removed with snap remove --purge.
// Outside snap: ~/.local/share/klyradb
func BaseDir() string {
	if d := os.Getenv("SNAP_USER_COMMON"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "klyradb")
}

type DBType string

const (
	TypePostgres DBType = "postgres"
	TypeMySQL    DBType = "mysql"
	TypeMariaDB  DBType = "mariadb"
	TypeRedis    DBType = "redis"
)

type Status string

const (
	StatusStopped Status = "stopped"
	StatusRunning Status = "running"
	StatusError   Status = "error"
	StatusInit    Status = "initializing"
)

type Instance struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      DBType    `json:"type"`
	Version   string    `json:"version"`
	Port      int       `json:"port"`
	DataDir   string    `json:"dataDir"`
	LogFile   string    `json:"logFile"`
	PIDFile   string    `json:"pidFile"`
	ConfFile  string    `json:"confFile,omitempty"`
	User      string    `json:"user"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	LastError      string    `json:"lastError,omitempty"`
	UpgradeVersion string    `json:"upgradeVersion,omitempty"`
}

type Version struct {
	Type      DBType `json:"type"`
	Major     string `json:"major"`
	BinPath   string `json:"binPath"`
	Installed bool   `json:"installed"`
	Label     string `json:"label"`
}

type Engine interface {
	DBType() DBType
	Versions() []Version
	Create(inst *Instance) error
	Start(inst *Instance) error
	Stop(inst *Instance) error
	Delete(inst *Instance) error
	CheckStatus(inst *Instance) Status
}

// CheckPID reads a PID file and verifies the process is alive via /proc.
// Reads only the first line to handle pg's multi-line postmaster.pid.
func CheckPID(pidFile string) (int, bool) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	line := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || pid <= 0 {
		return 0, false
	}
	_, err = os.Stat(fmt.Sprintf("/proc/%d", pid))
	return pid, err == nil
}

// PortOpen verifies a TCP port is accepting connections.
func PortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// PortFree returns true if the port is not bound by any process.
func PortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// IsAlive combines PID and TCP checks for robust status detection.
// Fixes the DBngin problem: stale PID file + closed port = stopped.
func IsAlive(inst *Instance) bool {
	_, pidOK := CheckPID(inst.PIDFile)
	portOK := PortOpen(inst.Port)
	return pidOK || portOK
}
