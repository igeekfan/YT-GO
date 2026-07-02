package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"YT-GO/internal/core"
)

func TestIsAuthWhitelistedExcludesEvents(t *testing.T) {
	if isAuthWhitelisted("/api/events") {
		t.Fatal("/api/events must require auth when YTGO_AUTH_TOKEN is set")
	}
}

func TestIsAuthWhitelistedAllowsHealthAndConfig(t *testing.T) {
	for _, path := range []string{"/api/health", "/api/config"} {
		if !isAuthWhitelisted(path) {
			t.Fatalf("%s should be whitelisted", path)
		}
	}
}

func TestCheckAuthAcceptsBearerToken(t *testing.T) {
	s := &Server{authToken: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/api/downloads", nil)
	r.Header.Set("Authorization", "Bearer secret")

	if !s.checkAuth(r) {
		t.Fatal("expected bearer token to authenticate")
	}
}

func TestCheckAuthAcceptsQueryToken(t *testing.T) {
	s := &Server{authToken: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/api/events?token=secret", nil)

	if !s.checkAuth(r) {
		t.Fatal("expected query token to authenticate")
	}
}

func TestCheckAuthRejectsWrongToken(t *testing.T) {
	s := &Server{authToken: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/api/downloads?token=wrong", nil)
	r.Header.Set("Authorization", "Bearer wrong")

	if s.checkAuth(r) {
		t.Fatal("expected wrong token to be rejected")
	}
}

func TestYtDlpVersionCheckRouteRejectsUnsupportedMethod(t *testing.T) {
	s := New(core.NewService("test"))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/ytdlp/version-check", nil)

	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
	if allow := w.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, allow)
	}
}
