package desktop

import (
	"os"
	"strings"
	"testing"
	"time"

	"YT-GO/internal/core"
)

func TestDesktopDownloadReviewAndRetry(t *testing.T) {
	fixture, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YTGO_YTDLP_PATH", fixture)
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
	service := core.NewService("test")
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	})
	app := &App{service: service}
	entries := app.ReviewDownloadURLs("https://video.example/1\nbad link\nhttps://VIDEO.example/1\nhttps://video.example/2")
	if len(entries) != 4 || entries[0].Status != "valid" || entries[1].Status != "invalid" ||
		entries[2].Status != "duplicate" || entries[3].Status != "valid" {
		t.Fatalf("unexpected desktop review: %+v", entries)
	}

	commands := make(chan string, 3)
	finished := make(chan string, 3)
	service.SetHooks(core.Hooks{
		AppLog: func(line string) {
			if strings.HasPrefix(line, "[runDownload] exec:") {
				commands <- line
			}
		},
		DownloadUpdate: func(task *core.DownloadTask) {
			if task.Status == "error" || task.Status == "completed" {
				finished <- task.ID
			}
		},
	})
	waitForAttempt := func(taskID string) string {
		t.Helper()
		select {
		case result := <-finished:
			if result != taskID {
				t.Fatalf("received task %s, want %s", result, taskID)
			}
			select {
			case command := <-commands:
				return command
			default:
				t.Fatal("expected the local command to run")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for desktop download")
		}
		return ""
	}

	outputDir := t.TempDir()
	var firstTaskID string
	for _, entry := range entries {
		if entry.Status != "valid" {
			continue
		}
		taskID, err := app.StartDownload(DownloadRequest{
			URL: entry.URL, OutputDir: outputDir, Quality: "f:123",
			Options: &DownloadOptions{FilenameTemplate: "desktop-%(id)s.%(ext)s"},
		})
		if err != nil {
			t.Fatal(err)
		}
		waitForAttempt(taskID)
		if firstTaskID == "" {
			firstTaskID = taskID
		}
	}
	retryID, err := app.RetryDownload(firstTaskID)
	if err != nil {
		t.Fatal(err)
	}
	command := waitForAttempt(retryID)
	if retryID == firstTaskID || !strings.Contains(command, "--format 123") || !strings.Contains(command, "desktop-%(id)s.%(ext)s") {
		t.Fatalf("desktop retry lost its task options: %s", command)
	}
	if len(app.GetDownloads()) != 3 {
		t.Fatalf("expected two original tasks and a retry, got %+v", app.GetDownloads())
	}
}
