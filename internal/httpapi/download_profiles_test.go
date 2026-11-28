package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"YT-GO/internal/core"
)

func TestWebProfileSettingsAndQuickDownload(t *testing.T) {
	setTestUserConfigDir(t)
	root := t.TempDir()
	fixedDir := filepath.Join(root, "fixed")
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
	fixture := filepath.Join(t.TempDir(), "yt-dlp-fixture")
	if err := os.WriteFile(fixture, []byte("fixture to prevent launching system yt-dlp"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YTGO_YTDLP_PATH", fixture)
	service := core.NewService("test")
	if err := service.Startup(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })

	openServer := func(fixed string) *Server {
		t.Helper()
		server, err := newServer(service, serverOptions{downloadRoot: root, fixedDir: fixed, resolver: testResolver{}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = server.Close() })
		return server
	}
	send := func(server *Server, method, path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(data))
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, recorder.Code, want, recorder.Body)
		}
		return recorder
	}

	server := openServer("")
	profile := core.DownloadProfile{ID: "archive", Name: "Archive", Quality: "audio", AudioFormat: "flac", OutputDir: "archive"}
	patch := func(profile core.DownloadProfile) map[string]core.DownloadProfiles {
		return map[string]core.DownloadProfiles{"downloadProfiles": {Items: []core.DownloadProfile{profile}, SelectedID: profile.ID}}
	}
	invalidProfile := profile
	invalidProfile.OutputDir = filepath.Join(t.TempDir(), "outside")
	send(server, http.MethodPatch, "/api/settings", patch(invalidProfile), http.StatusBadRequest)
	recorder := send(server, http.MethodPatch, "/api/settings", patch(profile), http.StatusOK)
	var saved core.Settings
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.DownloadProfiles.SelectedID != profile.ID || len(saved.DownloadProfiles.Items) != 1 {
		t.Fatalf("unexpected saved profiles: %+v", saved.DownloadProfiles)
	}
	expectedDirectory := filepath.Join(root, "archive")
	if saved.DownloadProfiles.Items[0].OutputDir != expectedDirectory {
		t.Fatalf("saved directory = %q, want %q", saved.DownloadProfiles.Items[0].OutputDir, expectedDirectory)
	}

	profile.OutputDir = ""
	send(server, http.MethodPatch, "/api/settings", patch(profile), http.StatusOK)
	if got := service.GetSettings().DownloadProfiles.Items[0].OutputDir; got != "" {
		t.Fatalf("clearing a profile directory should restore inheritance, got %q", got)
	}

	request := core.DownloadRequest{
		URL: "https://video.example/clip", OutputDir: expectedDirectory, Quality: "audio",
		Options: &core.DownloadOptions{SingleItem: true, AudioFormat: "flac"},
	}
	invalidRequest := request
	invalidRequest.URL = "https://video.example/one https://video.example/two"
	send(server, http.MethodPost, "/api/downloads", invalidRequest, http.StatusBadRequest)
	invalidRequest = request
	invalidRequest.Options = &core.DownloadOptions{SingleItem: true, AudioFormat: "unknown"}
	send(server, http.MethodPost, "/api/downloads", invalidRequest, http.StatusBadRequest)
	start := func(server *Server, req core.DownloadRequest, expectedDir string) {
		t.Helper()
		recorder := send(server, http.MethodPost, "/api/downloads", req, http.StatusAccepted)
		var result map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		task, err := service.GetDownload(result["id"])
		if err != nil {
			t.Fatal(err)
		}
		if task.OutputDir != expectedDir || task.Quality != req.Quality || task.URL != req.URL {
			t.Fatalf("unexpected queued request: %+v", task)
		}
	}
	start(server, request, expectedDirectory)
	start(openServer(fixedDir), request, fixedDir)
}
