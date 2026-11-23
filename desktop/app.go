package desktop

import (
	"context"
	"fmt"
	"time"

	"YT-GO/internal/core"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	app     *application.App
	window  application.Window
	service *core.Service
}

func NewApp(app *application.App, window application.Window, appVersion string) *App {
	return &App{app: app, window: window, service: core.NewService(appVersion)}
}

// ServiceStartup starts the shared service and connects its events to Wails.
func (a *App) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	a.service.SetHooks(core.Hooks{
		AppLog: func(msg string) {
			fmt.Println(msg)
			a.app.Event.Emit("app:log", msg)
		},
		DownloadUpdate: func(task *core.DownloadTask) {
			if task == nil {
				return
			}
			a.app.Event.Emit("download:update", fromCoreDownloadTask(*task))
		},
		DownloadRemove: func(taskID string) {
			a.app.Event.Emit("download:remove", taskID)
		},
		DownloadLog: func(taskID string, line string) {
			fmt.Printf("[%s] %s\n", taskID, line)
			a.app.Event.Emit("download:log", map[string]string{"taskId": taskID, "line": line})
		},
	})
	return a.service.Startup()
}

// ServiceShutdown stops downloads and flushes persisted state before exit.
func (a *App) ServiceShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.service.Shutdown(ctx)
}
