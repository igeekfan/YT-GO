package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"YT-GO/internal/core"
)

func TestIncrementalPlaylistRoutes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	server := newServerWithDownloadRecords(t, root, []core.DownloadRecord{{
		ID: "prior-success", URL: "https://youtu.be/FirstVideo1", Status: "completed", OutputDir: root,
	}})
	const (
		scanPath  = "/api/video/playlist/scan"
		queuePath = "/api/video/playlist/queue"
	)
	queue := core.IncrementalDownloadRequest{
		Videos:    []core.YouTubePlaylistVideo{{ID: "FirstVideo1"}},
		OutputDir: root,
		Quality:   "best",
	}
	outside := queue
	outside.OutputDir = t.TempDir()
	invalidTemplate := queue
	invalidTemplate.Options = &core.DownloadOptions{FilenameTemplate: "../%(title)s.%(ext)s"}
	invalidID := queue
	invalidID.Videos = []core.YouTubePlaylistVideo{{ID: "https://example.com"}}
	for _, test := range []struct {
		name   string
		method string
		path   string
		auth   bool
		body   any
		status int
	}{
		{"scan requires auth", http.MethodPost, scanPath, false, nil, http.StatusUnauthorized},
		{"queue requires auth", http.MethodPost, queuePath, false, queue, http.StatusUnauthorized},
		{"scan requires POST", http.MethodGet, scanPath, true, nil, http.StatusMethodNotAllowed},
		{"queue requires POST", http.MethodGet, queuePath, true, nil, http.StatusMethodNotAllowed},
		{"scan rejects unsupported collection", http.MethodPost, scanPath, true, URLRequest{URL: "https://example.com/playlist"}, http.StatusBadRequest},
		{"queue rejects outside output directory", http.MethodPost, queuePath, true, outside, http.StatusBadRequest},
		{"queue rejects invalid filename template", http.MethodPost, queuePath, true, invalidTemplate, http.StatusBadRequest},
		{"queue rejects invalid media identity", http.MethodPost, queuePath, true, invalidID, http.StatusBadRequest},
		{"queue skips completed video", http.MethodPost, queuePath, true, queue, http.StatusAccepted},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.body)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(test.method, test.path, bytes.NewReader(body))
			if test.auth {
				request.Header.Set("Authorization", "Bearer secret")
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d, response = %s", response.Code, test.status, response.Body)
			}
			if test.status == http.StatusAccepted {
				var result core.IncrementalDownloadResult
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result != (core.IncrementalDownloadResult{Skipped: 1}) {
					t.Fatalf("queue result = %+v, want 1 skipped", result)
				}
			}
		})
	}
	if count := len(server.service.GetDownloads()); count != 1 {
		t.Fatalf("history contains %d tasks, want the one pre-existing task", count)
	}
}
