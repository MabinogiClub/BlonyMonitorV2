package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"blonymonitorv2/internal/app"
	"blonymonitorv2/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	appCtx   context.Context
	trayApp  *tray.TrayApp
	isHidden bool
)

func main() {
	application := app.NewApp()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trayApp = tray.NewTrayApp(ctx)

	trayApp.OnDblClick(func() {
		if appCtx == nil {
			return
		}
		if isHidden {
			runtime.WindowShow(appCtx)
			isHidden = false
			trayApp.SetHidden(false)
		}
	})

	trayApp.OnQuit(func() {
		if appCtx != nil {
			runtime.Quit(appCtx)
		}
		trayApp.Quit()
		os.Exit(0)
	})

	trayApp.RunAsync()

	onStartup := func(ctx context.Context) {
		appCtx = ctx
		application.SetOnHide(func() {
			runtime.WindowHide(ctx)
			isHidden = true
			trayApp.SetHidden(true)
		})
		application.Startup(ctx)
	}

	err := wails.Run(&options.App{
		Title:            "BlonyMonitorV2",
		Width:            440,
		Height:           600,
		MinWidth:         440,
		MinHeight:        600,
		DisableResize:    false,
		Frameless:        true,
		AlwaysOnTop:      false,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 200},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  onStartup,
		OnShutdown: application.Shutdown,
		Bind: []interface{}{
			application,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.Dark,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
