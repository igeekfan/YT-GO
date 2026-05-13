package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func (s *Service) GetVideoInfo(rawInput string) (VideoInfo, error) {
	videoURL := extractURLFromText(rawInput)
	if isDouyinURL(videoURL) {
		return s.GetDouyinVideoInfo(videoURL)
	}
	if s.ytdlpPath == "" {
		return VideoInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(s.i18n, videoURL, settings); err != nil {
		s.emitLog("[GetVideoInfo] preflight failed: %v", err)
		return VideoInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"--ignore-config", "--dump-json", "--no-playlist", "--no-warnings"}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, videoURL)
	cmd := s.ytdlpMediaCmd(ctx, args...)
	s.emitLog("[GetVideoInfo] fetching info for URL: %s", videoURL)
	s.logCmd("GetVideoInfo", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(s.i18n, toUTF8(out), settings)
		s.emitLog("[GetVideoInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return VideoInfo{}, fmt.Errorf("%s", errMsg)
		}
		return VideoInfo{}, fmt.Errorf("failed to get video info: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		s.emitLog("[GetVideoInfo] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return VideoInfo{}, fmt.Errorf("failed to parse video info: %w", err)
	}
	info := VideoInfo{URL: videoURL}
	if v, ok := raw["title"].(string); ok {
		info.Title = v
	}
	if v, ok := raw["id"].(string); ok {
		info.ID = v
	}
	info.Thumbnail = extractThumbnailURL(raw)
	if v, ok := raw["duration"].(float64); ok {
		info.Duration = v
	}
	if v, ok := raw["uploader"].(string); ok {
		info.Uploader = v
	} else if v, ok := raw["channel"].(string); ok {
		info.Uploader = v
	}
	if v, ok := raw["extractor_key"].(string); ok {
		info.Platform = v
	} else if v, ok := raw["extractor"].(string); ok {
		info.Platform = v
	}
	if v, ok := raw["webpage_url"].(string); ok && v != "" {
		info.URL = v
	}
	if fallback := s.resolveVideoThumbnailFallback(info, raw); fallback != "" {
		info.Thumbnail = fallback
	}
	info.Subtitles = extractSubtitleLangs(raw)
	return info, nil
}

func extractSubtitleLangs(raw map[string]interface{}) []SubtitleLang {
	var result []SubtitleLang
	seen := make(map[string]bool)
	if subs, ok := raw["subtitles"].(map[string]interface{}); ok {
		for code := range subs {
			if seen[code] {
				continue
			}
			seen[code] = true
			name := code
			if arr, ok := subs[code].([]interface{}); ok && len(arr) > 0 {
				if obj, ok := arr[0].(map[string]interface{}); ok {
					if nameValue, ok := obj["name"].(string); ok && nameValue != "" {
						name = nameValue
					}
				}
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: false})
		}
	}
	if autoCaptions, ok := raw["automatic_captions"].(map[string]interface{}); ok {
		for code := range autoCaptions {
			if seen[code] {
				continue
			}
			seen[code] = true
			name := code
			if arr, ok := autoCaptions[code].([]interface{}); ok && len(arr) > 0 {
				if obj, ok := arr[0].(map[string]interface{}); ok {
					if nameValue, ok := obj["name"].(string); ok && nameValue != "" {
						name = nameValue
					}
				}
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: true})
		}
	}
	return result
}

func detectCollectionKind(url string) string {
	lower := strings.ToLower(url)
	if strings.Contains(lower, "/@") || strings.Contains(lower, "/channel/") || strings.Contains(lower, "/user/") || strings.Contains(lower, "/c/") {
		return "channel"
	}
	if strings.Contains(lower, "bilibili.com") && (strings.Contains(lower, "/favlist") || strings.Contains(lower, "/medialist") || strings.Contains(lower, "/channel/seriesdetail")) {
		return "channel"
	}
	return "playlist"
}

func (s *Service) GetPlaylistInfo(rawInput string) (PlaylistInfo, error) {
	videoURL := extractURLFromText(rawInput)
	if isDouyinURL(videoURL) {
		info, err := s.GetDouyinVideoInfo(videoURL)
		if err == nil {
			return PlaylistInfo{
				URL:    videoURL,
				Kind:   "playlist",
				Title:  info.Title,
				Count:  1,
				Videos: []VideoInfo{info},
			}, nil
		}
		s.emitLog("[GetPlaylistInfo] Douyin custom handler failed (%v), falling back to yt-dlp", err)
	}
	if s.ytdlpPath == "" {
		return PlaylistInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(s.i18n, videoURL, settings); err != nil {
		s.emitLog("[GetPlaylistInfo] preflight failed: %v", err)
		return PlaylistInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	args := []string{"--ignore-config", "--flat-playlist", "--dump-single-json", "--no-warnings"}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, videoURL)
	cmd := s.ytdlpMediaCmd(ctx, args...)
	s.emitLog("[GetPlaylistInfo] fetching playlist for URL: %s", videoURL)
	s.logCmd("GetPlaylistInfo", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(s.i18n, toUTF8(out), settings)
		s.emitLog("[GetPlaylistInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return PlaylistInfo{}, fmt.Errorf("%s", errMsg)
		}
		return PlaylistInfo{}, fmt.Errorf("failed to get playlist info: %w", err)
	}
	result := PlaylistInfo{URL: videoURL, Kind: detectCollectionKind(videoURL)}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		s.emitLog("[GetPlaylistInfo] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return PlaylistInfo{}, fmt.Errorf("failed to parse playlist info: %w", err)
	}
	if v, ok := raw["title"].(string); ok {
		result.Title = v
	}
	if v, ok := raw["playlist_title"].(string); ok && v != "" {
		result.Title = v
	}
	if v, ok := raw["uploader"].(string); ok {
		result.Uploader = v
	} else if v, ok := raw["playlist_uploader"].(string); ok {
		result.Uploader = v
	} else if v, ok := raw["channel"].(string); ok {
		result.Uploader = v
	}
	if result.Kind == "playlist" {
		if extractor, ok := raw["extractor_key"].(string); ok && strings.Contains(strings.ToLower(extractor), "tab") {
			lowerTitle := strings.ToLower(result.Title)
			if strings.Contains(lowerTitle, "channel") || strings.Contains(lowerTitle, "videos") {
				result.Kind = "channel"
			}
		}
	}
	if entries, ok := raw["entries"].([]interface{}); ok {
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			info := VideoInfo{}
			if v, ok := entryMap["webpage_url"].(string); ok {
				info.URL = v
			} else if v, ok := entryMap["url"].(string); ok {
				info.URL = v
			}
			if v, ok := entryMap["title"].(string); ok {
				info.Title = v
			}
			if v, ok := entryMap["id"].(string); ok {
				info.ID = v
			}
			info.Thumbnail = extractThumbnailURL(entryMap)
			if v, ok := entryMap["duration"].(float64); ok {
				info.Duration = v
			}
			if v, ok := entryMap["uploader"].(string); ok {
				info.Uploader = v
			} else if v, ok := entryMap["channel"].(string); ok {
				info.Uploader = v
			}
			if info.URL != "" || info.ID != "" {
				result.Videos = append(result.Videos, info)
			}
		}
	}
	result.Count = len(result.Videos)
	return result, nil
}

func (s *Service) GetFormats(rawInput string) (FormatInfo, error) {
	videoURL := extractURLFromText(rawInput)
	if isDouyinURL(videoURL) {
		return s.GetDouyinFormats(videoURL)
	}
	if s.ytdlpPath == "" {
		return FormatInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(s.i18n, videoURL, settings); err != nil {
		s.emitLog("[GetFormats] preflight failed: %v", err)
		return FormatInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"--ignore-config", "--dump-json", "--no-download", "--no-warnings", "--no-playlist"}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, videoURL)
	cmd := s.ytdlpMediaCmd(ctx, args...)
	s.emitLog("[GetFormats] fetching formats for URL: %s", videoURL)
	s.logCmd("GetFormats", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(s.i18n, toUTF8(out), settings)
		s.emitLog("[GetFormats] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return FormatInfo{}, fmt.Errorf("%s", errMsg)
		}
		return FormatInfo{}, fmt.Errorf("failed to get formats: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		s.emitLog("[GetFormats] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return FormatInfo{}, fmt.Errorf("failed to parse JSON: %w", err)
	}
	result := FormatInfo{URL: videoURL}
	if v, ok := raw["title"].(string); ok {
		result.Title = v
	}
	if formatsRaw, ok := raw["formats"].([]interface{}); ok {
		for _, rawFormat := range formatsRaw {
			formatMap, ok := rawFormat.(map[string]interface{})
			if !ok {
				continue
			}
			format := Format{}
			if v, ok := formatMap["format_id"].(string); ok {
				format.FormatID = v
			}
			if v, ok := formatMap["ext"].(string); ok {
				format.Ext = v
			}
			if v, ok := formatMap["resolution"].(string); ok {
				format.Resolution = v
			}
			if v, ok := formatMap["fps"].(float64); ok {
				format.FPS = v
			}
			if v, ok := formatMap["vcodec"].(string); ok {
				format.VCodec = v
				format.HasVideo = v != "none" && v != ""
			}
			if v, ok := formatMap["acodec"].(string); ok {
				format.ACodec = v
				format.HasAudio = v != "none" && v != ""
			}
			if v, ok := formatMap["filesize"].(float64); ok {
				format.Filesize = int64(v)
			} else if v, ok := formatMap["filesize_approx"].(float64); ok {
				format.Filesize = int64(v)
			}
			if v, ok := formatMap["tbr"].(float64); ok {
				format.TBR = v
			}
			if v, ok := formatMap["format_note"].(string); ok {
				format.Note = v
			}
			result.Formats = append(result.Formats, format)
		}
	}
	return result, nil
}

// --- Thumbnail helpers ---

func extractThumbnailURL(raw map[string]interface{}) string {
	for _, key := range []string{"thumbnail", "pic", "cover"} {
		if value, ok := raw[key].(string); ok {
			if normalized := normalizeThumbnailURL(value); normalized != "" {
				if isPlaceholderThumbnailURL(normalized) {
					continue
				}
				return normalized
			}
		}
	}
	thumbnails, ok := raw["thumbnails"].([]interface{})
	if !ok {
		return ""
	}
	for index := len(thumbnails) - 1; index >= 0; index-- {
		thumbMap, ok := thumbnails[index].(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range []string{"url", "src"} {
			if value, ok := thumbMap[key].(string); ok {
				if normalized := normalizeThumbnailURL(value); normalized != "" {
					if isPlaceholderThumbnailURL(normalized) {
						continue
					}
					return normalized
				}
			}
		}
	}
	return ""
}

func normalizeThumbnailURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	return value
}

func isPlaceholderThumbnailURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return lower == "" || strings.Contains(lower, "/transparent.png")
}

// --- Bilibili thumbnail fallback ---

func (s *Service) resolveVideoThumbnailFallback(info VideoInfo, raw map[string]interface{}) string {
	if info.Thumbnail != "" && !isPlaceholderThumbnailURL(info.Thumbnail) {
		return ""
	}
	if !isBilibiliVideo(info, raw) {
		return ""
	}
	bvid := extractBilibiliBVID(info, raw)
	if bvid == "" {
		return ""
	}
	thumbnail, err := s.fetchBilibiliThumbnail(bvid)
	if err != nil {
		s.emitLog("[GetVideoInfo] bilibili thumbnail fallback failed: %v", err)
		return ""
	}
	return thumbnail
}

func isBilibiliVideo(info VideoInfo, raw map[string]interface{}) bool {
	platform := strings.ToLower(info.Platform)
	videoURL := strings.ToLower(info.URL)
	if strings.Contains(platform, "bilibili") || strings.Contains(videoURL, "bilibili.com") {
		return true
	}
	if extractor, ok := raw["extractor_key"].(string); ok && strings.Contains(strings.ToLower(extractor), "bilibili") {
		return true
	}
	return false
}

func extractBilibiliBVID(info VideoInfo, raw map[string]interface{}) string {
	if id := strings.TrimSpace(info.ID); strings.HasPrefix(strings.ToUpper(id), "BV") {
		return id
	}
	for _, key := range []string{"bvid", "display_id"} {
		if value, ok := raw[key].(string); ok {
			value = strings.TrimSpace(value)
			if strings.HasPrefix(strings.ToUpper(value), "BV") {
				return value
			}
		}
	}
	match := regexp.MustCompile(`(?i)BV[0-9A-Za-z]+`).FindString(info.URL)
	return strings.TrimSpace(match)
}

func (s *Service) fetchBilibiliThumbnail(bvid string) (string, error) {
	settings := s.GetSettings()
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if settings.Proxy != "" {
		proxyURL, err := url.Parse(settings.Proxy)
		if err != nil {
			return "", fmt.Errorf("invalid proxy for bilibili thumbnail: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	endpoint := "https://api.bilibili.com/x/web-interface/view?bvid=" + url.QueryEscape(bvid)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/video/"+bvid)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Pic string `json:"pic"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	thumbnail := normalizeThumbnailURL(payload.Data.Pic)
	if payload.Code != 0 || thumbnail == "" || isPlaceholderThumbnailURL(thumbnail) {
		return "", fmt.Errorf("empty thumbnail in bilibili api")
	}
	return thumbnail, nil
}
