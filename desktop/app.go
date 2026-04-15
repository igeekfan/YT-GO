package desktop

import (
	"context"
	"fmt"
	"os/exec"

	"YT-GO/internal/core"
	"YT-GO/internal/platform"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	service *core.Service
}

func NewApp(appVersion string) *App {
	return &App{service: core.NewService(appVersion)}
}

func OnStartup(app *App) func(context.Context) {
	return app.startup
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.service.SetHooks(core.Hooks{
		AppLog: func(msg string) {
			fmt.Println(msg)
			wailsRuntime.EventsEmit(a.ctx, "app:log", msg)
		},
		DownloadUpdate: func(task *core.DownloadTask) {
			if task == nil {
				return
			}
			copy := fromCoreDownloadTask(*task)
			wailsRuntime.EventsEmit(a.ctx, "download:update", &copy)
		},
		DownloadRemove: func(taskID string) {
			wailsRuntime.EventsEmit(a.ctx, "download:remove", taskID)
		},
		DownloadLog: func(taskID string, line string) {
			wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{"taskId": taskID, "line": line})
		},
		HideCommand: func(cmd *exec.Cmd) {
			platform.HideCmdWindow(cmd)
		},
	})
	_ = a.service.Startup()
}

func (a *App) emitLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "app:log", msg)
	}
}