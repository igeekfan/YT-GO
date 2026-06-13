package core

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"YT-GO/internal/platform"

	"github.com/google/uuid"
	"github.com/lrstanley/go-ytdlp"
)

// isValidDownloadURL checks that the URL uses http or https protocol.
func isValidDownloadURL(rawURL string) bool {
	// Extract URL from possible text wrapping (e.g. pasted text with surrounding content)
	trimmed := strings.TrimSpace(rawURL)
	// Handle URL-encoded variants
	if strings.HasPrefix(trimmed, "file://") || strings.HasPrefix(trimmed, "file:") {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

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
	// Validate URL protocol: only allow http/https.
	if req.URL == "" {
		return "", fmt.Errorf("URL is required")
	}
	if !isValidDownloadURL(req.URL) {
		return "", fmt.Errorf("invalid URL: only http and https protocols are allowed")
	}
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" && !isDouyinURL(req.URL) {
		return "", fmt.Errorf("yt-dlp not found")
	}
	if err := ensureYouTubeJSRuntime(s.i18n, extractURLFromText(req.URL), s.GetSettings()); err != nil {
		return "", err
	}
	// Override output dir with YTGO_DOWNLOAD_DIR if configured (web mode)
	outputDir := req.OutputDir
	if s.downloadDir != "" {
		outputDir = s.downloadDir
		req.OutputDir = outputDir
	}
	taskID := uuid.New().String()
	task := &DownloadTask{ID: taskID, URL: req.URL, OutputDir: outputDir, Quality: req.Quality, Status: "pending", CreatedAt: time.Now().Format(time.RFC3339)}
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
	builder := s.newYtdlpCommand().SetExecutable(ytdlpPath)
	builder.SetSeparateProcessGroup(true)
	builder.SetCancelMaxWait(3 * time.Second)
	applyFormatArgs(builder, req.Quality)
	builder.IgnoreConfig()

	if runtime.GOOS == "windows" {
		builder.WindowsFilenames()
	}

	outputTemplate := "%(title)s.%(ext)s"
	if settings.FilenameTemplate != "" {
		outputTemplate = settings.FilenameTemplate
	}
	// Per-download filename template overrides the global one when provided.
	if req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "" {
		outputTemplate = strings.TrimSpace(req.Options.FilenameTemplate)
	}
	builder.Newline()
	builder.Progress().ProgressDelta(0.5).ProgressTemplate(structuredProgressPrefix + "%()j")
	builder.Print("after_move:[YT-GO-OUTPUT]%(filepath)s")
	builder.Output(filepath.Join(req.OutputDir, outputTemplate))
	builder.NoPlaylist()

	if settings.RateLimit != "" {
		builder.LimitRate(settings.RateLimit)
	}
	if settings.Proxy != "" {
		builder.Proxy(settings.Proxy)
	}
	if settings.MergeOutputFormat != "" && shouldApplyMergeOutputFormat(req.Quality) {
		builder.MergeOutputFormat(settings.MergeOutputFormat)
	}
	if requiresAudioExtraction(req.Quality) {
		audioFmt := settings.AudioFormat
		if audioFmt == "" && req.Quality == "audio" {
			audioFmt = "mp3"
		}
		if audioFmt != "" {
			builder.AudioFormat(audioFmt)
		}
	}

	// Resolve per-download options (override global settings).
	optSaveDescription := settings.SaveDescription
	optSaveThumbnail := settings.SaveThumbnail
	subtitleCfg := resolveSubtitleDownloadConfig(settings, req.Options)
	optEmbedChapters := settings.EmbedChapters
	optSponsorBlock := settings.SponsorBlock
	if req.Options != nil {
		if req.Options.SaveDescription != nil {
			optSaveDescription = *req.Options.SaveDescription
		}
		if req.Options.SaveThumbnail != nil {
			optSaveThumbnail = *req.Options.SaveThumbnail
		}
		if req.Options.EmbedChapters != nil {
			optEmbedChapters = *req.Options.EmbedChapters
		}
		if req.Options.SponsorBlock != nil {
			optSponsorBlock = *req.Options.SponsorBlock
		}
	}

	if optSaveDescription {
		builder.WriteDescription()
	}
	if optSaveThumbnail {
		builder.WriteThumbnail()
	}
	applySubtitleDownloadConfig(builder, subtitleCfg, req.URL)
	if optEmbedChapters {
		builder.EmbedChapters()
	}
	if optSponsorBlock {
		builder.SponsorblockMark("all")
	}

	builder.IgnoreErrors()

	applyCookiesArgs(builder, settings)
	s.applyMediaCommand(builder)

	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Starting download: %s", req.URL))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] yt-dlp path: %s", ytdlpPath))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir))

	// Use BuildCommand to get exec.Cmd, then manage execution ourselves
	// for proper cancel support. We use a lineWriter to parse progress
	// from stdout/stderr, just like the old implementation.
	execCmd := builder.BuildCommand(ctx, req.URL)
	platform.ConfigureCmdWindow(execCmd, true)
	s.emitLog("[runDownload] exec: %s", strings.Join(execCmd.Args, " "))

	// Store the command so CancelDownload can kill the process.
	s.mu.Lock()
	s.cmds[taskID] = execCmd
	s.mu.Unlock()

	var lastOutputFile string
	writer := &lineWriter{handler: func(line string) {
		line = sanitizeYTLine(line)
		if line == "" {
			return
		}
		if progressUpdate, ok := parseStructuredProgressLine(line); ok {
			if isSidecarProgressFile(progressUpdate.Filename) {
				return
			}
			s.handleStructuredProgressLog(taskID, progressUpdate)
			return
		}
		if m := finalPathRe.FindStringSubmatch(line); m != nil {
			lastOutputFile = strings.TrimSpace(m[1])
			return
		}
		s.emitDownloadLog(taskID, line)
		m := progressRe.FindStringSubmatch(line)
		if m != nil {
			pct, _ := strconv.ParseFloat(m[1], 64)
			var updated *DownloadTask
			s.mu.Lock()
			if t, ok := s.downloads[taskID]; ok {
				t.Progress = pct
				t.Size = m[2]
				t.Speed = m[3]
				if len(m) > 4 {
					t.ETA = m[4]
				}
				copy := *t
				updated = &copy
			}
			s.mu.Unlock()
			if updated != nil {
				s.emitDownloadUpdate(updated)
			}
		} else if m := progressDoneRe.FindStringSubmatch(line); m != nil {
			pct, _ := strconv.ParseFloat(m[1], 64)
			var updated *DownloadTask
			s.mu.Lock()
			if t, ok := s.downloads[taskID]; ok {
				t.Progress = pct
				t.Size = m[2]
				t.Speed = m[3]
				t.ETA = ""
				copy := *t
				updated = &copy
			}
			s.mu.Unlock()
			if updated != nil {
				s.emitDownloadUpdate(updated)
			}
		} else if m := destRe1.FindStringSubmatch(line); m != nil {
			lastOutputFile = m[1]
		} else if m := destRe2.FindStringSubmatch(line); m != nil {
			lastOutputFile = strings.Trim(m[1], `"`)
		} else if m := destRe3.FindStringSubmatch(line); m != nil {
			lastOutputFile = m[1]
		}
	}}

	execCmd.Stdout = writer
	execCmd.Stderr = writer

	runErr := execCmd.Start()
	if runErr != nil {
		wasCancelled := ctx.Err() != nil
		cancel()
		s.mu.Lock()
		delete(s.cancelFns, taskID)
		delete(s.cmds, taskID)
		if t, ok := s.downloads[taskID]; ok {
			if wasCancelled {
				delete(s.downloads, taskID)
				s.mu.Unlock()
				s.emitDownloadRemove(taskID)
				go s.deleteRecords([]string{taskID})
				return
			}
			t.Status = "error"
			t.Error = runErr.Error()
			copy := *t
			s.mu.Unlock()
			s.emitDownloadUpdate(&copy)
			go s.upsertRecord(&copy)
		} else {
			s.mu.Unlock()
		}
		return
	}

	// Wait for the process to finish.
	runErr = execCmd.Wait()
	// Emit any remaining buffered bytes that lacked a trailing newline.
	writer.Flush()

	wasCancelled := ctx.Err() != nil
	cancel()
	s.mu.Lock()
	delete(s.cancelFns, taskID)
	delete(s.cmds, taskID)
	var finalTask *DownloadTask
	var removed bool
	if t, ok := s.downloads[taskID]; ok {
		switch {
		case wasCancelled:
			delete(s.downloads, taskID)
			removed = true
		case runErr != nil:
			// If the output file was produced, treat as completed despite non-zero exit
			// (e.g. subtitle/danmaku postprocessing errors shouldn't fail the whole download).
			outputReady := false
			if lastOutputFile != "" {
				absPath := lastOutputFile
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(t.OutputDir, absPath)
				}
				if _, statErr := os.Stat(absPath); statErr == nil {
					outputReady = true
				}
			}
			if outputReady {
				t.Status = "completed"
				t.Progress = 100
				t.OutputPath = lastOutputFile
				if !filepath.IsAbs(t.OutputPath) {
					t.OutputPath = filepath.Join(t.OutputDir, t.OutputPath)
				}
				s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Download completed with warnings: %s", runErr.Error()))
			} else {
				t.Status = "error"
				t.Error = runErr.Error()
			}
		default:
			t.Status = "completed"
			t.Progress = 100
			if lastOutputFile != "" {
				// Ensure outputPath is absolute
				if !filepath.IsAbs(lastOutputFile) {
					lastOutputFile = filepath.Join(t.OutputDir, lastOutputFile)
				}
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

// lineWriter buffers bytes into complete lines and calls handler for each.
// Splits on both '\n' and '\r' so yt-dlp progress bars (which can use
// carriage-return overwrites when --newline isn't honored) still stream.
type lineWriter struct {
	buf     []byte
	handler func(string)
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lw.buf = append(lw.buf, p...)
	for {
		idx := bytes.IndexAny(lw.buf, "\r\n")
		if idx < 0 {
			break
		}
		line := strings.TrimRight(toUTF8(lw.buf[:idx]), "\r\n")
		lw.buf = lw.buf[idx+1:]
		if line != "" {
			lw.handler(line)
		}
	}
	return len(p), nil
}

// Flush emits any bytes still in the buffer as a final line. Call after the
// process exits so the last update (which may lack a trailing newline) isn't lost.
func (lw *lineWriter) Flush() {
	if len(lw.buf) == 0 {
		return
	}
	line := strings.TrimRight(toUTF8(lw.buf), "\r\n")
	lw.buf = nil
	if line != "" {
		lw.handler(line)
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

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(bytes)/1024/1024)
	}
	return fmt.Sprintf("%.1fGB", float64(bytes)/1024/1024/1024)
}

func mergeSubtitleLangs(groups ...string) string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, group := range groups {
		for _, lang := range strings.Split(group, ",") {
			lang = strings.TrimSpace(lang)
			if lang == "" {
				continue
			}
			if _, ok := seen[lang]; ok {
				continue
			}
			seen[lang] = struct{}{}
			merged = append(merged, lang)
		}
	}
	return strings.Join(merged, ",")
}

type subtitleDownloadConfig struct {
	WriteSubtitles    bool
	WriteManualSubs   bool
	WriteAutoSubs     bool
	SubtitleLangs     string
	AutoSubtitleLangs string
	EmbedSubtitles    bool
	ExplicitMode      bool
}

func resolveSubtitleDownloadConfig(settings Settings, opts *DownloadOptions) subtitleDownloadConfig {
	cfg := subtitleDownloadConfig{
		WriteSubtitles:    settings.WriteSubtitles,
		WriteManualSubs:   settings.WriteSubtitles,
		WriteAutoSubs:     settings.WriteSubtitles,
		SubtitleLangs:     settings.SubtitleLangs,
		AutoSubtitleLangs: settings.SubtitleLangs,
		EmbedSubtitles:    settings.EmbedSubtitles,
	}
	if opts == nil {
		return cfg
	}
	if opts.WriteSubtitles != nil {
		cfg.WriteSubtitles = *opts.WriteSubtitles
	}
	if opts.WriteManualSubs != nil {
		cfg.WriteManualSubs = *opts.WriteManualSubs
		cfg.ExplicitMode = true
	}
	if opts.WriteAutoSubs != nil {
		cfg.WriteAutoSubs = *opts.WriteAutoSubs
		cfg.ExplicitMode = true
	}
	if opts.SubtitleLangs != "" {
		cfg.SubtitleLangs = opts.SubtitleLangs
		cfg.ExplicitMode = true
	}
	if opts.AutoSubtitleLangs != "" {
		cfg.AutoSubtitleLangs = opts.AutoSubtitleLangs
		cfg.ExplicitMode = true
	}
	if opts.EmbedSubtitles != nil {
		cfg.EmbedSubtitles = *opts.EmbedSubtitles
	}
	if !cfg.WriteSubtitles {
		cfg.WriteManualSubs = false
		cfg.WriteAutoSubs = false
		cfg.SubtitleLangs = ""
		cfg.AutoSubtitleLangs = ""
		cfg.EmbedSubtitles = false
		return cfg
	}
	if cfg.ExplicitMode {
		if !cfg.WriteManualSubs {
			cfg.SubtitleLangs = ""
		}
		if !cfg.WriteAutoSubs {
			cfg.AutoSubtitleLangs = ""
		}
	}
	return cfg
}

func applySubtitleDownloadConfig(builder interface {
	WriteSubs() *ytdlp.Command
	WriteAutoSubs() *ytdlp.Command
	SubLangs(string) *ytdlp.Command
	EmbedSubs() *ytdlp.Command
	Retries(string) *ytdlp.Command
	ExtractorRetries(string) *ytdlp.Command
	FragmentRetries(string) *ytdlp.Command
	RetrySleep(string) *ytdlp.Command
	SleepRequests(float64) *ytdlp.Command
	SleepSubtitles(float64) *ytdlp.Command
}, cfg subtitleDownloadConfig, rawURL string) {
	if !cfg.WriteSubtitles {
		return
	}
	if cfg.WriteManualSubs {
		builder.WriteSubs()
	}
	if cfg.WriteAutoSubs {
		builder.WriteAutoSubs()
		applyYouTubeAutoSubtitleWorkarounds(builder, rawURL)
	}
	langs := cfg.SubtitleLangs
	if cfg.ExplicitMode {
		langs = mergeSubtitleLangs(cfg.SubtitleLangs, cfg.AutoSubtitleLangs)
	}
	if langs != "" {
		builder.SubLangs(langs)
	}
	if cfg.EmbedSubtitles {
		builder.EmbedSubs()
	}
}

func applyYouTubeAutoSubtitleWorkarounds(builder interface {
	Retries(string) *ytdlp.Command
	ExtractorRetries(string) *ytdlp.Command
	FragmentRetries(string) *ytdlp.Command
	RetrySleep(string) *ytdlp.Command
	SleepRequests(float64) *ytdlp.Command
	SleepSubtitles(float64) *ytdlp.Command
}, rawURL string) {
	if !isYouTubeURL(rawURL) {
		return
	}
	builder.Retries("5")
	builder.ExtractorRetries("5")
	builder.FragmentRetries("5")
	builder.RetrySleep("http:linear=1::2")
	builder.RetrySleep("fragment:linear=1::2")
	builder.SleepRequests(0.75)
	builder.SleepSubtitles(1.5)
}

func (s *Service) handleStructuredProgressLog(taskID string, progress structuredProgressUpdate) {
	now := time.Now()
	var updated *DownloadTask
	var logLine string

	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		startedAt, err := time.Parse(time.RFC3339, task.CreatedAt)
		if err != nil {
			startedAt = now
		}

		elapsedSeconds := now.Sub(startedAt).Seconds()
		speedBytesPerSec := 0.0
		if elapsedSeconds > 0 {
			speedBytesPerSec = float64(progress.DownloadedBytes) / elapsedSeconds
		}

		percent := 0.0
		if progress.TotalBytes > 0 {
			percent = float64(progress.DownloadedBytes) / float64(progress.TotalBytes) * 100
		}
		if progress.Status == "finished" && percent < 100 {
			percent = 100
		}

		eta := ""
		if speedBytesPerSec > 0 && progress.TotalBytes > progress.DownloadedBytes {
			remainingSeconds := float64(progress.TotalBytes-progress.DownloadedBytes) / speedBytesPerSec
			eta = formatDuration(time.Duration(remainingSeconds * float64(time.Second)))
		}

		task.Progress = percent
		if progress.TotalBytes > 0 {
			task.Size = formatBytes(progress.TotalBytes)
		} else if progress.DownloadedBytes > 0 {
			task.Size = formatBytes(progress.DownloadedBytes)
		}
		if speedBytesPerSec > 0 {
			task.Speed = formatSpeed(speedBytesPerSec)
		}
		task.ETA = eta

		copy := *task
		updated = &copy

		if progress.TotalBytes > 0 {
			logLine = fmt.Sprintf("[download] %.1f%% of %s at %s", percent, formatBytes(progress.TotalBytes), task.Speed)
		} else {
			logLine = fmt.Sprintf("[download] %s downloaded at %s", formatBytes(progress.DownloadedBytes), task.Speed)
		}
		if eta != "" {
			logLine += fmt.Sprintf(" ETA %s", eta)
		}
		if progress.FragmentCount > 0 {
			logLine += fmt.Sprintf(" (frag %d/%d)", progress.FragmentIndex, progress.FragmentCount)
		}
	}
	s.mu.Unlock()

	if updated != nil {
		s.emitDownloadUpdate(updated)
		if logLine != "" {
			s.emitDownloadLog(taskID, logLine)
		}
	}
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
	s.mu.Lock()
	cancel, ok := s.cancelFns[taskID]
	cmd := s.cmds[taskID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("task not found or not active")
	}
	// Cancel the context — this signals exec.CommandContext to kill the process.
	cancel()
	// Additionally, forcefully kill the process if it's still running.
	// On Windows with CREATE_NEW_PROCESS_GROUP, this ensures the yt-dlp
	// process and its children (ffmpeg, etc.) are terminated.
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
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
