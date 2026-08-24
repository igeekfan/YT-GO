package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"YT-GO/internal/core"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setTestUserConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	return dir
}

func multipartRequest(t *testing.T, path, name, contents string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestCookiesUploadEnforcesAggregateBodyLimit(t *testing.T) {
	setTestUserConfigDir(t)
	server := newTestServer(t)
	request := multipartRequest(t, "/api/cookies/upload", "cookies.txt", strings.Repeat("a", maxJSONBodyBytes+1))
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want %d: %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
	}
}

func TestCookiesUploadRequiresAuthentication(t *testing.T) {
	setTestUserConfigDir(t)
	server, err := newServer(core.NewService("test"), serverOptions{
		downloadRoot: filepath.Join(t.TempDir(), "downloads"),
		resolver:     testResolver{},
		authToken:    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	request := multipartRequest(t, "/api/cookies/upload", "cookies.txt", "cookies")
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d: %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}

func TestCookiesUploadUsesPrivateUniqueServerNames(t *testing.T) {
	setTestUserConfigDir(t)
	server := newTestServer(t)
	upload := func(contents string) map[string]string {
		t.Helper()
		request := multipartRequest(t, "/api/cookies/upload", `..\chosen-name.txt`, contents)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("upload status=%d: %s", recorder.Code, recorder.Body.String())
		}
		var response map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	first := upload("first")
	second := upload("second")
	if first["path"] == second["path"] || first["name"] == second["name"] {
		t.Fatalf("uploads reused a destination: first=%v second=%v", first, second)
	}
	for _, result := range []map[string]string{first, second} {
		if filepath.Base(result["path"]) != result["name"] || !strings.HasPrefix(result["name"], "cookies-") {
			t.Fatalf("upload used an unsafe name: %v", result)
		}
		info, err := os.Stat(result["path"])
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("uploaded cookie mode=%#o, want 0600", info.Mode().Perm())
		}
	}
	firstContents, err := os.ReadFile(first["path"])
	if err != nil {
		t.Fatal(err)
	}
	if string(firstContents) != "first" {
		t.Fatalf("first upload was overwritten: %q", firstContents)
	}
}

func newServerWithDownloadRecords(t *testing.T, root string, records []core.DownloadRecord, settingRecords ...core.SettingsRecord) *Server {
	t.Helper()
	configBase := setTestUserConfigDir(t)
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
	t.Setenv("YTGO_YTDLP_PATH", "")
	appDir := filepath.Join(configBase, "YT-GO")
	if runtime.GOOS == "darwin" {
		appDir = filepath.Join(configBase, "Library", "Application Support", "YT-GO")
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(appDir, "history.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&core.DownloadRecord{}, &core.SettingsRecord{}); err != nil {
		t.Fatal(err)
	}
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := range settingRecords {
		if err := db.Create(&settingRecords[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	service := core.NewService("test")
	if err := service.Startup(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	server, err := newServer(service, serverOptions{
		downloadRoot: root,
		resolver:     testResolver{},
		authToken:    "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func TestDownloadFileRouteAuthStatusAndConfinement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	safePath := filepath.Join(root, "safe-video.mp4")
	if err := os.WriteFile(safePath, []byte("video"), 0o640); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(filepath.Dir(root), "outside.mp4")
	if err := os.WriteFile(outsidePath, []byte("secret"), 0o640); err != nil {
		t.Fatal(err)
	}
	records := []core.DownloadRecord{
		{ID: "safe", URL: "https://video.example/safe", Status: "completed", OutputPath: safePath, OutputDir: root, CreatedAt: time.Now().Format(time.RFC3339)},
		{ID: "outside", URL: "https://video.example/outside", Status: "completed", OutputPath: outsidePath, OutputDir: root, CreatedAt: time.Now().Format(time.RFC3339)},
		{ID: "incomplete", URL: "https://video.example/incomplete", Status: "error", OutputPath: safePath, OutputDir: root, CreatedAt: time.Now().Format(time.RFC3339)},
	}
	symlinkPath := filepath.Join(root, "escape.mp4")
	symlinkAvailable := os.Symlink(outsidePath, symlinkPath) == nil
	if symlinkAvailable {
		records = append(records, core.DownloadRecord{ID: "symlink", URL: "https://video.example/symlink", Status: "completed", OutputPath: symlinkPath, OutputDir: root, CreatedAt: time.Now().Format(time.RFC3339)})
	}
	server := newServerWithDownloadRecords(t, root, records)

	request := httptest.NewRequest(http.MethodGet, "/api/downloads/safe/file", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated file status=%d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	getFile := func(id string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/api/downloads/"+id+"/file", nil)
		request.Header.Set("Authorization", "Bearer secret")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		return recorder
	}
	safe := getFile("safe")
	if safe.Code != http.StatusOK || safe.Body.String() != "video" {
		t.Fatalf("safe file status=%d body=%q", safe.Code, safe.Body.String())
	}
	if disposition := safe.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("unexpected content disposition: %q", disposition)
	}
	if outside := getFile("outside"); outside.Code != http.StatusForbidden {
		t.Fatalf("outside file status=%d, want %d", outside.Code, http.StatusForbidden)
	}
	if incomplete := getFile("incomplete"); incomplete.Code != http.StatusConflict {
		t.Fatalf("incomplete file status=%d, want %d", incomplete.Code, http.StatusConflict)
	}
	if symlinkAvailable {
		if symlink := getFile("symlink"); symlink.Code != http.StatusForbidden {
			t.Fatalf("escaping symlink status=%d, want %d", symlink.Code, http.StatusForbidden)
		}
	}
}

func TestSettingsProtectsProxyAndCookieFileBoundaries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	server := newServerWithDownloadRecords(t, root, nil)
	cookiesDir := filepath.Join(server.service.GetDataDir(), "cookies")
	if err := os.MkdirAll(cookiesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	validCookie := filepath.Join(cookiesDir, "cookies-valid.txt")
	if err := os.WriteFile(validCookie, []byte("cookies"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideCookie := filepath.Join(filepath.Dir(cookiesDir), "outside.txt")
	if err := os.WriteFile(outsideCookie, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	base := core.Settings{
		OutputDir:     root,
		Quality:       "best",
		Language:      "en-US",
		Theme:         "dark",
		MaxConcurrent: 3,
	}
	postSettings := func(settings core.Settings) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer secret")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		return recorder
	}

	privateProxy := base
	privateProxy.Proxy = "http://127.0.0.1:8080"
	if recorder := postSettings(privateProxy); recorder.Code != http.StatusBadRequest {
		t.Fatalf("private proxy status=%d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	browserCookie := base
	browserCookie.CookiesFrom = "chrome"
	if recorder := postSettings(browserCookie); recorder.Code != http.StatusBadRequest {
		t.Fatalf("browser cookie import status=%d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	escapingCookie := base
	escapingCookie.CookiesFile = outsideCookie
	if recorder := postSettings(escapingCookie); recorder.Code != http.StatusBadRequest {
		t.Fatalf("outside cookie file status=%d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	symlinkCookie := filepath.Join(cookiesDir, "cookies-link.txt")
	if err := os.Symlink(outsideCookie, symlinkCookie); err == nil {
		settings := base
		settings.CookiesFile = symlinkCookie
		if recorder := postSettings(settings); recorder.Code != http.StatusBadRequest {
			t.Fatalf("symlink cookie file status=%d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	}

	valid := base
	valid.Proxy = "https://proxy.example:8443"
	valid.CookiesFile = validCookie
	if recorder := postSettings(valid); recorder.Code != http.StatusOK {
		t.Fatalf("valid settings status=%d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get settings status=%d: %s", recorder.Code, recorder.Body.String())
	}
	var saved core.Settings
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.OutputDir != server.DownloadRoot() || saved.Proxy != valid.Proxy || saved.CookiesFile != validCookie || saved.CookiesFrom != "" {
		t.Fatalf("saved settings escaped normalization: %+v", saved)
	}
}

func TestOutboundEndpointsRejectUnsafePersistedWebSettings(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	server := newServerWithDownloadRecords(t, root, nil, core.SettingsRecord{
		ID:            1,
		OutputDir:     root,
		MaxConcurrent: 3,
		Proxy:         "http://127.0.0.1:8080",
	})
	body := strings.NewReader(`{"url":"https://video.example/watch/1"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/video/info", body)
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "proxy endpoint") {
		t.Fatalf("unsafe persisted proxy status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNewRejectsFixedDownloadDirOutsidePolicyRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	t.Setenv("YTGO_WEB_DOWNLOAD_ROOT", root)
	t.Setenv("YTGO_DOWNLOAD_DIR", outside)
	if _, err := New(core.NewService("test")); err == nil {
		t.Fatal("expected conflicting fixed directory and policy root to be rejected")
	}
}

func TestNewUsesSafeBareDefaultDownloadRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("YTGO_WEB_DOWNLOAD_ROOT", "")
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
	server, err := New(core.NewService("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	expected, err := filepath.EvalSymlinks(filepath.Join(home, "Downloads"))
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(server.DownloadRoot(), expected) || server.fixedDir != "" {
		t.Fatalf("root=%q fixed=%q, want root=%q without fixed output", server.DownloadRoot(), server.fixedDir, expected)
	}
}

func TestHTTPServerRejectsOversizedHeaders(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewHTTPServer(listener.Addr().String(), http.NotFoundHandler())
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		<-serveErr
	})

	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintf(connection, "GET / HTTP/1.1\r\nHost: localhost\r\nX-Oversized: %s\r\n\r\n", strings.Repeat("a", maxHeaderBytes+8192)); err != nil {
		t.Fatal(err)
	}
	statusLine, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusLine, "431") {
		t.Fatalf("oversized header response=%q, want 431", statusLine)
	}
}

func TestHTTPServerGracefulShutdownDrainsSSE(t *testing.T) {
	api := newTestServer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewHTTPServer(listener.Addr().String(), api.Handler())
	server.RegisterOnShutdown(api.Hub().Close)
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	response, err := http.Get("http://" + listener.Addr().String() + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != ": connected\n" {
		response.Body.Close()
		t.Fatalf("SSE greeting=%q err=%v", line, err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err = server.Shutdown(shutdownCtx)
	cancel()
	if err != nil {
		response.Body.Close()
		t.Fatalf("graceful shutdown: %v", err)
	}
	_, readErr := io.Copy(io.Discard, reader)
	response.Body.Close()
	if readErr != nil && !errors.Is(readErr, net.ErrClosed) {
		t.Fatalf("drain SSE response: %v", readErr)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve returned %v, want http.ErrServerClosed", err)
	}
}
