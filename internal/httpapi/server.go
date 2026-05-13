package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"YT-GO/internal/core"
)

type Server struct {
	service *core.Service
	mux     *http.ServeMux
	hub     *EventHub
}

type URLRequest struct {
	URL string `json:"url"`
}

func New(service *core.Service) *Server {
	server := &Server{
		service: service,
		mux:     http.NewServeMux(),
		hub:     NewEventHub(),
	}
	server.registerRoutes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
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

func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"lang": s.service.GetLang()})
	case http.MethodPost:
		var req struct {
			Lang string `json:"lang"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.service.SetLang(req.Lang)
		writeJSON(w, http.StatusOK, map[string]string{"lang": s.service.GetLang()})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.service.GetAboutInfo())
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"version": s.service.GetCurrentVersion()})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	info, err := s.service.CheckForUpdate()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleYtDlpStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.service.CheckYtDlp())
}

func (s *Server) handleYtDlpUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	output, err := s.service.UpdateYtDlp()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "output": output})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (s *Server) handleYtDlpInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	output, err := s.service.InstallYtDlp()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "output": output})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.service.GetSettings())
	case http.MethodPost:
		var settings core.Settings
		if err := decodeJSON(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.service.SaveSettings(settings); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleFirstRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"firstRun": s.service.IsFirstRun()})
}

func (s *Server) handleNeedsCookie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsCookieConfig": s.service.NeedsCookieConfig()})
}

func (s *Server) handleResetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if err := s.service.ResetSettings(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleBrowseDir returns subdirectories of a given path for web mode directory browsing.
// POST body: {"path": "/home/user"} → {"path": "/home/user", "dirs": ["Downloads", "Videos", ...]}
func (s *Server) handleBrowseDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	dir := req.Path
	if dir == "" {
		dir, _ = os.UserHomeDir()
		if dir == "" {
			dir = "/"
		}
	}

	// Clean and validate the path
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		writeJSON(w, http.StatusOK, map[string]any{
			"path": dir,
			"dirs": []string{},
		})
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"path": dir,
			"dirs": []string{},
		})
		return
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			// Skip hidden directories
			if !strings.HasPrefix(name, ".") {
				dirs = append(dirs, name)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":    dir,
		"parent":  filepath.Dir(dir),
		"dirs":    dirs,
		"homeDir": func() string { h, _ := os.UserHomeDir(); return h }(),
	})
}

// handleCookiesUpload accepts a cookies file upload for web mode.
// The file is saved to the data directory and the path is returned.
func (s *Server) handleCookiesUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	// Max 1MB cookies file
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("failed to read uploaded file: %w", err))
		return
	}
	defer file.Close()

	// Save to data directory
	dataDir := s.service.GetDataDir()
	if dataDir == "" {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("data directory not configured"))
		return
	}

	cookiesDir := filepath.Join(dataDir, "cookies")
	if err := os.MkdirAll(cookiesDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to create cookies directory: %w", err))
		return
	}

	// Use original filename but sanitize it
	safeName := filepath.Base(header.Filename)
	safeName = strings.ReplaceAll(safeName, " ", "_")
	destPath := filepath.Join(cookiesDir, safeName)

	dst, err := os.Create(destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to create file: %w", err))
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to save file: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"path": destPath,
		"name": safeName,
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.service.GetWebConfig())
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.service.GetDiagnosticInfo())
}

func (s *Server) handleDeps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.service.GetDepStatus())
}

func (s *Server) handleDenoUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	output, err := s.service.UpdateDeno()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error(), "output": output})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (s *Server) handleVideoInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req URLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := s.service.GetVideoInfo(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleFormats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req URLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := s.service.GetFormats(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req URLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := s.service.GetPlaylistInfo(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.service.GetDownloads())
	case http.MethodPost:
		var req core.DownloadRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		id, err := s.service.StartDownload(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"id": id})
	case http.MethodDelete:
		s.service.ClearCompleted()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodDelete)
	}
}

func (s *Server) handleDownloadAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/downloads/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			writeMethodNotAllowed(w, http.MethodDelete)
			return
		}
		if err := s.service.RemoveDownload(parts[0]); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	taskID := parts[0]
	action := parts[1]
	switch action {
	case "cancel":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if err := s.service.CancelDownload(taskID); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	case "file":
		// Web mode: download the completed file
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.serveDownloadFile(w, r, taskID)
	default:
		http.NotFound(w, r)
	}
}

// serveDownloadFile serves a completed download file for web mode.
func (s *Server) serveDownloadFile(w http.ResponseWriter, r *http.Request, taskID string) {
	task, err := s.service.GetDownload(taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if task.OutputPath == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("file not available"))
		return
	}

	filePath := filepath.Clean(task.OutputPath)
	info, err := os.Stat(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("file not found: %w", err))
		return
	}

	// If it's a directory, try to find the actual media file inside
	if info.IsDir() {
		writeError(w, http.StatusNotFound, fmt.Errorf("path is a directory, not a file"))
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to open file: %w", err))
		return
	}
	defer f.Close()

	fileName := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, fileName, info.ModTime(), f)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeMethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
