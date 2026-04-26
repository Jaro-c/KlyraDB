package manager

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"klyradb/internal/engine"
	"klyradb/internal/mariadb"
	"klyradb/internal/mysql"
	"klyradb/internal/pg"
	"klyradb/internal/redis"
	"klyradb/internal/store"
)

// defaultPorts are starting port search points per DB type.
var defaultPorts = map[engine.DBType]int{
	engine.TypePostgres: 5432,
	engine.TypeMySQL:    3306,
	engine.TypeMariaDB:  3316, // offset to avoid collision if MySQL also installed
	engine.TypeRedis:    6379,
}

// confExtensions maps DB types to config file extensions.
var confExtensions = map[engine.DBType]string{
	engine.TypeMySQL:   ".cnf",
	engine.TypeMariaDB: ".cnf",
	engine.TypeRedis:   ".conf",
}

type Manager struct {
	mu        sync.RWMutex
	instances map[string]*engine.Instance
	engines   map[engine.DBType]engine.Engine
	store     *store.Store
	baseDir   string
}

func New(s *store.Store) *Manager {
	baseDir := engine.BaseDir()
	for _, sub := range []string{"data", "logs", "pids", "conf", "engines"} {
		_ = os.MkdirAll(filepath.Join(baseDir, sub), 0o755)
	}
	return &Manager{
		instances: map[string]*engine.Instance{},
		engines: map[engine.DBType]engine.Engine{
			engine.TypePostgres: pg.New(),
			engine.TypeMySQL:    mysql.New(),
			engine.TypeMariaDB:  mariadb.New(),
			engine.TypeRedis:    redis.New(),
		},
		store:   s,
		baseDir: baseDir,
	}
}

func (m *Manager) LoadAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, err := m.store.ReadRaw()
	if err != nil {
		return
	}
	var list []engine.Instance
	if err := json.Unmarshal(raw, &list); err != nil {
		return
	}
	for _, r := range list {
		inst := r
		eng, ok := m.engines[inst.Type]
		if !ok {
			inst.Status = engine.StatusStopped
		} else {
			inst.Status = eng.CheckStatus(&inst)
		}
		m.instances[inst.ID] = &inst
	}
}

func (m *Manager) ListVersions() []engine.Version {
	var out []engine.Version
	for _, eng := range m.engines {
		out = append(out, eng.Versions()...)
	}
	return out
}

func (m *Manager) Instances() []engine.Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]engine.Instance, 0, len(m.instances))
	for _, i := range m.instances {
		inst := *i
		inst.UpgradeVersion = m.latestAvailable(inst.Type, inst.Version)
		out = append(out, inst)
	}
	return out
}

// latestAvailable returns the highest installed version newer than currentVersion
// for the given DB type. Only works for integer-major versions (PostgreSQL).
func (m *Manager) latestAvailable(t engine.DBType, currentVersion string) string {
	eng, ok := m.engines[t]
	if !ok {
		return ""
	}
	cur, err := strconv.Atoi(currentVersion)
	if err != nil {
		return ""
	}
	best := 0
	bestStr := ""
	for _, v := range eng.Versions() {
		if !v.Installed {
			continue
		}
		maj, err := strconv.Atoi(v.Major)
		if err != nil {
			continue
		}
		if maj > cur && maj > best {
			best = maj
			bestStr = v.Major
		}
	}
	return bestStr
}

func (m *Manager) Create(name, dbType, version string, port int) (engine.Instance, error) {
	t := engine.DBType(dbType)
	eng, ok := m.engines[t]
	if !ok {
		return engine.Instance{}, fmt.Errorf("unknown database type: %s", dbType)
	}

	if port == 0 {
		port = m.NextFreePort(t)
	}
	if !engine.PortFree(port) {
		return engine.Instance{}, fmt.Errorf("port %d is already in use by another process", port)
	}
	if m.portTaken(port) {
		return engine.Instance{}, fmt.Errorf("port %d is already used by another KlyraDB instance", port)
	}

	id := randID()
	usr := currentUser()
	dataDir := filepath.Join(m.baseDir, "data", id)
	logFile := filepath.Join(m.baseDir, "logs", id+".log")
	pidFile := filepath.Join(m.baseDir, "pids", id+".pid")

	var confFile string
	if ext, ok := confExtensions[t]; ok {
		confFile = filepath.Join(m.baseDir, "conf", id+ext)
	}

	inst := engine.Instance{
		ID:        id,
		Name:      name,
		Type:      t,
		Version:   version,
		Port:      port,
		DataDir:   dataDir,
		LogFile:   logFile,
		PIDFile:   pidFile,
		ConfFile:  confFile,
		User:      usr,
		Status:    engine.StatusInit,
		CreatedAt: time.Now(),
	}

	if err := eng.Create(&inst); err != nil {
		inst.Status = engine.StatusError
		inst.LastError = err.Error()
		return inst, err
	}

	inst.Status = engine.StatusStopped
	m.mu.Lock()
	m.instances[id] = &inst
	m.mu.Unlock()
	m.persist()
	return inst, nil
}

func (m *Manager) Start(id string) error {
	m.mu.Lock()
	inst, ok := m.instances[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}
	eng, ok := m.engines[inst.Type]
	if !ok {
		return fmt.Errorf("no engine for type %s", inst.Type)
	}
	if err := eng.Start(inst); err != nil {
		m.mu.Lock()
		inst.Status = engine.StatusError
		inst.LastError = err.Error()
		m.mu.Unlock()
		m.persist()
		return err
	}
	m.mu.Lock()
	inst.Status = engine.StatusRunning
	inst.LastError = ""
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) Stop(id string) error {
	m.mu.RLock()
	inst, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}
	eng, ok := m.engines[inst.Type]
	if !ok {
		return fmt.Errorf("no engine for type %s", inst.Type)
	}
	if err := eng.Stop(inst); err != nil {
		return err
	}
	m.mu.Lock()
	inst.Status = engine.StatusStopped
	m.mu.Unlock()
	m.persist()
	return nil
}

func (m *Manager) StopAll() {
	m.mu.RLock()
	ids := make([]string, 0)
	for id, i := range m.instances {
		if i.Status == engine.StatusRunning {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	inst, ok := m.instances[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	delete(m.instances, id)
	m.mu.Unlock()

	if eng, ok := m.engines[inst.Type]; ok {
		_ = eng.Delete(inst)
	}
	m.persist()
	return nil
}

func (m *Manager) Status(id string) (engine.Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[id]
	if !ok {
		return "", fmt.Errorf("instance %s not found", id)
	}
	if eng, ok := m.engines[inst.Type]; ok {
		inst.Status = eng.CheckStatus(inst)
	}
	return inst.Status, nil
}

// NextFreePort finds the first free port starting from the type's default,
// skipping ports already used by other instances.
func (m *Manager) NextFreePort(t engine.DBType) int {
	start, ok := defaultPorts[t]
	if !ok {
		start = 15432
	}
	for p := start; p < start+500; p++ {
		if engine.PortFree(p) && !m.portTaken(p) {
			return p
		}
	}
	return start
}

// ---- internals ----

func (m *Manager) portTaken(p int) bool {
	for _, i := range m.instances {
		if i.Port == p {
			return true
		}
	}
	return false
}

func (m *Manager) persist() {
	m.mu.RLock()
	list := make([]engine.Instance, 0, len(m.instances))
	for _, i := range m.instances {
		list = append(list, *i)
	}
	m.mu.RUnlock()
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = m.store.WriteRaw(b)
}

func randID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "klyradb"
}
