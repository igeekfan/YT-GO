package httpapi

import (
	"net/http"
	"regexp"
	"strings"
)

type URLRequest struct {
	URL string `json:"url"`
}

func extractURLFromText(input string) string {
	trimmed := strings.TrimSpace(input)
	if !strings.ContainsAny(trimmed, " \t\r\n") {
		return trimmed
	}
	m := regexp.MustCompile(`https?://\S+`).FindString(trimmed)
	if m == "" {
		return trimmed
	}
	return strings.TrimRight(m, ".,;:!?)]}，。；：！？、）】》」』")
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
	if _, err := s.validateStoredWebSettings(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	validatedURL, err := s.urls.validate(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := s.service.GetVideoInfo(validatedURL)
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
	if _, err := s.validateStoredWebSettings(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	validatedURL, err := s.urls.validate(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := s.service.GetFormats(validatedURL)
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
	if _, err := s.validateStoredWebSettings(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	validatedURL, err := s.urls.validate(r.Context(), req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := s.service.GetPlaylistInfo(validatedURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}
