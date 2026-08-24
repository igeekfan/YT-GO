package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"YT-GO/internal/core"
)

type testResolver struct{}

func (testResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	server, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: filepath.Join(t.TempDir(), "downloads"),
		resolver:     testResolver{},
	})
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func TestIsAuthWhitelistedExcludesEvents(t *testing.T) {
	if isAuthWhitelisted("/api/events") {
		t.Fatal("/api/events must require auth when YTGO_AUTH_TOKEN is set")
	}
}

func TestNewServerRequiresCoreService(t *testing.T) {
	if _, err := newServer(nil, serverOptions{downloadRoot: t.TempDir()}); err == nil {
		t.Fatal("expected nil core service to be rejected")
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

func TestCheckAuthRejectsQueryTokenOutsideEvents(t *testing.T) {
	s := &Server{authToken: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/api/version?token=secret", nil)

	if s.checkAuth(r) {
		t.Fatal("query token must be restricted to the SSE endpoint")
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
	s := newTestServer(t)
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

func TestAuthMiddlewareWhitelistsBootstrapAndProtectsAPIs(t *testing.T) {
	s, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: filepath.Join(t.TempDir(), "downloads"),
		resolver:     testResolver{},
		authToken:    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, path := range []string{"/api/health", "/api/config"} {
		recorder := httptest.NewRecorder()
		s.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s status=%d, want %d", path, recorder.Code, http.StatusOK)
		}
	}

	for _, path := range []string{"/api/version", "/api/downloads", "/api/downloads/task/file"} {
		recorder := httptest.NewRecorder()
		s.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("GET %s status=%d, want %d", path, recorder.Code, http.StatusUnauthorized)
		}
	}

	queryRequest := httptest.NewRequest(http.MethodGet, "/api/version?token=secret", nil)
	queryRecorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(queryRecorder, queryRequest)
	if queryRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("query token authenticated a non-SSE route: status=%d", queryRecorder.Code)
	}

	bearerRequest := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	bearerRequest.Header.Set("Authorization", "Bearer secret")
	bearerRecorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(bearerRecorder, bearerRequest)
	if bearerRecorder.Code != http.StatusOK {
		t.Fatalf("valid bearer status=%d, want %d: %s", bearerRecorder.Code, http.StatusOK, bearerRecorder.Body.String())
	}
}

func TestConfigHidesDeploymentDetailsUntilAuthenticated(t *testing.T) {
	t.Setenv("YTGO_EXTERNAL_URL", "https://files.example.test")
	root := filepath.Join(t.TempDir(), "downloads")
	fixedDir := filepath.Join(root, "fixed")
	s, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: root,
		fixedDir:     fixedDir,
		resolver:     testResolver{},
		authToken:    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	type configResponse struct {
		DownloadDir  string `json:"downloadDir"`
		ExternalURL  string `json:"externalURL"`
		HasFixedDir  bool   `json:"hasFixedDir"`
		AuthRequired bool   `json:"authRequired"`
	}
	requestConfig := func(token string) configResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		s.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("config status=%d: %s", recorder.Code, recorder.Body.String())
		}
		var response configResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	unauthenticated := requestConfig("")
	if !unauthenticated.AuthRequired || unauthenticated.DownloadDir != "" || unauthenticated.ExternalURL != "" || unauthenticated.HasFixedDir {
		t.Fatalf("unauthenticated config leaked details: %+v", unauthenticated)
	}
	wrong := requestConfig("wrong")
	if wrong.DownloadDir != "" || wrong.ExternalURL != "" || wrong.HasFixedDir {
		t.Fatalf("invalid bearer config leaked details: %+v", wrong)
	}
	authenticated := requestConfig("secret")
	if authenticated.DownloadDir != s.fixedDir || authenticated.DownloadDir == s.DownloadRoot() || authenticated.ExternalURL != "https://files.example.test" || !authenticated.HasFixedDir {
		t.Fatalf("authenticated config does not reflect policy: %+v", authenticated)
	}
}

func TestSSERequiresAuthAndAcceptsEventSourceQueryToken(t *testing.T) {
	s, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: filepath.Join(t.TempDir(), "downloads"),
		resolver:     testResolver{},
		authToken:    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	testServer := httptest.NewServer(s.Handler())
	defer testServer.Close()
	client := &http.Client{}

	response, err := client.Get(testServer.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated SSE status=%d, want %d", response.StatusCode, http.StatusUnauthorized)
	}

	response, err = client.Get(testServer.URL + "/api/events?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	response.Body.Close()
	if err != nil {
		t.Fatalf("read SSE greeting: %v", err)
	}
	if response.StatusCode != http.StatusOK || line != ": connected\n" {
		t.Fatalf("authenticated SSE status=%d greeting=%q", response.StatusCode, line)
	}
}

func TestCORSPreflightRequiresAllowedOrigin(t *testing.T) {
	s, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: filepath.Join(t.TempDir(), "downloads"),
		resolver:     testResolver{},
		corsOrigin:   "https://ui.example.test",
		authToken:    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	request := httptest.NewRequest(http.MethodOptions, "/api/downloads", nil)
	request.Header.Set("Origin", "https://ui.example.test")
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || recorder.Header().Get("Access-Control-Allow-Origin") != "https://ui.example.test" {
		t.Fatalf("allowed preflight status=%d origin=%q", recorder.Code, recorder.Header().Get("Access-Control-Allow-Origin"))
	}

	request = httptest.NewRequest(http.MethodOptions, "/api/downloads", nil)
	request.Header.Set("Origin", "https://attacker.example.test")
	recorder = httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed preflight status=%d origin=%q", recorder.Code, recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestOriginPolicyBlocksCrossOriginSimplePostBeforeHandler(t *testing.T) {
	s := newTestServer(t)
	originalLanguage := s.service.GetLang()
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/lang", strings.NewReader(`{"lang":"en-US"}`))
	request.Header.Set("Origin", "https://attacker.example.test")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin simple POST status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
	if language := s.service.GetLang(); language != originalLanguage {
		t.Fatalf("blocked handler mutated language from %q to %q", originalLanguage, language)
	}
}

func TestOriginPolicyAllowsSameHostIgnoringScheme(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "http://app.example.test/api/lang", strings.NewReader(`{"lang":"en-US"}`))
	request.Header.Set("Origin", "https://app.example.test")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("same-host POST status=%d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "https://app.example.test" {
		t.Fatalf("same-host allow-origin=%q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if language := s.service.GetLang(); language != "en-US" {
		t.Fatalf("same-host handler language=%q, want en-US", language)
	}
}

func TestOriginPolicyAllowsConfiguredOrigin(t *testing.T) {
	s, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: filepath.Join(t.TempDir(), "downloads"),
		resolver:     testResolver{},
		corsOrigin:   "https://ui.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	request := httptest.NewRequest(http.MethodPost, "http://api.example.test/api/lang", strings.NewReader(`{"lang":"en-US"}`))
	request.Header.Set("Origin", "https://ui.example.test")
	recorder := httptest.NewRecorder()

	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("configured-origin POST status=%d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "https://ui.example.test" {
		t.Fatalf("configured allow-origin=%q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}
