package httpapi

import (
	"net/http"
	"strings"

	"YT-GO/internal/core"
)

func (s *Server) handleIncrementalPlaylistScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req URLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.validateStoredWebSettings(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	validatedURL, err := s.urls.validate(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.urls.validate(r.Context(), "https://www.youtube.com"); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.service.ScanYouTubeCollection(validatedURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleIncrementalPlaylistQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req core.IncrementalDownloadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	settings, err := s.validateStoredWebSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.urls.validate(r.Context(), "https://www.youtube.com"); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	filenameTemplate := settings.FilenameTemplate
	if req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "" {
		filenameTemplate = req.Options.FilenameTemplate
	}
	req.OutputDir, err = s.validateDownloadDestination(req.OutputDir, filenameTemplate)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.service.QueueNewYouTubeVideos(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
