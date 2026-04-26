package main

import (
	"embed"
	"log"

	"klyradb/internal/i18n"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend
var assets embed.FS

//go:embed all:locales
var localesFS embed.FS

func main() {
	if err := i18n.Load(localesFS, "locales"); err != nil {
		log.Printf("i18n load failed, falling back to keys: %v", err)
	}
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "KlyraDB",
		Width:     1180,
		Height:    740,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 13, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			ProgramName:         "klyradb",
			WindowIsTranslucent: false,
		},
	})

	if err != nil {
		panic(err)
	}
}
