package main

import (
	"context"
	"fmt"

	"klyradb/internal/engine"
	"klyradb/internal/i18n"
	"klyradb/internal/manager"
	"klyradb/internal/store"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	manager *manager.Manager
	store   *store.Store
	lang    *i18n.Lang
}

func NewApp() *App {
	s, err := store.New()
	if err != nil {
		panic(err)
	}
	return &App{
		store:   s,
		manager: manager.New(s),
		lang:    i18n.Detect(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.LoadAll()
}

func (a *App) shutdown(ctx context.Context) {
	a.manager.StopAll()
}

// ---- Wails bindings (exposed to frontend) ----

func (a *App) ListVersions() []engine.Version {
	return a.manager.ListVersions()
}

func (a *App) ListInstances() []engine.Instance {
	return a.manager.Instances()
}

func (a *App) CreateInstance(name, dbType, version string, port int) (engine.Instance, error) {
	if name == "" {
		return engine.Instance{}, fmt.Errorf("%s", a.lang.T("error.name_required"))
	}
	return a.manager.Create(name, dbType, version, port)
}

func (a *App) StartInstance(id string) error {
	return a.manager.Start(id)
}

func (a *App) StopInstance(id string) error {
	return a.manager.Stop(id)
}

func (a *App) DeleteInstance(id string) error {
	return a.manager.Delete(id)
}

func (a *App) InstanceStatus(id string) (engine.Status, error) {
	return a.manager.Status(id)
}

// InstallBinary installs the DB binary for the given instance, streaming
// progress lines as "install:progress:<id>" events to the frontend.
func (a *App) InstallBinary(id string) error {
	return a.manager.Install(id, func(line string) {
		wailsRuntime.EventsEmit(a.ctx, "install:progress:"+id, line)
	})
}

// UpgradePatch stops all running instances of the same type+major, upgrades
// the binary, then restarts them. Streams progress as "install:progress:<id>" events.
func (a *App) UpgradePatch(id string) error {
	return a.manager.UpgradePatch(id, func(line string) {
		wailsRuntime.EventsEmit(a.ctx, "install:progress:"+id, line)
	})
}

// SuggestPort returns a free port for the given DB type.
func (a *App) SuggestPort(dbType string) int {
	return a.manager.NextFreePort(engine.DBType(dbType))
}

// SetLocale changes the active language at runtime and returns new strings.
func (a *App) SetLocale(code string) map[string]string {
	a.lang = i18n.For(code)
	return a.lang.All()
}

// ---- i18n bindings ----

func (a *App) Strings() map[string]string {
	return a.lang.All()
}

func (a *App) Locale() string {
	return a.lang.Code
}

func (a *App) Direction() string {
	return a.lang.Dir
}

func (a *App) AvailableLocales() []string {
	return i18n.Available()
}
