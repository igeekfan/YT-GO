package httpapi

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"YT-GO/internal/core"
)

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
		settings, err := s.validateStoredWebSettings(r.Context())
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		validatedURL, err := s.urls.validate(r.Context(), req.URL)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		requestedOutputDir := req.OutputDir
		if s.fixedDir != "" {
			requestedOutputDir = s.fixedDir
		}
		outputDir, err := s.downloads.ensureDir(requestedOutputDir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		effectiveTemplate := settings.FilenameTemplate
		if req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "" {
			effectiveTemplate = req.Options.FilenameTemplate
		}
		if err := validateFilenameTemplate(effectiveTemplate); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		req.URL = validatedURL
		req.OutputDir = outputDir
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
// Validates that the file resides within the configured download directory
// to prevent path traversal attacks.
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
	if task.Status != "completed" {
		writeError(w, http.StatusConflict, fmt.Errorf("download is not completed"))
		return
	}

	f, info, err := s.downloads.openFile(task.OutputPath)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	defer f.Close()

	fileName := filepath.Base(task.OutputPath)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fileName}))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, fileName, info.ModTime(), f)
}
