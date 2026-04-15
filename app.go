package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"

	"YT-GO/internal/core"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed wails.json
var wailsJSON string

var WailsInfo struct {
	Info struct {
		ProductVersion string `json:"productVersion"`
	} `json:"info"`
}

func init() {
	json.Unmarshal([]byte(wailsJSON), &WailsInfo)
}

// App struct
type App struct {
	ctx     context.Context
	service *core.Service
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{service: core.NewService(WailsInfo.Info.ProductVersion)}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
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
			hideCmdWindow(cmd)
		},
	})
	_ = a.service.Startup()
}

// emitLog sends a log message to the frontend via the "app:log" event.
func (a *App) emitLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "app:log", msg)
	}
}
