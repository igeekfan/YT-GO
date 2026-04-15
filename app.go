package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
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
	ctx         context.Context
	i18n        *I18n
	downloads   map[string]*DownloadTask
	cancelFns   map[string]context.CancelFunc
	mu          sync.RWMutex
	ytdlpPath   string
	db          *gorm.DB
	downloadSem chan struct{} // semaphore for concurrent download limiting
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		downloads:   make(map[string]*DownloadTask),
		cancelFns:   make(map[string]context.CancelFunc),
		i18n:        NewI18n(),
		downloadSem: make(chan struct{}, 3), // default max 3 concurrent downloads
	}
	app.ytdlpPath = app.findYtDlp()
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if db, err := openDB(); err == nil {
		a.db = db
		a.cleanupTransientDownloads()
		a.loadFromDB()
		// Initialize download semaphore based on settings
		settings := a.GetSettings()
		maxConcurrent := settings.MaxConcurrent
		if maxConcurrent < 1 {
			maxConcurrent = 3 // default
		}
		if maxConcurrent > 10 {
			maxConcurrent = 10 // cap at 10
		}
		a.downloadSem = make(chan struct{}, maxConcurrent)
	}
}

// emitLog sends a log message to the frontend via the "app:log" event.
func (a *App) emitLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "app:log", msg)
	}
}
