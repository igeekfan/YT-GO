package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers for all responses.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Enforce the origin policy before auth and handlers so cross-origin
		// simple requests cannot trigger mutations in unauthenticated loopback
		// deployments. Requests without Origin (for example CLI clients) pass.
		if s.handleOrigin(w, r) {
			return
		}

		// Auth check: verify Bearer token if YTGO_AUTH_TOKEN is set.
		// Whitelist paths that don't require authentication.
		if s.authToken != "" && !isAuthWhitelisted(r.URL.Path) {
			if !s.checkAuth(r) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}

		// Bound every request body before a handler attempts to parse it. The
		// cookies endpoint applies the same one-megabyte limit to multipart data.
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
		}

		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	allowed := false
	allowOrigin := origin
	switch {
	case s.corsOrigin == "*":
		allowed = true
		allowOrigin = "*"
	case s.corsOrigin != "":
		allowed = origin == s.corsOrigin
	default:
		parsed, err := url.Parse(origin)
		allowed = err == nil &&
			(parsed.Scheme == "http" || parsed.Scheme == "https") &&
			parsed.User == nil && parsed.Host != "" &&
			parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" &&
			strings.EqualFold(parsed.Host, r.Host)
	}
	if !allowed {
		writeError(w, http.StatusForbidden, fmt.Errorf("CORS origin is not allowed"))
		return true
	}

	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Vary", "Origin")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// isAuthWhitelisted returns true if the path does not require authentication.
func isAuthWhitelisted(path string) bool {
	whitelist := []string{
		"/api/health",
		"/api/config",
	}
	for _, p := range whitelist {
		if path == p {
			return true
		}
	}
	return false
}

// checkAuth validates the Bearer token. A query token is accepted only for the
// SSE endpoint because EventSource cannot set an Authorization header.
func (s *Server) checkAuth(r *http.Request) bool {
	// Check Authorization: Bearer <token>
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if secureTokenEqual(token, s.authToken) {
			return true
		}
	}
	// Check ?token= only for SSE/EventSource, which cannot set custom headers.
	if r.URL.Path == "/api/events" && secureTokenEqual(r.URL.Query().Get("token"), s.authToken) {
		return true
	}
	return false
}

func secureTokenEqual(left, right string) bool {
	leftDigest := sha256.Sum256([]byte(left))
	rightDigest := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftDigest[:], rightDigest[:]) == 1
}
