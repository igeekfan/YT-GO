package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	i18n            *I18n
	downloads       map[string]*DownloadTask
	activeDownloads map[string]*downloadControl
	mu              sync.RWMutex
	hookMu          sync.RWMutex
	wechatCacheMu   sync.Mutex
	wechatCache     map[string]wechatChannelsCacheEntry
	dbMu            sync.RWMutex
	db              *gorm.DB
	appVersion      string
	hooks           Hooks
	downloadDir     string // from YTGO_DOWNLOAD_DIR env
	externalURL     string // from YTGO_EXTERNAL_URL env (for web mode download links)
	ytdlpPath       string // from YTGO_YTDLP_PATH env (explicit yt-dlp path)
	outboundPolicy  outboundNetworkPolicy
	restrictedNet   atomic.Bool

	lifecycleMu    sync.Mutex
	accepting      bool
	runtimeStarted bool
	jobs           chan downloadJob
	workerWG       sync.WaitGroup
	limiter        *downloadLimiter
	shutdownOnce   sync.Once
	shutdownDone   chan struct{}
	shutdownErrMu  sync.Mutex
	shutdownErr    error

	persistCh       chan persistenceOperation
	persistWG       sync.WaitGroup
	persistSubmitMu sync.Mutex
	persistClosed   bool
	persistErrMu    sync.Mutex
	persistErr      error
}

func NewService(appVersion string) *Service {
	s := &Service{
		i18n:            NewI18n(),
		downloads:       make(map[string]*DownloadTask),
		activeDownloads: make(map[string]*downloadControl),
		wechatCache:     make(map[string]wechatChannelsCacheEntry),
		appVersion:      appVersion,
		downloadDir:     os.Getenv("YTGO_DOWNLOAD_DIR"),
		externalURL:     os.Getenv("YTGO_EXTERNAL_URL"),
		ytdlpPath:       os.Getenv("YTGO_YTDLP_PATH"),
		outboundPolicy:  newOutboundNetworkPolicy(),
		accepting:       true,
		limiter:         newDownloadLimiter(defaultMaxConcurrentDownloads),
		shutdownDone:    make(chan struct{}),
	}
	return s
}

func (s *Service) SetHooks(h Hooks) {
	s.hookMu.Lock()
	s.hooks = h
	s.hookMu.Unlock()
}

func (s *Service) Startup() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.accepting {
		return ErrServiceClosed
	}
	if s.database() != nil {
		return nil
	}
	db, err := openDB()
	if err != nil {
		return err
	}
	s.dbMu.Lock()
	s.db = db
	s.dbMu.Unlock()
	if err := s.cleanupTransientDownloads(); err != nil {
		s.discardDatabase()
		return err
	}
	if err := s.loadFromDB(); err != nil {
		s.discardDatabase()
		return err
	}
	settings := s.GetSettings()
	// Initialize i18n language from saved settings
	if settings.Language != "" {
		s.i18n.SetLang(Lang(settings.Language))
	}
	maxConcurrent := settings.MaxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 3
	}
	if maxConcurrent > 10 {
		maxConcurrent = 10
	}
	s.limiter.SetLimit(maxConcurrent)
	return nil
}

func (s *Service) discardDatabase() {
	_ = s.closeDatabase()
	s.dbMu.Lock()
	s.db = nil
	s.dbMu.Unlock()
}

func (s *Service) database() *gorm.DB {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	return s.db
}

func (s *Service) emitLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.hookMu.RLock()
	hook := s.hooks.AppLog
	s.hookMu.RUnlock()
	if hook != nil {
		hook(msg)
	}
}

func (s *Service) emitDownloadUpdate(task *DownloadTask) {
	if task == nil {
		return
	}
	snapshot := cloneDownloadTask(task)
	s.hookMu.RLock()
	hook := s.hooks.DownloadUpdate
	s.hookMu.RUnlock()
	if hook != nil {
		hook(snapshot)
	}
}

func (s *Service) emitDownloadRemove(taskID string) {
	s.hookMu.RLock()
	hook := s.hooks.DownloadRemove
	s.hookMu.RUnlock()
	if hook != nil {
		hook(taskID)
	}
}

func (s *Service) emitDownloadLog(taskID string, line string) {
	s.hookMu.RLock()
	downloadHook := s.hooks.DownloadLog
	appHook := s.hooks.AppLog
	s.hookMu.RUnlock()
	if downloadHook != nil {
		downloadHook(taskID, line)
	}
	if appHook != nil {
		appHook(fmt.Sprintf("[%s] %s", taskID, line))
	}
}

// resolveYtDlp resolves the yt-dlp executable path.
// Priority: YTGO_YTDLP_PATH env > go-ytdlp library > exec.LookPath fallback.
// Does NOT trigger a download — use InstallYtDlp for that.
func (s *Service) resolveYtDlp() string {
	if s.ytdlpPath != "" {
		if info, err := os.Stat(s.ytdlpPath); err == nil && !info.IsDir() {
			return s.ytdlpPath
		}
		s.emitLog("YTGO_YTDLP_PATH is set but file not found: %s", s.ytdlpPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resolved, err := ytdlp.Install(ctx, &ytdlp.InstallOptions{
		DisableDownload:      true,
		DisableSystem:        false,
		AllowVersionMismatch: true,
	})
	if err == nil && resolved.Executable != "" {
		return resolved.Executable
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}
	return ""
}

// GetDataDir returns the application data directory path.
func (s *Service) GetDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "YT-GO")
}

// GetDownload returns a single download task by ID.
func (s *Service) GetDownload(id string) (*DownloadTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.downloads[id]
	if !ok {
		return nil, fmt.Errorf("download task not found: %s", id)
	}
	return cloneDownloadTask(task), nil
}

// GetExternalURL returns the configured external URL for download links (web mode).
func (s *Service) GetExternalURL() string {
	return s.externalURL
}

// WebConfig returns web-mode specific configuration for the frontend.
type WebConfig struct {
	DownloadDir string `json:"downloadDir"`
	ExternalURL string `json:"externalURL"`
	HasFixedDir bool   `json:"hasFixedDir"`
}

// GetWebConfig returns web-mode configuration.
func (s *Service) GetWebConfig() WebConfig {
	return WebConfig{
		DownloadDir: s.downloadDir,
		ExternalURL: s.externalURL,
		HasFixedDir: s.downloadDir != "",
	}
}
