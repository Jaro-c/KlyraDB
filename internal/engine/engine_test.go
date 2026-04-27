package engine

import (
	"fmt"
	"net"
	"os"
	"testing"
)

func TestPortFree_bound(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if PortFree(port) {
		t.Errorf("port %d is bound but PortFree returned true", port)
	}
	l.Close()
	if !PortFree(port) {
		t.Errorf("port %d is released but PortFree returned false", port)
	}
}

func TestPortOpen_listening(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	go func() {
		c, _ := l.Accept()
		if c != nil {
			c.Close()
		}
	}()
	if !PortOpen(port) {
		t.Errorf("port %d is listening but PortOpen returned false", port)
	}
	l.Close()
	if PortOpen(port) {
		t.Errorf("port %d is closed but PortOpen returned true", port)
	}
}

func TestCheckPID_alive(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "pid")
	if err != nil {
		t.Fatal(err)
	}
	pid := os.Getpid()
	fmt.Fprintf(f, "%d\n", pid)
	f.Close()

	got, alive := CheckPID(f.Name())
	if !alive {
		t.Error("current process should be alive")
	}
	if got != pid {
		t.Errorf("expected pid %d, got %d", pid, got)
	}
}

func TestCheckPID_stalePID(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "pid")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(f, "999999999\n")
	f.Close()

	_, alive := CheckPID(f.Name())
	if alive {
		t.Error("pid 999999999 should not be alive")
	}
}

func TestCheckPID_missing(t *testing.T) {
	_, alive := CheckPID("/tmp/klyradb_no_such_pid_file.pid")
	if alive {
		t.Error("missing pid file should not be alive")
	}
}

func TestCheckPID_multiline(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "pid")
	if err != nil {
		t.Fatal(err)
	}
	pid := os.Getpid()
	fmt.Fprintf(f, "%d\nextra line\nanother line\n", pid)
	f.Close()

	got, alive := CheckPID(f.Name())
	if !alive {
		t.Error("multiline PID file (PostgreSQL style) should read first line")
	}
	if got != pid {
		t.Errorf("expected pid %d, got %d", pid, got)
	}
}

func TestBaseDir_snapUserCommon(t *testing.T) {
	t.Setenv("SNAP_USER_COMMON", "/tmp/snap-common")
	if got := BaseDir(); got != "/tmp/snap-common" {
		t.Errorf("expected /tmp/snap-common, got %s", got)
	}
}

func TestBaseDir_default(t *testing.T) {
	t.Setenv("SNAP_USER_COMMON", "")
	home, _ := os.UserHomeDir()
	want := home + "/.local/share/klyradb"
	if got := BaseDir(); got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestSnapPath_inside(t *testing.T) {
	t.Setenv("SNAP", "/snap/klyradb/current")
	got := SnapPath("usr/bin/redis-server")
	want := "/snap/klyradb/current/usr/bin/redis-server"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestSnapPath_outside(t *testing.T) {
	t.Setenv("SNAP", "")
	if got := SnapPath("usr/bin/redis-server"); got != "" {
		t.Errorf("expected empty outside snap, got %s", got)
	}
}
