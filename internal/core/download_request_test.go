package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func useLocalDownloadFixture(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", configDir)
	case "darwin":
		t.Setenv("HOME", configDir)
	default:
		t.Setenv("XDG_CONFIG_HOME", configDir)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YTGO_YTDLP_PATH", executable)
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
}

func openDownloadTestService(t *testing.T) *Service {
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

func runDownloadAttempt(t *testing.T, service *Service, start func() (string, error)) (*DownloadTask, string) {
	t.Helper()
	logs := make(chan string, 1)
	finished := make(chan *DownloadTask, 1)
	service.SetHooks(Hooks{
		AppLog: func(line string) {
			if strings.HasPrefix(line, "[runDownload] exec:") {
				logs <- line
			}
		},
		DownloadUpdate: func(task *DownloadTask) {
			if task.Status == "error" || task.Status == "completed" {
				finished <- task
			}
		},
	})
	taskID, err := start()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case task := <-finished:
		if task.ID != taskID || task.Status != "error" {
			t.Fatalf("expected the local fixture to fail task %q, got %+v", taskID, task)
		}
		select {
		case command := <-logs:
			return task, command
		default:
			t.Fatalf("task failed before the fixture command was built: %s", task.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the local download fixture")
	}
	return nil, ""
}

func assertDownloadCommand(t *testing.T, command string, want, unwanted []string) {
	t.Helper()
	for _, value := range want {
		if !strings.Contains(command, value) {
			t.Errorf("command missing %q: %s", value, command)
		}
	}
	for _, value := range unwanted {
		if strings.Contains(command, value) {
			t.Errorf("unexpected %q in command: %s", value, command)
		}
	}
}

func TestRetryDownloadPreservesMediaOptionsAfterRestart(t *testing.T) {
	useLocalDownloadFixture(t)
	service := openDownloadTestService(t)
	settings := service.GetSettings()
	settings.FilenameTemplate = "original-default.%(ext)s"
	settings.AudioFormat = "flac"
	settings.MergeOutputFormat = "mkv"
	settings.SaveDescription = true
	settings.EmbedChapters = true
	settings.SponsorBlock = true
	settings.SubtitleLangs = "fr"
	settings.WriteSubtitles = true
	settings.RateLimit = "1M"
	proxyCredential := strings.Join([]string{"original", "proxy", "fixture"}, "-")
	settings.Proxy = "https://proxy-user:" + proxyCredential + "@proxy.example:8443"
	settings.CookiesFrom = "edge"
	settings.CookiesFile = "original-cookie-fixture.txt"
	if err := service.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	req := DownloadRequest{
		URL: "https://video.example/watch?id=123", OutputDir: t.TempDir(), Quality: "fa:251",
		VideoInfo: &VideoInfo{
			Title: "Fixture title", Thumbnail: "https://video.example/cover.jpg?token=thumb-fixture",
		},
		Options: &DownloadOptions{
			FilenameTemplate:  "selected-%(id)s.%(ext)s",
			SaveDescription:   new(false),
			SaveThumbnail:     new(true),
			WriteManualSubs:   new(false),
			WriteAutoSubs:     new(true),
			AutoSubtitleLangs: "en-zh-Hans",
			EmbedSubtitles:    new(true),
		},
	}
	selected, command := runDownloadAttempt(t, service, func() (string, error) { return service.StartDownload(req) })
	selectedOptions := []string{
		"--format 251", "--extract-audio", "--audio-format flac", "--write-thumbnail",
		"--write-auto-subs", "--sub-langs en-zh-Hans", "--embed-subs", "--embed-chapters",
		"--sponsorblock-mark all", "--limit-rate 1M", "selected-%(id)s.%(ext)s",
	}
	assertDownloadCommand(t, command, selectedOptions, []string{"--write-subs", "--write-description", "--merge-output-format"})

	preset, _ := runDownloadAttempt(t, service, func() (string, error) {
		return service.StartDownload(DownloadRequest{
			URL: "https://video.example/preset", OutputDir: req.OutputDir, Quality: "720p",
		})
	})
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := openDownloadTestService(t)
	var storedOptions, thumbnail string
	err := restarted.db.Raw("SELECT media_options, thumbnail FROM download_records WHERE id = ?", selected.ID).
		Row().Scan(&storedOptions, &thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{proxyCredential, "original-cookie-fixture", "thumb-fixture", `"proxy"`, `cookies`} {
		if strings.Contains(storedOptions+thumbnail, secret) {
			t.Fatalf("task record persisted credential marker %q", secret)
		}
	}

	settings.FilenameTemplate = "changed.%(ext)s"
	settings.AudioFormat = "mp3"
	settings.MergeOutputFormat = "webm"
	settings.WriteSubtitles = false
	settings.SubtitleLangs = "ja"
	settings.EmbedChapters = false
	settings.SponsorBlock = false
	settings.RateLimit = "2M"
	settings.Proxy = "https://proxy.example:9443"
	settings.CookiesFrom = "firefox"
	if err := restarted.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	retry, command := runDownloadAttempt(t, restarted, func() (string, error) {
		return restarted.RetryDownload(selected.ID, func(rawURL, outputDir, filenameTemplate string) (string, string, error) {
			if rawURL != req.URL || outputDir != req.OutputDir || filenameTemplate != req.Options.FilenameTemplate {
				t.Fatalf("retry validation received unexpected URL, destination, or template: %s %s %s", rawURL, outputDir, filenameTemplate)
			}
			return rawURL, outputDir, nil
		})
	})
	if retry.ID == selected.ID || retry.Quality != req.Quality || retry.Title != req.VideoInfo.Title || retry.OutputDir != req.OutputDir {
		t.Fatalf("retry did not preserve source metadata or allocate a new attempt: %+v", retry)
	}
	assertDownloadCommand(t, command, append(selectedOptions, "https://proxy.example:9443", "--cookies-from-browser firefox"),
		[]string{proxyCredential, "--write-subs", "--write-description", "--merge-output-format", "changed.", "--audio-format mp3"})

	_, command = runDownloadAttempt(t, restarted, func() (string, error) {
		return restarted.RetryDownload(preset.ID, nil)
	})
	assertDownloadCommand(t, command,
		[]string{"height<=720", "--merge-output-format mkv", "--write-subs", "--write-auto-subs", "--sub-langs fr", "original-default.%(ext)s"},
		[]string{"--merge-output-format webm", "changed."})
}

func TestRetryDownloadWorksWithPreSnapshotHistory(t *testing.T) {
	useLocalDownloadFixture(t)
	appDir := NewService("test").GetDataDir()
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(appDir, "history.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE download_records (
		id TEXT PRIMARY KEY, url TEXT NOT NULL, quality TEXT, status TEXT NOT NULL DEFAULT 'pending',
		output_dir TEXT NOT NULL, created_at TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	if err := db.Exec(`INSERT INTO download_records (id, url, quality, status, output_dir, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"legacy", "https://video.example/legacy", "", "error", outputDir, time.Now().Format(time.RFC3339)).Error; err != nil {
		t.Fatal(err)
	}
	legacyDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}

	service := openDownloadTestService(t)
	settings := service.GetSettings()
	settings.Quality = "720p"
	settings.FilenameTemplate = "legacy-current.%(ext)s"
	if err := service.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	retry, command := runDownloadAttempt(t, service, func() (string, error) { return service.RetryDownload("legacy", nil) })
	if retry.Quality != "best" || retry.OutputDir != outputDir || retry.mediaOptions == nil {
		t.Fatalf("legacy retry has unexpected options: %+v", retry)
	}
	assertDownloadCommand(t, command, []string{"legacy-current.%(ext)s", "https://video.example/legacy"}, []string{"height<=720"})
}

func TestDownloadHistoryRedactsSourceCredentials(t *testing.T) {
	useLocalDownloadFixture(t)
	service := openDownloadTestService(t)
	sourceCredential := "fixture-" + "source-credential"
	rawURL := "https://fixture-user:" + sourceCredential + "@video.example/watch?v=123&%61ccess_token=fixture-token#watch?refresh_token=fixture-refresh"
	task, command := runDownloadAttempt(t, service, func() (string, error) {
		return service.StartDownload(DownloadRequest{URL: rawURL, OutputDir: t.TempDir()})
	})
	assertDownloadCommand(t, command, []string{rawURL}, nil)
	if task.URL != "https://video.example/watch?v=123" {
		t.Fatalf("unexpected redacted URL: %q", task.URL)
	}
	_, command = runDownloadAttempt(t, service, func() (string, error) { return service.RetryDownload(task.ID, nil) })
	assertDownloadCommand(t, command, []string{rawURL}, nil)
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := openDownloadTestService(t)
	var url, options string
	if err := restarted.db.Raw("SELECT url, media_options FROM download_records WHERE id = ?", task.ID).
		Row().Scan(&url, &options); err != nil {
		t.Fatal(err)
	}
	if url != task.URL || strings.Contains(options, "fixture-") {
		t.Fatalf("stored credential or changed safe URL: %s %s", url, options)
	}
	if _, err := restarted.RetryDownload(task.ID, nil); err == nil || err.Error() != restarted.i18n.T("download.retry_credentials") {
		t.Fatalf("credential-dependent retry after restart returned %v", err)
	}
}

func TestReviewDownloadURLsPreservesValidLinesAndMarksDuplicates(t *testing.T) {
	results := ReviewDownloadURLs(" https://VIDEO.example:443 \r\ninvalid\r\n\r\nhttps://video.example/\rhttps://video.example/?id=abc\n"+
		"分享 https://second.example/watch?id=123。 \nfile:///movie.mp4\nhttps:opaque", nil)
	wantLines := []int{1, 2, 4, 5, 6, 7, 8}
	wantStatuses := []string{"valid", "invalid", "duplicate", "valid", "valid", "invalid", "invalid"}
	if len(results) != len(wantStatuses) {
		t.Fatalf("results=%+v", results)
	}
	for index, result := range results {
		if result.Line != wantLines[index] || result.Status != wantStatuses[index] {
			t.Errorf("result %d=%+v, want line %d status %s", index, result, wantLines[index], wantStatuses[index])
		}
	}
	if results[4].URL != "https://second.example/watch?id=123" {
		t.Fatalf("shared link was not extracted: %+v", results[4])
	}
	if results[0].Key != results[2].Key || results[0].Key == results[3].Key {
		t.Fatalf("unexpected duplicate keys: %+v", results)
	}
}
