package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"YT-GO/internal/core"
)

func downloadAPIPost(t *testing.T, server *Server, path string, body any, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	if auth {
		request.Header.Set("Authorization", "Bearer secret")
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	return recorder
}

func TestDownloadBatchReviewAndPartialQueueWeb(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	server := newServerWithDownloadRecords(t, root, nil)
	input := "https://video.example/first\nfile:///movie.mp4\nhttps://VIDEO.example:443/first\n" +
		"http://127.0.0.1/movie.mp4\nhttps://video.example/second\nhttps://another.example/third"
	body := map[string]string{"input": input}
	if result := downloadAPIPost(t, server, "/api/downloads/review", body, false); result.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated review: %d %s", result.Code, result.Body.String())
	}
	result := downloadAPIPost(t, server, "/api/downloads/review", body, true)
	if result.Code != http.StatusOK {
		t.Fatalf("review: %d %s", result.Code, result.Body.String())
	}
	var entries []core.DownloadURLReview
	if err := json.Unmarshal(result.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	wantStatuses := []string{"valid", "invalid", "duplicate", "invalid", "valid", "valid"}
	if len(entries) != len(wantStatuses) {
		t.Fatalf("review entries=%+v", entries)
	}
	for index, entry := range entries {
		if entry.Status != wantStatuses[index] {
			t.Errorf("line %d status=%s, want %s", entry.Line, entry.Status, wantStatuses[index])
		}
	}
	if entries[3].Error == "" || len(server.service.GetDownloads()) != 0 {
		t.Fatalf("review lacked policy feedback or added downloads: %+v", entries)
	}

	enqueue := func(rawURL, outputDir string, status int) {
		t.Helper()
		result := downloadAPIPost(t, server, "/api/downloads", core.DownloadRequest{
			URL: rawURL, OutputDir: outputDir, Quality: "720p",
		}, true)
		if result.Code != status {
			t.Errorf("enqueue %s: %d want %d, %s", rawURL, result.Code, status, result.Body.String())
		}
	}
	enqueue(entries[0].URL, root, http.StatusAccepted)
	enqueue(entries[4].URL, filepath.Join(filepath.Dir(root), "outside"), http.StatusBadRequest)
	enqueue(entries[3].URL, root, http.StatusBadRequest)
	enqueue(entries[5].URL, root, http.StatusAccepted)

	queued := server.service.GetDownloads()
	if len(queued) != 2 {
		t.Fatalf("expected two queued entries after partial failure, got %+v", queued)
	}
	for _, task := range queued {
		if !samePath(task.OutputDir, root) || task.Quality != "720p" ||
			(task.URL != entries[0].URL && task.URL != entries[5].URL) {
			t.Errorf("unexpected queued entry: %+v", task)
		}
	}
}

func TestDownloadRetryWebRevalidatesLegacyURLAndDestination(t *testing.T) {
	root := filepath.Join(t.TempDir(), "downloads")
	created := time.Now().Format(time.RFC3339)
	server := newServerWithDownloadRecords(t, root, []core.DownloadRecord{
		{ID: "legacy", URL: "https://video.example/legacy", Status: "error", OutputDir: root, Quality: "480p", CreatedAt: created},
		{ID: "blocked-url", URL: "http://127.0.0.1/movie.mp4", Status: "error", OutputDir: root, CreatedAt: created},
		{ID: "blocked-dir", URL: "https://video.example/outside", Status: "error", OutputDir: filepath.Join(filepath.Dir(root), "outside"), CreatedAt: created},
	})

	if result := downloadAPIPost(t, server, "/api/downloads/legacy/retry", nil, false); result.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated retry: %d %s", result.Code, result.Body.String())
	}
	for _, id := range []string{"blocked-url", "blocked-dir"} {
		result := downloadAPIPost(t, server, "/api/downloads/"+id+"/retry", nil, true)
		if result.Code != http.StatusBadRequest {
			t.Errorf("retry %s: %d %s", id, result.Code, result.Body.String())
		}
	}
	result := downloadAPIPost(t, server, "/api/downloads/legacy/retry", nil, true)
	if result.Code != http.StatusAccepted {
		t.Fatalf("retry legacy entry: %d %s", result.Code, result.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	retry, err := server.service.GetDownload(response["id"])
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID == "legacy" || retry.Quality != "480p" || !samePath(retry.OutputDir, root) || len(server.service.GetDownloads()) != 4 {
		t.Fatalf("unexpected retry result: %+v", retry)
	}
}
