package core

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) runWechatChannelsDownload(taskID string, req DownloadRequest, ctx context.Context) (downloadResult, error) {
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Starting WeChat Channels download: %s", req.URL))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir))

	resolved, err := s.resolveWechatChannels(ctx, req.URL)
	if err != nil {
		return downloadResult{}, err
	}

	formatID := resolveWechatChannelsFormatID(req.Quality)
	videoURL := resolved.VideoURLs[formatID]
	if videoURL == "" {
		formatID = wechatChannelsDefaultFormat
		videoURL = resolved.VideoURLs[formatID]
	}
	if videoURL == "" {
		return downloadResult{}, fmt.Errorf("no downloadable WeChat Channels video URL found")
	}
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] WeChat Channels format: %s", formatID))

	settings := s.GetSettings()
	outputPath, err := buildWechatChannelsOutputPath(req, settings, resolved.Info, videoURL)
	if err != nil {
		return downloadResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return downloadResult{}, err
	}
	outputPath = ensureUniqueWechatFilePath(outputPath)
	partPath := outputPath + ".part"

	if err := s.downloadWechatChannelsFile(ctx, taskID, videoURL, partPath, outputPath, settings); err != nil {
		if ctx.Err() != nil {
			_ = os.Remove(partPath)
			return downloadResult{}, context.Canceled
		}
		_ = os.Remove(partPath)
		return downloadResult{}, err
	}

	s.saveWechatChannelsSidecars(ctx, taskID, outputPath, resolved.Info, settings, req.Options)
	s.emitDownloadLog(taskID, "[YT-GO] WeChat Channels download completed")
	return downloadResult{outputPath: outputPath}, nil
}

func (s *Service) downloadWechatChannelsFile(ctx context.Context, taskID, videoURL, partPath, outputPath string, settings Settings) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", wechatDesktopUserAgent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://channels.weixin.qq.com/")

	resp, err := s.newWechatHTTPClient(settings).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("video download failed with HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(partPath)
	if err != nil {
		return err
	}
	defer file.Close()

	total := resp.ContentLength
	buf := make([]byte, 64*1024)
	var downloaded int64
	start := time.Now()
	lastEmit := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
			if time.Since(lastEmit) >= 500*time.Millisecond {
				s.updateWechatChannelsProgress(taskID, downloaded, total, start)
				lastEmit = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(partPath, outputPath); err != nil {
		return err
	}
	s.updateWechatChannelsProgress(taskID, downloaded, downloaded, start)
	return nil
}

func (s *Service) saveWechatChannelsSidecars(ctx context.Context, taskID, outputPath string, info VideoInfo, settings Settings, options *DownloadOptions) {
	if shouldSaveThumbnail(settings, options) && strings.TrimSpace(info.Thumbnail) != "" {
		thumbnailPath, err := s.downloadWechatChannelsThumbnail(ctx, info.Thumbnail, outputPath, settings)
		if err != nil {
			s.emitDownloadLog(taskID, fmt.Sprintf(s.i18n.T("wechat.thumbnail_failed"), err.Error()))
		} else {
			s.emitDownloadLog(taskID, fmt.Sprintf(s.i18n.T("wechat.thumbnail_saved"), thumbnailPath))
		}
	}
	if shouldSaveDescription(settings, options) {
		descriptionPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".description"
		if err := os.WriteFile(descriptionPath, []byte(info.Title), 0o644); err != nil {
			s.emitDownloadLog(taskID, fmt.Sprintf(s.i18n.T("wechat.description_failed"), err.Error()))
		} else {
			s.emitDownloadLog(taskID, fmt.Sprintf(s.i18n.T("wechat.description_saved"), descriptionPath))
		}
	}
}

func (s *Service) downloadWechatChannelsThumbnail(ctx context.Context, thumbnailURL, outputPath string, settings Settings) (string, error) {
	thumbnailURL = strings.TrimSpace(thumbnailURL)
	if strings.HasPrefix(thumbnailURL, "//") {
		thumbnailURL = "https:" + thumbnailURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, thumbnailURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", wechatDesktopUserAgent())
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Referer", "https://channels.weixin.qq.com/")

	resp, err := s.newWechatHTTPClient(settings).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("thumbnail download failed with HTTP %d", resp.StatusCode)
	}

	ext := wechatThumbnailExtension(resp.Header.Get("Content-Type"), thumbnailURL)
	thumbnailPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "." + ext
	partPath := thumbnailPath + ".part"
	file, err := os.Create(partPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(partPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}
	if err := os.Rename(partPath, thumbnailPath); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}
	return thumbnailPath, nil
}

func wechatThumbnailExtension(contentType, rawURL string) string {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch mediaType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), ".")
		switch ext {
		case "jpg", "jpeg":
			return "jpg"
		case "png", "webp":
			return ext
		}
	}
	return "jpg"
}

func (s *Service) updateWechatChannelsProgress(taskID string, downloaded, total int64, start time.Time) {
	elapsed := time.Since(start).Seconds()
	speedBytes := 0.0
	if elapsed > 0 {
		speedBytes = float64(downloaded) / elapsed
	}
	progress := 0.0
	if total > 0 {
		progress = float64(downloaded) / float64(total) * 100
	}
	eta := ""
	if total > downloaded && speedBytes > 0 {
		eta = formatDuration(time.Duration(float64(total-downloaded) / speedBytes * float64(time.Second)))
	}

	var updated *DownloadTask
	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		task.Progress = progress
		if total > 0 {
			task.Size = formatBytes(total)
		} else if downloaded > 0 {
			task.Size = formatBytes(downloaded)
		}
		if speedBytes > 0 {
			task.Speed = formatSpeed(speedBytes)
		}
		task.ETA = eta
		copy := *task
		updated = &copy
	}
	s.mu.Unlock()
	if updated != nil {
		s.emitActiveDownloadUpdate(taskID, updated)
	}
}
