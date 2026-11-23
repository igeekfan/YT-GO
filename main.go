//go:build !web

package main

import (
	"embed"
	"log"

	"YT-GO/desktop"
	"YT-GO/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	platform.EnableUTF8Console()
	app := application.New(application.Options{
		Name: "YT-GO",
		Icon: icon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{ProgramName: "YT-GO"},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "YT-GO",
		Width:            960,
		Height:           700,
		MinWidth:         720,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})
	app.RegisterService(application.NewService(desktop.NewApp(app, window, currentAppVersion())))

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
