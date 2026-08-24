package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func safeDouyinTitle(title string, fallback string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_", "\n", "_", "\r", "_", "\t", "_", "#", "_", "@", "_")
	title = strings.TrimSpace(replacer.Replace(title))
	title = regexp.MustCompile(`_+`).ReplaceAllString(title, "_")
	title = strings.Trim(title, "_. ")
	if title == "" {
		return fallback
	}
	runes := []rune(title)
	if len(runes) > 60 {
		return string(runes[:60])
	}
	return title
}

func (s *Service) runDouyinDownload(taskID string, req DownloadRequest, ctx context.Context) (downloadResult, error) {
	settings := s.GetSettings()
	itemInfo, videoID, _, err := s.resolveDouyinItemInfo(req.URL)
	if err != nil {
		return downloadResult{}, s.translateDouyinError(err)
	}
	if req.Quality == "audio" {
		return downloadResult{}, errors.New(s.i18n.T("douyin.audio_not_supported"))
	}
	videoURL, err := douyinVideoURL(itemInfo)
	if err != nil {
		return downloadResult{}, s.translateDouyinError(err)
	}
	title := douyinString(itemInfo, "desc")
	if title == "" {
		title = "douyin_" + videoID
	}
	safeTitle := safeDouyinTitle(title, "douyin_"+videoID)
	outputPath := filepath.Join(req.OutputDir, safeTitle+".mp4")
	tempPath := outputPath + ".part"
	client, err := s.newDouyinHTTPClient(settings, 0)
	if err != nil {
		return downloadResult{}, err
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return downloadResult{}, err
	}
	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return downloadResult{}, err
	}
	applyHeaders(reqHTTP, douyinDefaultHeaders)
	applyDouyinCookies(reqHTTP, settings)
	resp, err := client.Do(reqHTTP)
	if err != nil {
		if ctx.Err() != nil {
			return downloadResult{}, context.Canceled
		}
		return downloadResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return downloadResult{}, errors.New(fmt.Sprintf(s.i18n.T("douyin.download_failed"), resp.StatusCode))
	}
	file, err := os.Create(tempPath)
	if err != nil {
		return downloadResult{}, err
	}
	defer file.Close()
	var total int64 = -1
	if resp.ContentLength > 0 {
		total = resp.ContentLength
	}
	buffer := make([]byte, 64*1024)
	var written int64
	start := time.Now()
	lastUpdate := time.Time{}
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := file.Write(buffer[:n]); err != nil {
				_ = os.Remove(tempPath)
				return downloadResult{}, err
			}
			written += int64(n)
			if total > 0 && time.Since(lastUpdate) >= 200*time.Millisecond {
				pct := float64(written) / float64(total) * 100
				s.updateDouyinTaskProgress(taskID, pct, written, total, start)
				lastUpdate = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = os.Remove(tempPath)
			if ctx.Err() != nil {
				return downloadResult{}, context.Canceled
			}
			return downloadResult{}, readErr
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return downloadResult{}, err
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		return downloadResult{}, err
	}
	if shouldSaveThumbnail(settings, req.Options) {
		if thumbURL := douyinCoverURL(itemInfo); thumbURL != "" {
			thumbPath := filepath.Join(req.OutputDir, safeTitle+".jpg")
			_ = downloadDouyinSidecar(client, thumbURL, thumbPath)
		}
	}
	if shouldSaveDescription(settings, req.Options) {
		descriptionPath := filepath.Join(req.OutputDir, safeTitle+".description")
		_ = os.WriteFile(descriptionPath, []byte(title), 0o644)
	}
	s.emitDownloadLog(taskID, fmt.Sprintf(s.i18n.T("douyin.download_complete"), outputPath))
	return downloadResult{outputPath: outputPath, size: formatDouyinBytes(written)}, nil
}

func (s *Service) updateDouyinTaskProgress(taskID string, pct float64, written int64, total int64, start time.Time) {
	elapsed := time.Since(start).Seconds()
	speedBytes := float64(0)
	if elapsed > 0 {
		speedBytes = float64(written) / elapsed
	}
	eta := ""
	if speedBytes > 0 && total > written {
		etaSeconds := int(float64(total-written) / speedBytes)
		eta = formatDouyinETA(etaSeconds)
	}
	var updated *DownloadTask
	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		task.Progress = pct
		task.Size = formatDouyinBytes(total)
		task.Speed = formatDouyinBytes(int64(speedBytes)) + "/s"
		task.ETA = eta
		copy := *task
		updated = &copy
	}
	s.mu.Unlock()
	if updated != nil {
		s.emitActiveDownloadUpdate(taskID, updated)
	}
}

func shouldSaveThumbnail(settings Settings, options *DownloadOptions) bool {
	result := settings.SaveThumbnail
	if options != nil && options.SaveThumbnail != nil {
		result = *options.SaveThumbnail
	}
	return result
}

func shouldSaveDescription(settings Settings, options *DownloadOptions) bool {
	result := settings.SaveDescription
	if options != nil && options.SaveDescription != nil {
		result = *options.SaveDescription
	}
	return result
}

func downloadDouyinSidecar(client *http.Client, sourceURL string, filePath string) error {
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	applyHeaders(req, douyinDefaultHeaders)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("douyin.status_code:%d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, body, 0o644)
}

func formatDouyinBytes(value int64) string {
	if value <= 0 {
		return ""
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(value)/1024)
	}
	if value < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(value)/1024/1024)
	}
	return fmt.Sprintf("%.1f GB", float64(value)/1024/1024/1024)
}

func formatDouyinETA(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	minutes := seconds / 60
	remain := seconds % 60
	if minutes > 0 {
		return fmt.Sprintf("%d:%02d", minutes, remain)
	}
	return fmt.Sprintf("0:%02d", remain)
}
