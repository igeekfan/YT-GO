package httpapi

import "net/http"

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

func (s *Server) handleYtDlpVersionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	result, err := s.service.CheckYtDlpVersion()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	cfg := s.service.GetWebConfig()
	response := map[string]any{
		"downloadDir":  "",
		"externalURL":  "",
		"hasFixedDir":  false,
		"authRequired": s.authToken != "",
	}
	// The route stays public so the login bootstrap can discover whether auth
	// is enabled, but filesystem and deployment details require a valid bearer
	// token when authentication is configured.
	if s.authToken == "" || s.checkAuth(r) {
		downloadDir := s.downloads.path
		if s.fixedDir != "" {
			downloadDir = s.fixedDir
		}
		response["downloadDir"] = downloadDir
		response["externalURL"] = cfg.ExternalURL
		response["hasFixedDir"] = s.fixedDir != ""
	}
	writeJSON(w, http.StatusOK, response)
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
