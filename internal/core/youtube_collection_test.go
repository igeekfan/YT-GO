package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestYouTubeVideoIdentity(t *testing.T) {
	const videoID = "AbCdEfGhI_1"
	for _, variant := range []string{
		"https://www.youtube.com/watch?v=" + videoID + "&t=14&list=PLA",
		"https://youtu.be/" + videoID + "?si=tracking",
		"https://m.youtube.com/shorts/" + videoID,
		"http://youtube.com:80/live/" + videoID + "/",
		"https://music.youtube.com/watch?v=" + videoID,
		"https://www.youtube-nocookie.com/embed/" + videoID,
	} {
		if id := youtubeVideoID(variant); id != videoID {
			t.Errorf("youtubeVideoID(%q) = %q, want %q", variant, id, videoID)
		}
	}
	for _, invalid := range []string{
		"https://www.youtube.com.evil.example/watch?v=" + videoID,
		"https://example.com/shorts/" + videoID,
		"https://www.youtube.com@other.example/watch?v=" + videoID,
		"https://youtube.com/playlist?list=" + videoID,
		"https://youtube.com/watch?v=" + videoID + "&v=Different01",
		"https://youtube.com/watch?v=short",
	} {
		if id := youtubeVideoID(invalid); id != "" {
			t.Errorf("youtubeVideoID(%q) = %q, want empty", invalid, id)
		}
	}
}

func TestYouTubeCollectionURL(t *testing.T) {
	for _, input := range []string{
		"https://www.youtube.com/playlist?list=PL_example",
		"Share https://m.youtube.com/watch?v=AbCdEfGhI_1&list=PL_example.",
		"https://youtu.be/AbCdEfGhI_1?list=PL_example",
	} {
		if got := youtubeCollectionURL(input); got != youtubeOrigin+"/playlist?list=PL_example" {
			t.Errorf("unexpected collection URL for %q: %q", input, got)
		}
	}
	for _, path := range []string{"/@example", "/channel/UC-example/videos", "/user/example/shorts", "/c/example/streams"} {
		if got := youtubeCollectionURL("https://m.youtube.com" + path + "?si=tracking"); got != youtubeOrigin+path {
			t.Errorf("unexpected collection URL for %q: %q", path, got)
		}
	}
	for _, input := range []string{
		"https://youtube.com/watch?v=AbCdEfGhI_1",
		"https://youtube.com/@example/playlists",
		"https://example.com/playlist?list=PL_example",
	} {
		if got := youtubeCollectionURL(input); got != "" {
			t.Errorf("unexpected collection URL for %q: %q", input, got)
		}
	}
}

func TestIncrementalYouTubeCollectionLifecycle(t *testing.T) {
	fixture := newIncrementalYtDlpFixture(t)
	service := startIncrementalTestService(t)
	const (
		playlistURL = "https://www.youtube.com/playlist?list=PL_example"
		channelURL  = "https://www.youtube.com/@example"
		first       = "FirstVideo1"
		second      = "SecondVid01"
		failed      = "FailVideo01"
		cancelled   = "CancelVid01"
		noFile      = "SilentFail1"
	)
	entries := `{"_type":"url","ie_key":"Youtube","id":"FirstVideo1","url":"https://www.youtube.com/shorts/FirstVideo1","title":"First"},
		{"_type":"playlist","entries":[{"_type":"url","ie_key":"Youtube","id":"FirstVideo1","url":"https://youtu.be/FirstVideo1"}]},
		{"id":"FailVideo01","url":"https://www.youtube.com/watch?v=FailVideo01"},
		{"id":"CancelVid01","url":"https://www.youtube.com/live/CancelVid01"},
		{"id":"SilentFail1","ie_key":"Youtube"},
		{"id":"PrivateVid1","title":"[Private video]","availability":"private","url":"https://youtu.be/PrivateVid1"},
		{"id":"DeletedVid1","title":"[Deleted video]","url":"https://youtu.be/DeletedVid1"},
		{"id":"FirstVideo1","url":"https://example.com/video","ie_key":"Other"},
		{"url":"https://youtube.com/watch?v=invalid"},
		null`
	writeIncrementalPlaylist(t, fixture, entries)

	scan := checkIncrementalScan(t, service, playlistURL, []string{first, failed, cancelled, noFile}, 1, 5)
	manual, err := service.GetPlaylistInfo(playlistURL)
	if err != nil || manual.Count != 8 || manual.Videos[0].URL != "https://www.youtube.com/shorts/"+first || manual.Videos[1].URL != "https://youtu.be/"+first {
		t.Fatalf("manual playlist = %+v, err = %v", manual, err)
	}

	cancelReady := make(chan struct{}, 1)
	service.SetHooks(Hooks{DownloadLog: func(_ string, line string) {
		if line == "[fixture] ready to cancel" {
			cancelReady <- struct{}{}
		}
	}})
	outputDir := t.TempDir()
	result, err := service.QueueNewYouTubeVideos(IncrementalDownloadRequest{Videos: scan.NewVideos, OutputDir: outputDir, Quality: "best"})
	if err != nil || result != (IncrementalDownloadResult{Queued: 4}) {
		t.Fatalf("first queue = %+v, err = %v", result, err)
	}
	select {
	case <-cancelReady:
	case <-time.After(10 * time.Second):
		t.Fatal("fixture download did not start")
	}
	waitForIncrementalTask(t, service, first, "completed")
	waitForIncrementalTask(t, service, failed, "error")
	waitForIncrementalTask(t, service, noFile, "error")
	cancelTask := waitForIncrementalTask(t, service, cancelled, "downloading")
	checkIncrementalScan(t, service, playlistURL, []string{failed, noFile}, 3, 5)
	if err := service.CancelDownload(cancelTask.ID); err != nil {
		t.Fatal(err)
	}
	waitForIncrementalTask(t, service, cancelled, "")

	writeIncrementalPlaylist(t, fixture, entries+`,
		{"id":"SecondVid01","url":"https://music.youtube.com/watch?v=SecondVid01","title":"Second"},
		{"id":"ManualVid01","url":"https://www.youtube.com/live/ManualVid01","title":"Manual"}`)
	checkIncrementalScan(t, service, channelURL, []string{failed, cancelled, noFile, second, "ManualVid01"}, 2, 5)

	if _, err := service.StartDownload(DownloadRequest{URL: "https://youtu.be/ManualVid01", OutputDir: outputDir}); err != nil {
		t.Fatal(err)
	}
	waitForIncrementalTask(t, service, "ManualVid01", "completed")

	manualID, err := service.StartDownload(DownloadRequest{URL: "https://youtu.be/" + first + "?si=repeat", OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	manualDownload := waitForIncrementalTask(t, service, "https://youtu.be/"+first+"?si=repeat", "completed")
	if manualDownload.ID != manualID {
		t.Fatalf("manual re-download used task %q, want %q", manualDownload.ID, manualID)
	}
	service.ClearCompleted()
	if count := len(service.GetDownloads()); count != 0 {
		t.Fatalf("history still has %d tasks", count)
	}
	checkIncrementalScan(t, service, channelURL, []string{failed, cancelled, noFile, second}, 3, 5)
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := startIncrementalTestService(t)
	if count := len(restarted.GetDownloads()); count != 0 {
		t.Fatalf("cleared history restored %d tasks", count)
	}
	checkIncrementalScan(t, restarted, channelURL, []string{failed, cancelled, noFile, second}, 3, 5)

	var largePlaylist []string
	for index := range maxQueuedDownloads + 1 {
		largePlaylist = append(largePlaylist, fmt.Sprintf(`{"ie_key":"Youtube","id":"Fixture%04d"}`, index))
	}
	writeIncrementalPlaylist(t, fixture, strings.Join(largePlaylist, ","))
	largeScan, err := restarted.ScanYouTubeCollection(playlistURL)
	if err != nil || largeScan.NewCount != len(largePlaylist) || len(largeScan.NewVideos) != maxQueuedDownloads {
		t.Fatalf("large scan: new count %d, first batch size %d, err = %v", largeScan.NewCount, len(largeScan.NewVideos), err)
	}
}

func TestIncrementalYouTubeBackfillsCompletedHistory(t *testing.T) {
	service := newPersistenceTestServiceWithoutRuntime(t)
	records := []DownloadRecord{
		{ID: "old-success", Status: "completed", URL: "https://youtu.be/FirstVideo1"},
		{ID: "old-duplicate", Status: "completed", URL: "https://youtube.com/shorts/FirstVideo1"},
		{ID: "failed", Status: "error", URL: "https://youtube.com/watch?v=FailVideo01"},
		{ID: "cancelled", Status: "cancelled", URL: "https://youtube.com/watch?v=CancelVid01"},
		{ID: "unknown", Status: "completed", URL: "https://some.example/watch?v=SecondVid01"},
		{ID: "playlist", Status: "completed", URL: "https://youtube.com/playlist?list=SecondVid01"},
	}
	db := service.database()
	if err := db.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := migrateYouTubeDownloads(db); err != nil {
			t.Fatal(err)
		}
	}
	var completed []string
	if err := db.Model(&youtubeDownload{}).Pluck("video_id", &completed).Error; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(completed, []string{"FirstVideo1"}) {
		t.Fatalf("backfilled IDs = %v", completed)
	}
}

func TestIncrementalYouTubeQueueRechecksAndReserves(t *testing.T) {
	service := newPersistenceTestServiceWithoutRuntime(t)
	if err := migrateYouTubeDownloads(service.database()); err != nil {
		t.Fatal(err)
	}
	service.downloadDir = ""
	service.limiter.SetLimit(1)
	if err := service.limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
		service.limiter.Release()
	})

	outputDir := t.TempDir()
	if _, err := service.StartDownload(DownloadRequest{URL: "https://youtu.be/FirstVideo1?t=42", OutputDir: outputDir}); err != nil {
		t.Fatal(err)
	}
	request := IncrementalDownloadRequest{
		Videos: []YouTubePlaylistVideo{{ID: "FirstVideo1"}, {ID: "SecondVid01"}, {ID: "SecondVid01"}}, OutputDir: outputDir,
	}
	results := make(chan IncrementalDownloadResult, 2)
	for range 2 {
		go func() {
			result, err := service.QueueNewYouTubeVideos(request)
			if err != nil {
				t.Error(err)
			}
			results <- result
		}()
	}
	first, second := <-results, <-results
	if first.Queued+second.Queued != 1 || first.Skipped+second.Skipped != 5 || len(service.GetDownloads()) != 2 {
		t.Fatalf("concurrent requests reported %+v and %+v", first, second)
	}
	for _, task := range service.GetDownloads() {
		if err := service.CancelDownload(task.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func newIncrementalYtDlpFixture(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	executable := filepath.Join(binDir, "yt-dlp"+suffix)
	cmd := exec.Command("go", "build", "-o", executable, "testdata/incremental_yt_dlp.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	if err := os.Link(executable, filepath.Join(binDir, "deno"+suffix)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("YTGO_YTDLP_PATH", executable)
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
	config := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", config)
	case "darwin":
		t.Setenv("HOME", config)
	default:
		t.Setenv("XDG_CONFIG_HOME", config)
	}
	path := filepath.Join(binDir, "playlist.json")
	t.Setenv("YTGO_TEST_PLAYLIST_FILE", path)
	return path
}

func startIncrementalTestService(t *testing.T) *Service {
	t.Helper()
	service := NewService("test")
	if err := service.Startup(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	})
	return service
}

func writeIncrementalPlaylist(t *testing.T, name, entries string) {
	t.Helper()
	data := fmt.Sprintf(`{"_type":"playlist","title":"Fixture collection","entries":[%s]}`, entries)
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(data)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, compact.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func checkIncrementalScan(t *testing.T, service *Service, url string, newIDs []string, skipped, unavailable int) IncrementalPlaylistScan {
	t.Helper()
	scan, err := service.ScanYouTubeCollection(url)
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := make([]string, 0, len(scan.NewVideos))
	for _, video := range scan.NewVideos {
		gotIDs = append(gotIDs, video.ID)
	}
	if !slices.Equal(gotIDs, newIDs) || scan.NewCount != len(newIDs) || scan.Skipped != skipped || scan.Unavailable != unavailable {
		t.Fatalf("new IDs %v, new count %d, skipped %d, unavailable %d. Want IDs %v, skipped %d, unavailable %d",
			gotIDs, scan.NewCount, scan.Skipped, scan.Unavailable, newIDs, skipped, unavailable)
	}
	return scan
}

func waitForIncrementalTask(t *testing.T, service *Service, urlOrID, status string) *DownloadTask {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		found := false
		for _, task := range service.GetDownloads() {
			if task.URL == urlOrID || !strings.Contains(urlOrID, "://") && youtubeVideoID(task.URL) == urlOrID {
				if task.Status == status {
					return task
				}
				found = true
			}
		}
		if !found && status == "" {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %q did not reach status %q. Downloads: %+v", urlOrID, status, service.GetDownloads())
	return nil
}
