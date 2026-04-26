package main

import (
	"context"
	"fmt"

	"klyradb/internal/engine"
	"klyradb/internal/i18n"
	"klyradb/internal/manager"
	"klyradb/internal/store"
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

// SuggestPort returns a free port for the given DB type.
func (a *App) SuggestPort(dbType string) int {
	return a.manager.NextFreePort(engine.DBType(dbType))
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
