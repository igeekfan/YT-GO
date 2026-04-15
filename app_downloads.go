package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// cleanupTransientDownloads removes stale transient tasks left by a previous session.
func (a *App) cleanupTransientDownloads() {
	if a.db == nil {
		return
	}
	a.db.Where("status IN ?", []string{"pending", "downloading", "cancelled"}).Delete(&DownloadRecord{})
}

// loadFromDB loads all persisted download records into the in-memory map.
func (a *App) loadFromDB() {
	if a.db == nil {
		return
	}
	var records []DownloadRecord
	if err := a.db.Order("created_at desc").Find(&records).Error; err != nil {
		return
	}
	a.mu.Lock()
	for _, record := range records {
		task := recordToTask(record)
		a.downloads[task.ID] = task
	}
	a.mu.Unlock()
}

func (a *App) upsertRecord(t *DownloadTask) {
	if a.db == nil {
		return
	}
	record := taskToRecord(t)
	a.db.Save(&record)
}

func (a *App) deleteRecords(ids []string) {
	if a.db == nil || len(ids) == 0 {
		return
	}
	a.db.Delete(&DownloadRecord{}, ids)
}

// StartDownload enqueues and starts a video download, returns the task ID.
func (a *App) StartDownload(req DownloadRequest) (string, error) {
	if a.ytdlpPath == "" {
		return "", fmt.Errorf("yt-dlp not found")
	}
	taskID := uuid.New().String()
	task := &DownloadTask{
		ID:        taskID,
		URL:       req.URL,
		OutputDir: req.OutputDir,
		Quality:   req.Quality,
		Status:    "pending",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if req.VideoInfo != nil {
		task.Title = req.VideoInfo.Title
		task.Thumbnail = req.VideoInfo.Thumbnail
	}

	a.mu.Lock()
	a.downloads[taskID] = task
	a.mu.Unlock()

	go a.upsertRecord(task)
	wailsRuntime.EventsEmit(a.ctx, "download:update", task)
	go a.runDownload(taskID, req)
	return taskID, nil
}

type lineWriter struct {
	mu      sync.Mutex
	buf     []byte
	handler func(string)
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.buf = append(lw.buf, p...)
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(toUTF8(lw.buf[:idx]), "\r")
		lw.buf = lw.buf[idx+1:]
		if line != "" {
			lw.handler(line)
		}
	}
	return len(p), nil
}

var (
	progressRe  = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+at\s+(\S+)(?:\s+ETA\s+(\S+))?`)
	destRe1     = regexp.MustCompile(`^\[download\] Destination: (.+)$`)
	destRe2     = regexp.MustCompile(`Merging formats into "(.+)"`)
	destRe3     = regexp.MustCompile(`^\[ExtractAudio\] Destination: (.+)$`)
	finalPathRe = regexp.MustCompile(`^\[YT-GO-OUTPUT\](.+)$`)
)

func (a *App) runDownload(taskID string, req DownloadRequest) {
	a.downloadSem <- struct{}{}
	defer func() { <-a.downloadSem }()

	ctx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	a.cancelFns[taskID] = cancel
	a.downloads[taskID].Status = "downloading"
	{
		cp := *a.downloads[taskID]
		a.mu.Unlock()
		wailsRuntime.EventsEmit(a.ctx, "download:update", &cp)
	}

	args := qualityArgs(req.Quality)
	args = append(args, "--ignore-config")

	settings := a.GetSettings()
	outputTemplate := "%(title)s.%(ext)s"
	if settings.FilenameTemplate != "" {
		outputTemplate = settings.FilenameTemplate
	}
	args = append(args,
		"--newline",
		"--progress",
		"--print", "after_move:[YT-GO-OUTPUT]%(filepath)s",
		"-o", filepath.Join(req.OutputDir, outputTemplate),
		"--no-playlist",
	)

	if settings.RateLimit != "" {
		args = append(args, "--rate-limit", settings.RateLimit)
	}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	if settings.MergeOutputFormat != "" {
		args = append(args, "--merge-output-format", settings.MergeOutputFormat)
	}
	if settings.AudioFormat != "" && req.Quality == "audio" {
		args = append(args, "--audio-format", settings.AudioFormat)
	}

	optSaveDescription := settings.SaveDescription
	optSaveThumbnail := settings.SaveThumbnail
	optWriteSubtitles := settings.WriteSubtitles
	optSubtitleLangs := settings.SubtitleLangs
	optEmbedSubtitles := settings.EmbedSubtitles
	optEmbedChapters := settings.EmbedChapters
	optSponsorBlock := settings.SponsorBlock
	if req.Options != nil {
		if req.Options.SaveDescription != nil {
			optSaveDescription = *req.Options.SaveDescription
		}
		if req.Options.SaveThumbnail != nil {
			optSaveThumbnail = *req.Options.SaveThumbnail
		}
		if req.Options.WriteSubtitles != nil {
			optWriteSubtitles = *req.Options.WriteSubtitles
		}
		if req.Options.SubtitleLangs != "" {
			optSubtitleLangs = req.Options.SubtitleLangs
		}
		if req.Options.EmbedSubtitles != nil {
			optEmbedSubtitles = *req.Options.EmbedSubtitles
		}
		if req.Options.EmbedChapters != nil {
			optEmbedChapters = *req.Options.EmbedChapters
		}
		if req.Options.SponsorBlock != nil {
			optSponsorBlock = *req.Options.SponsorBlock
		}
	}

	if optSaveDescription {
		args = append(args, "--write-description")
	}
	if optSaveThumbnail {
		args = append(args, "--write-thumbnail")
	}
	if optWriteSubtitles {
		args = append(args, "--write-subs")
		if optSubtitleLangs != "" {
			args = append(args, "--sub-langs", optSubtitleLangs)
		}
		if optEmbedSubtitles {
			args = append(args, "--embed-subs")
		}
	}
	if optEmbedChapters {
		args = append(args, "--embed-chapters")
	}
	if optSponsorBlock {
		args = append(args, "--sponsorblock-mark", "all")
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, req.URL)

	cmd := a.ytdlpCmd(ctx, args...)
	wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{"taskId": taskID, "line": fmt.Sprintf("[YT-GO] Starting download: %s", req.URL)})
	wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{"taskId": taskID, "line": fmt.Sprintf("[YT-GO] yt-dlp path: %s", a.ytdlpPath)})
	wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{"taskId": taskID, "line": fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir)})

	var lastOutputFile string
	writer := &lineWriter{
		handler: func(line string) {
			if m := finalPathRe.FindStringSubmatch(line); m != nil {
				lastOutputFile = strings.TrimSpace(m[1])
				return
			}

			wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{"taskId": taskID, "line": line})
			if m := progressRe.FindStringSubmatch(line); m != nil {
				pct, _ := strconv.ParseFloat(m[1], 64)
				var cp *DownloadTask
				a.mu.Lock()
				if t, ok := a.downloads[taskID]; ok {
					t.Progress = pct
					t.Size = m[2]
					t.Speed = m[3]
					if len(m) > 4 {
						t.ETA = m[4]
					}
					taskCopy := *t
					cp = &taskCopy
				}
				a.mu.Unlock()
				if cp != nil {
					wailsRuntime.EventsEmit(a.ctx, "download:update", cp)
				}
			} else if m := destRe1.FindStringSubmatch(line); m != nil {
				lastOutputFile = m[1]
			} else if m := destRe2.FindStringSubmatch(line); m != nil {
				lastOutputFile = strings.Trim(m[1], `"`)
			} else if m := destRe3.FindStringSubmatch(line); m != nil {
				lastOutputFile = m[1]
			}
		},
	}

	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	wasCancelled := ctx.Err() != nil
	cancel()

	a.mu.Lock()
	delete(a.cancelFns, taskID)
	var finalTask *DownloadTask
	var removed bool
	if t, ok := a.downloads[taskID]; ok {
		switch {
		case wasCancelled:
			delete(a.downloads, taskID)
			removed = true
		case err != nil:
			t.Status = "error"
			t.Error = err.Error()
		default:
			t.Status = "completed"
			t.Progress = 100
			if lastOutputFile != "" {
				t.OutputPath = lastOutputFile
			}
		}
		cp := *t
		finalTask = &cp
	}
	a.mu.Unlock()

	if removed {
		wailsRuntime.EventsEmit(a.ctx, "download:remove", taskID)
		go a.deleteRecords([]string{taskID})
		return
	}
	if finalTask != nil {
		wailsRuntime.EventsEmit(a.ctx, "download:update", finalTask)
		go a.upsertRecord(finalTask)
	}
}

// CancelDownload cancels an active download by task ID.
func (a *App) CancelDownload(taskID string) error {
	a.mu.RLock()
	cancel, ok := a.cancelFns[taskID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task not found or not active")
	}
	cancel()
	return nil
}

// GetDownloads returns all download tasks.
func (a *App) GetDownloads() []*DownloadTask {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*DownloadTask, 0, len(a.downloads))
	for _, task := range a.downloads {
		copy := *task
		result = append(result, &copy)
	}
	return result
}

// ClearCompleted removes finished/failed/cancelled tasks from the list.
func (a *App) ClearCompleted() {
	a.mu.Lock()
	var ids []string
	for id, task := range a.downloads {
		if task.Status == "completed" || task.Status == "error" || task.Status == "cancelled" {
			ids = append(ids, id)
			delete(a.downloads, id)
		}
	}
	a.mu.Unlock()
	go a.deleteRecords(ids)
}
