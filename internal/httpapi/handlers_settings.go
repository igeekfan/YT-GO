package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"YT-GO/internal/core"
)

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
		// Validate settings fields.
		if settings.MaxConcurrent < 1 || settings.MaxConcurrent > 10 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("maxConcurrent must be between 1 and 10"))
			return
		}
		requestedOutputDir := settings.OutputDir
		if s.fixedDir != "" {
			requestedOutputDir = s.fixedDir
		}
		outputDir, err := s.downloads.ensureDir(requestedOutputDir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := validateFilenameTemplate(settings.FilenameTemplate); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.urls.validateProxy(r.Context(), settings.Proxy); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(settings.CookiesFrom) != "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("browser cookie import is not available in web mode"))
			return
		}
		cookiesFile, err := validateCookieFile(s.service.GetDataDir(), settings.CookiesFile)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		settings.OutputDir = outputDir
		settings.CookiesFile = cookiesFile
		if err := s.service.SaveSettings(settings); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) validateStoredWebSettings(ctx context.Context) (core.Settings, error) {
	settings := s.service.GetSettings()
	if err := s.urls.validateProxy(ctx, settings.Proxy); err != nil {
		return settings, err
	}
	if strings.TrimSpace(settings.CookiesFrom) != "" {
		return settings, fmt.Errorf("browser cookie import is not available in web mode")
	}
	if _, err := validateCookieFile(s.service.GetDataDir(), settings.CookiesFile); err != nil {
		return settings, err
	}
	return settings, nil
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

// handleBrowseDir returns subdirectories beneath the server's web download root.
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

	dir, parent, dirs, err := s.downloads.readDir(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":    dir,
		"parent":  parent,
		"dirs":    dirs,
		"homeDir": s.downloads.path,
	})
}

// handleCookiesUpload accepts a cookies file upload for web mode.
// The file is saved to the data directory and the path is returned.
func (s *Server) handleCookiesUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	file, _, err := r.FormFile("file")
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
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
	if err := os.MkdirAll(cookiesDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to create cookies directory: %w", err))
		return
	}
	if err := os.Chmod(cookiesDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to secure cookies directory: %w", err))
		return
	}

	// Use a server-generated name so an upload cannot overwrite a chosen file
	// or follow a pre-created symlink. os.CreateTemp creates the file with 0600.
	dst, err := os.CreateTemp(cookiesDir, "cookies-*.txt")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to create file: %w", err))
		return
	}
	if err := dst.Chmod(0o600); err != nil {
		_ = dst.Close()
		_ = os.Remove(dst.Name())
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to secure uploaded file: %w", err))
		return
	}
	destPath := dst.Name()
	safeName := filepath.Base(destPath)
	complete := false
	defer func() {
		_ = dst.Close()
		if !complete {
			_ = os.Remove(destPath)
		}
	}()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to save file: %w", err))
		return
	}
	if err := dst.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to close uploaded file: %w", err))
		return
	}
	complete = true

	writeJSON(w, http.StatusOK, map[string]string{
		"path": destPath,
		"name": safeName,
	})
}
