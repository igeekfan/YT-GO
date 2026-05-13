package core

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lrstanley/go-ytdlp"
)

func (s *Service) cleanupTransientDownloads() {
	if s.db == nil {
		return
	}
	s.db.Where("status IN ?", []string{"pending", "downloading", "cancelled"}).Delete(&DownloadRecord{})
}

func (s *Service) loadFromDB() {
	if s.db == nil {
		return
	}
	var records []DownloadRecord
	if err := s.db.Order("created_at desc").Find(&records).Error; err != nil {
		return
	}
	s.mu.Lock()
	for _, record := range records {
		task := recordToTask(record)
		s.downloads[task.ID] = task
	}
	s.mu.Unlock()
}

func (s *Service) upsertRecord(task *DownloadTask) {
	if s.db == nil {
		return
	}
	record := taskToRecord(task)
	s.db.Save(&record)
}

func (s *Service) deleteRecords(ids []string) {
	if s.db == nil || len(ids) == 0 {
		return
	}
	s.db.Delete(&DownloadRecord{}, ids)
}

func (s *Service) StartDownload(req DownloadRequest) (string, error) {
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" && !isDouyinURL(req.URL) {
		return "", fmt.Errorf("yt-dlp not found")
	}
	if err := ensureYouTubeJSRuntime(s.i18n, extractURLFromText(req.URL), s.GetSettings()); err != nil {
		return "", err
	}
	taskID := uuid.New().String()
	task := &DownloadTask{ID: taskID, URL: req.URL, OutputDir: req.OutputDir, Quality: req.Quality, Status: "pending", CreatedAt: time.Now().Format(time.RFC3339)}
	if req.VideoInfo != nil {
		task.Title = req.VideoInfo.Title
		task.Thumbnail = req.VideoInfo.Thumbnail
	}
	s.mu.Lock()
	s.downloads[taskID] = task
	s.mu.Unlock()
	go s.upsertRecord(task)
	s.emitDownloadUpdate(task)
	go s.runDownload(taskID, req, ytdlpPath)
	return taskID, nil
}

func (s *Service) runDownload(taskID string, req DownloadRequest, ytdlpPath string) {
	s.downloadSem <- struct{}{}
	defer func() { <-s.downloadSem }()
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelFns[taskID] = cancel
	s.downloads[taskID].Status = "downloading"
	cp := *s.downloads[taskID]
	s.mu.Unlock()
	s.emitDownloadUpdate(&cp)
	if isDouyinURL(req.URL) {
		s.runDouyinDownload(taskID, req, ctx)
		cancel()
		return
	}

	settings := s.GetSettings()

	// Build the download command using go-ytdlp builder.
	cmd := s.newYtdlpCommand().SetExecutable(ytdlpPath)
	applyFormatArgs(cmd, req.Quality)
	cmd.IgnoreConfig()

	if runtime.GOOS == "windows" {
		cmd.WindowsFilenames()
	}

	outputTemplate := "%(title)s.%(ext)s"
	if settings.FilenameTemplate != "" {
		outputTemplate = settings.FilenameTemplate
	}
	cmd.Newline()
	cmd.Print("after_move:[YT-GO-OUTPUT]%(filepath)s")
	cmd.Output(filepath.Join(req.OutputDir, outputTemplate))
	cmd.NoPlaylist()

	if settings.RateLimit != "" {
		cmd.LimitRate(settings.RateLimit)
	}
	if settings.Proxy != "" {
		cmd.Proxy(settings.Proxy)
	}
	if settings.MergeOutputFormat != "" && shouldApplyMergeOutputFormat(req.Quality) {
		cmd.MergeOutputFormat(settings.MergeOutputFormat)
	}
	if requiresAudioExtraction(req.Quality) {
		audioFmt := settings.AudioFormat
		if audioFmt == "" && req.Quality == "audio" {
			audioFmt = "mp3"
		}
		if audioFmt != "" {
			cmd.AudioFormat(audioFmt)
		}
	}

	// Resolve per-download options (override global settings).
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
		cmd.WriteDescription()
	}
	if optSaveThumbnail {
		cmd.WriteThumbnail()
	}
	if optWriteSubtitles {
		cmd.WriteSubs()
		if optSubtitleLangs != "" {
			cmd.SubLangs(optSubtitleLangs)
		}
		if optEmbedSubtitles {
			cmd.EmbedSubs()
		}
	}
	if optEmbedChapters {
		cmd.EmbedChapters()
	}
	if optSponsorBlock {
		cmd.SponsorblockMark("all")
	}

	applyCookiesArgs(cmd, settings)
	s.applyMediaCommand(cmd)

	// Set up progress callback.
	var lastOutputFile string
	cmd.ProgressFunc(200*time.Millisecond, func(update ytdlp.ProgressUpdate) {
		var updated *DownloadTask
		s.mu.Lock()
		if t, ok := s.downloads[taskID]; ok {
			t.Progress = update.Percent()
			if update.TotalBytes > 0 {
				t.Size = fmt.Sprintf("%.1fMB", float64(update.DownloadedBytes)/1024/1024)
			}
			if update.Filename != "" {
				lastOutputFile = update.Filename
			}
			// Calculate speed and ETA from duration.
			elapsed := update.Duration()
			if elapsed > 0 && update.DownloadedBytes > 0 {
				speed := float64(update.DownloadedBytes) / elapsed.Seconds()
				t.Speed = formatSpeed(speed)
				if update.TotalBytes > 0 {
					remaining := time.Duration(float64(update.TotalBytes-update.DownloadedBytes)/speed) * time.Second
					t.ETA = formatDuration(remaining)
				}
			}
			copy := *t
			updated = &copy
		}
		s.mu.Unlock()
		if updated != nil {
			s.emitDownloadUpdate(updated)
		}
	})

	// Set up stderr callback for logging.
	cmd.StderrFunc(func(line string) {
		// Check for output path markers from our --print after_move directive.
		if m := finalPathRe.FindStringSubmatch(line); m != nil {
			lastOutputFile = strings.TrimSpace(m[1])
			return
		}
		s.emitDownloadLog(taskID, line)
	})

	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Starting download: %s", req.URL))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] yt-dlp path: %s", ytdlpPath))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir))

	_, err := cmd.Run(ctx, req.URL)
	wasCancelled := ctx.Err() != nil
	cancel()
	s.mu.Lock()
	delete(s.cancelFns, taskID)
	var finalTask *DownloadTask
	var removed bool
	if t, ok := s.downloads[taskID]; ok {
		switch {
		case wasCancelled:
			delete(s.downloads, taskID)
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
		copy := *t
		finalTask = &copy
	}
	s.mu.Unlock()
	if removed {
		s.emitDownloadRemove(taskID)
		go s.deleteRecords([]string{taskID})
		return
	}
	if finalTask != nil {
		s.emitDownloadUpdate(finalTask)
		go s.upsertRecord(finalTask)
	}
}

// formatSpeed formats bytes per second as a human-readable string.
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0fB/s", bytesPerSec)
	}
	if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1fKB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.1fMB/s", bytesPerSec/1024/1024)
}

// formatDuration formats a duration for display (e.g. "1:23:45").
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (s *Service) CancelDownload(taskID string) error {
	s.mu.RLock()
	cancel, ok := s.cancelFns[taskID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task not found or not active")
	}
	cancel()
	return nil
}

func (s *Service) RemoveDownload(taskID string) error {
	s.mu.Lock()
	task, ok := s.downloads[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task not found")
	}
	if task.Status == "pending" || task.Status == "downloading" {
		s.mu.Unlock()
		return fmt.Errorf("active task cannot be removed")
	}
	delete(s.downloads, taskID)
	s.mu.Unlock()

	s.emitDownloadRemove(taskID)
	go s.deleteRecords([]string{taskID})
	return nil
}

func (s *Service) GetDownloads() []*DownloadTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*DownloadTask, 0, len(s.downloads))
	for _, task := range s.downloads {
		copy := *task
		result = append(result, &copy)
	}
	return result
}

func (s *Service) ClearCompleted() {
	s.mu.Lock()
	var ids []string
	for id, task := range s.downloads {
		if task.Status == "completed" || task.Status == "error" || task.Status == "cancelled" {
			ids = append(ids, id)
			delete(s.downloads, id)
		}
	}
	s.mu.Unlock()
	go s.deleteRecords(ids)
}
