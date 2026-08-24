package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"YT-GO/internal/core"
)

type Server struct {
	service    *core.Service
	mux        *http.ServeMux
	hub        *EventHub
	downloads  *downloadPolicy
	urls       *urlPolicy
	fixedDir   string
	corsOrigin string // allowed CORS origin, empty means same-origin only
	authToken  string // bearer token for web auth, empty means no auth
}

type serverOptions struct {
	downloadRoot string
	fixedDir     string
	resolver     ipResolver
	corsOrigin   string
	authToken    string
}

func New(service *core.Service) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("core service is required")
	}
	rawFixedDir := os.Getenv("YTGO_DOWNLOAD_DIR")
	fixedDir := strings.TrimSpace(rawFixedDir)
	if rawFixedDir != fixedDir {
		return nil, fmt.Errorf("YTGO_DOWNLOAD_DIR must not have leading or trailing whitespace")
	}
	root := strings.TrimSpace(os.Getenv("YTGO_WEB_DOWNLOAD_ROOT"))
	if root == "" {
		root = fixedDir
	}
	if root == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			root = filepath.Join(home, "Downloads")
		} else {
			dataDir := service.GetDataDir()
			if dataDir == "" {
				return nil, fmt.Errorf("cannot determine a safe default web download root; set YTGO_WEB_DOWNLOAD_ROOT")
			}
			root = filepath.Join(dataDir, "downloads")
		}
	}
	return newServer(service, serverOptions{
		downloadRoot: root,
		fixedDir:     fixedDir,
		corsOrigin:   os.Getenv("YTGO_CORS_ORIGIN"),
		authToken:    os.Getenv("YTGO_AUTH_TOKEN"),
	})
}

func newServer(service *core.Service, options serverOptions) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("core service is required")
	}
	downloads, err := newDownloadPolicy(options.downloadRoot)
	if err != nil {
		return nil, err
	}
	fixedDir := strings.TrimSpace(options.fixedDir)
	if fixedDir != "" {
		if !filepath.IsAbs(fixedDir) {
			_ = downloads.close()
			return nil, fmt.Errorf("YTGO_DOWNLOAD_DIR must be an absolute path when web mode is enabled")
		}
		absoluteFixedDir, err := filepath.Abs(fixedDir)
		if err != nil {
			_ = downloads.close()
			return nil, fmt.Errorf("resolve YTGO_DOWNLOAD_DIR: %w", err)
		}
		resolvedFixedDir, err := downloads.ensureDir(absoluteFixedDir)
		if err != nil {
			_ = downloads.close()
			return nil, fmt.Errorf("YTGO_DOWNLOAD_DIR must remain within the web download root: %w", err)
		}
		if !samePath(filepath.Clean(absoluteFixedDir), resolvedFixedDir) {
			_ = downloads.close()
			return nil, fmt.Errorf("YTGO_DOWNLOAD_DIR must not contain symbolic links in web mode")
		}
		fixedDir = resolvedFixedDir
	}
	service.EnableRestrictedNetworking()
	server := &Server{
		service:    service,
		mux:        http.NewServeMux(),
		hub:        NewEventHub(),
		downloads:  downloads,
		urls:       newURLPolicy(options.resolver),
		fixedDir:   fixedDir,
		corsOrigin: options.corsOrigin,
		authToken:  strings.TrimSpace(options.authToken),
	}
	server.registerRoutes()
	return server, nil
}

func (s *Server) Close() error {
	s.hub.Close()
	return s.downloads.close()
}

func (s *Server) DownloadRoot() string {
	return s.downloads.path
}

func (s *Server) Hub() *EventHub {
	return s.hub
}

func (s *Server) registerRoutes() {
	s.mux.Handle("/api/events", s.hub)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/lang", s.handleLang)
	s.mux.HandleFunc("/api/about", s.handleAbout)
	s.mux.HandleFunc("/api/version", s.handleVersion)
	s.mux.HandleFunc("/api/update", s.handleUpdate)
	s.mux.HandleFunc("/api/ytdlp/status", s.handleYtDlpStatus)
	s.mux.HandleFunc("/api/ytdlp/version-check", s.handleYtDlpVersionCheck)
	s.mux.HandleFunc("/api/ytdlp/update", s.handleYtDlpUpdate)
	s.mux.HandleFunc("/api/ytdlp/install", s.handleYtDlpInstall)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/settings/first-run", s.handleFirstRun)
	s.mux.HandleFunc("/api/settings/needs-cookie", s.handleNeedsCookie)
	s.mux.HandleFunc("/api/settings/reset", s.handleResetSettings)
	s.mux.HandleFunc("/api/settings/browse-dir", s.handleBrowseDir)
	s.mux.HandleFunc("/api/cookies/upload", s.handleCookiesUpload)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/diagnostics", s.handleDiagnostics)
	s.mux.HandleFunc("/api/diagnostics/deps", s.handleDeps)
	s.mux.HandleFunc("/api/diagnostics/deno/update", s.handleDenoUpdate)
	s.mux.HandleFunc("/api/video/info", s.handleVideoInfo)
	s.mux.HandleFunc("/api/video/formats", s.handleFormats)
	s.mux.HandleFunc("/api/video/playlist", s.handlePlaylist)
	s.mux.HandleFunc("/api/downloads", s.handleDownloads)
	s.mux.HandleFunc("/api/downloads/", s.handleDownloadAction)
}

func samePath(left, right string) bool {
	if filepath.Separator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}
