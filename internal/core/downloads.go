package core

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
	if s.ytdlpPath == "" && !isDouyinURL(req.URL) {
		return "", fmt.Errorf("yt-dlp not found")
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
	go s.runDownload(taskID, req)
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

func (s *Service) runDownload(taskID string, req DownloadRequest) {
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
	args := qualityArgs(req.Quality)
	args = append(args, "--ignore-config")
	settings := s.GetSettings()
	outputTemplate := "%(title)s.%(ext)s"
	if settings.FilenameTemplate != "" {
		outputTemplate = settings.FilenameTemplate
	}
	args = append(args, "--newline", "--progress", "--print", "after_move:[YT-GO-OUTPUT]%(filepath)s", "-o", filepath.Join(req.OutputDir, outputTemplate), "--no-playlist")
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
	cmd := s.ytdlpCmd(ctx, args...)
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Starting download: %s", req.URL))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] yt-dlp path: %s", s.ytdlpPath))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir))
	var lastOutputFile string
	writer := &lineWriter{handler: func(line string) {
		if m := finalPathRe.FindStringSubmatch(line); m != nil {
			lastOutputFile = strings.TrimSpace(m[1])
			return
		}
		s.emitDownloadLog(taskID, line)
		if m := progressRe.FindStringSubmatch(line); m != nil {
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
		} else if m := destRe1.FindStringSubmatch(line); m != nil {
			lastOutputFile = m[1]
		} else if m := destRe2.FindStringSubmatch(line); m != nil {
			lastOutputFile = strings.Trim(m[1], `"`)
		} else if m := destRe3.FindStringSubmatch(line); m != nil {
			lastOutputFile = m[1]
		}
	}}
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
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
