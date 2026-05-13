package core

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/lrstanley/go-ytdlp"
	"gorm.io/gorm"
)

type Hooks struct {
	AppLog         func(string)
	DownloadUpdate func(*DownloadTask)
	DownloadRemove func(string)
	DownloadLog    func(string, string)
}

type Service struct {
	i18n        *I18n
	downloads   map[string]*DownloadTask
	cancelFns   map[string]context.CancelFunc
	cmds        map[string]*exec.Cmd // running commands for forceful cancel
	mu          sync.RWMutex
	db          *gorm.DB
	downloadSem chan struct{}
	appVersion  string
	hooks       Hooks
}

func NewService(appVersion string) *Service {
	s := &Service{
		i18n:        NewI18n(),
		downloads:   make(map[string]*DownloadTask),
		cancelFns:   make(map[string]context.CancelFunc),
		cmds:        make(map[string]*exec.Cmd),
		downloadSem: make(chan struct{}, 3),
		appVersion:  appVersion,
	}
	return s
}

func (s *Service) SetHooks(h Hooks) {
	s.hooks = h
}

func (s *Service) Startup() error {
	db, err := openDB()
	if err != nil {
		return err
	}
	s.db = db
	s.cleanupTransientDownloads()
	s.loadFromDB()
	settings := s.GetSettings()
	maxConcurrent := settings.MaxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 3
	}
	if maxConcurrent > 10 {
		maxConcurrent = 10
	}
	s.downloadSem = make(chan struct{}, maxConcurrent)
	return nil
}

func (s *Service) emitLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if s.hooks.AppLog != nil {
		s.hooks.AppLog(msg)
	}
}

func (s *Service) emitDownloadUpdate(task *DownloadTask) {
	if s.hooks.DownloadUpdate != nil {
		s.hooks.DownloadUpdate(task)
	}
}

func (s *Service) emitDownloadRemove(taskID string) {
	if s.hooks.DownloadRemove != nil {
		s.hooks.DownloadRemove(taskID)
	}
}

func (s *Service) emitDownloadLog(taskID string, line string) {
	if s.hooks.DownloadLog != nil {
		s.hooks.DownloadLog(taskID, line)
	}
}

// resolveYtDlp resolves the yt-dlp executable path using go-ytdlp.
// It checks the system PATH first, then falls back to go-ytdlp's cache.
// Does NOT trigger a download — use InstallYtDlp for that.
func (s *Service) resolveYtDlp() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resolved, err := ytdlp.Install(ctx, &ytdlp.InstallOptions{
		DisableDownload:      true,
		DisableSystem:        false,
		AllowVersionMismatch: true,
	})
	if err != nil {
		return ""
	}
	return resolved.Executable
}
