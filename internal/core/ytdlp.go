package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lrstanley/go-ytdlp"
)

func (s *Service) GetVideoInfo(rawInput string) (VideoInfo, error) {
	videoURL := extractURLFromText(rawInput)
	if isDouyinURL(videoURL) {
		return s.GetDouyinVideoInfo(videoURL)
	}
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" {
		return VideoInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(s.i18n, videoURL, settings); err != nil {
		s.emitLog("[GetVideoInfo] preflight failed: %v", err)
		return VideoInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := s.newYtdlpCommand().SetExecutable(ytdlpPath).
		IgnoreConfig().DumpJSON().NoPlaylist().NoWarnings()
	if settings.Proxy != "" {
		cmd.Proxy(settings.Proxy)
	}
	applyCookiesArgs(cmd, settings)
	s.applyMediaCommand(cmd)

	s.emitLog("[GetVideoInfo] fetching info for URL: %s", videoURL)
	s.logCmd("GetVideoInfo", cmd, ctx, videoURL)

	result, err := cmd.Run(ctx, videoURL)
	if err != nil {
		errMsg := normalizeYtDlpError(s.i18n, toUTF8([]byte(result.Stderr)), settings)
		s.emitLog("[GetVideoInfo] failed: err=%s, output=%s", toUTF8([]byte(err.Error())), errMsg)
		if errMsg != "" {
			return VideoInfo{}, fmt.Errorf("%s", errMsg)
		}
		return VideoInfo{}, fmt.Errorf("failed to get video info: %w", err)
	}

	infoList, parseErr := result.GetExtractedInfo()
	if parseErr != nil || len(infoList) == 0 {
		s.emitLog("[GetVideoInfo] JSON parse failed: %v", parseErr)
		return VideoInfo{}, fmt.Errorf("failed to parse video info: %w", parseErr)
	}

	raw := infoList[0]
	info := extractVideoInfoFromExtracted(raw, videoURL)
	if isYouTubeURL(videoURL) {
		info.Subtitles = s.mergeListedSubtitles(ctx, ytdlpPath, videoURL, settings, info.Subtitles)
	}
	if fallback := s.resolveVideoThumbnailFallback(info, raw); fallback != "" {
		info.Thumbnail = fallback
	}
	return info, nil
}

// extractVideoInfoFromExtracted converts a go-ytdlp ExtractedInfo to our VideoInfo type.
func extractVideoInfoFromExtracted(raw *ytdlp.ExtractedInfo, videoURL string) VideoInfo {
	info := VideoInfo{URL: videoURL}
	if raw.ID != "" {
		info.ID = raw.ID
	}
	if raw.Title != nil {
		info.Title = *raw.Title
	}
	info.Thumbnail = extractThumbnailFromExtracted(raw)
	if raw.Duration != nil {
		info.Duration = *raw.Duration
	}
	if raw.Uploader != nil {
		info.Uploader = *raw.Uploader
	} else if raw.Channel != nil {
		info.Uploader = *raw.Channel
	}
	if raw.ExtractorKey != nil {
		info.Platform = *raw.ExtractorKey
	} else if raw.Extractor != nil {
		info.Platform = *raw.Extractor
	}
	if raw.WebpageURL != nil && *raw.WebpageURL != "" {
		info.URL = *raw.WebpageURL
	}
	info.Subtitles = extractSubtitleLangsFromExtracted(raw)
	return info
}

// extractThumbnailFromExtracted extracts the best thumbnail URL from ExtractedInfo.
func extractThumbnailFromExtracted(raw *ytdlp.ExtractedInfo) string {
	// Try direct thumbnail field
	if raw.Thumbnail != nil && *raw.Thumbnail != "" {
		if normalized := normalizeThumbnailURL(*raw.Thumbnail); normalized != "" {
			if !isPlaceholderThumbnailURL(normalized) {
				return normalized
			}
		}
	}
	// Try thumbnails array (last = highest quality)
	if raw.Thumbnails != nil {
		for i := len(raw.Thumbnails) - 1; i >= 0; i-- {
			thumb := raw.Thumbnails[i]
			if normalized := normalizeThumbnailURL(thumb.URL); normalized != "" {
				if !isPlaceholderThumbnailURL(normalized) {
					return normalized
				}
			}
		}
	}
	return ""
}

// extractSubtitleLangsFromExtracted extracts subtitle language info from ExtractedInfo.
func extractSubtitleLangsFromExtracted(raw *ytdlp.ExtractedInfo) []SubtitleLang {
	var result []SubtitleLang
	seen := make(map[string]bool)

	// Manual subtitles
	if raw.Subtitles != nil {
		for code, subs := range raw.Subtitles {
			selector := "manual:" + code
			if seen[selector] {
				continue
			}
			seen[selector] = true
			name := code
			if len(subs) > 0 && subs[0].Name != nil {
				name = *subs[0].Name
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: false, Selector: selector})
		}
	}
	// Automatic captions
	if raw.AutomaticCaptions != nil {
		for code, subs := range raw.AutomaticCaptions {
			selector := "auto:" + code
			if seen[selector] {
				continue
			}
			seen[selector] = true
			name := code
			if len(subs) > 0 && subs[0].Name != nil {
				name = *subs[0].Name
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: true, Selector: selector})
		}
	}
	return result
}

var listSubsLineRe = regexp.MustCompile(`^(\S+)(?:\s{2,}(.*?))?\s{2,}(\S.*)$`)

func (s *Service) mergeListedSubtitles(ctx context.Context, ytdlpPath string, videoURL string, settings Settings, existing []SubtitleLang) []SubtitleLang {
	listed, err := s.listSubtitles(ctx, ytdlpPath, videoURL, settings)
	if err != nil {
		s.emitLog("[GetVideoInfo] list-subs probe failed: %v", err)
		return existing
	}
	return mergeSubtitleEntries(existing, listed)
}

func (s *Service) listSubtitles(ctx context.Context, ytdlpPath string, videoURL string, settings Settings) ([]SubtitleLang, error) {
	cmd := s.newYtdlpCommand().SetExecutable(ytdlpPath).
		IgnoreConfig().ListSubs().NoPlaylist().NoWarnings()
	if settings.Proxy != "" {
		cmd.Proxy(settings.Proxy)
	}
	applyCookiesArgs(cmd, settings)
	s.applyMediaCommand(cmd)

	s.logCmd("GetVideoInfo:list-subs", cmd, ctx, videoURL)
	result, err := cmd.Run(ctx, videoURL)
	output := toUTF8([]byte(strings.TrimSpace(result.Stdout + "\n" + result.Stderr)))
	if err != nil {
		if output == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%s", strings.TrimSpace(output))
	}
	return parseListSubsOutput(output), nil
}

func parseListSubsOutput(output string) []SubtitleLang {
	var result []SubtitleLang
	seen := make(map[string]struct{})
	currentAuto := false
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "[info] Available automatic captions"):
			currentAuto = true
			continue
		case strings.HasPrefix(line, "[info] Available subtitles"):
			currentAuto = false
			continue
		case strings.HasPrefix(line, "Language"):
			continue
		case strings.HasPrefix(line, "["):
			continue
		}

		matches := listSubsLineRe.FindStringSubmatch(strings.TrimRight(rawLine, "\r"))
		if len(matches) == 0 {
			continue
		}
		code := strings.TrimSpace(matches[1])
		name := strings.TrimSpace(matches[2])
		if code == "" {
			continue
		}
		if name == "" {
			name = code
		}
		selector := subtitleSelector(code, currentAuto)
		if _, ok := seen[selector]; ok {
			continue
		}
		seen[selector] = struct{}{}
		result = append(result, SubtitleLang{Code: code, Name: name, Auto: currentAuto, Selector: selector})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Auto != result[j].Auto {
			return !result[i].Auto && result[j].Auto
		}
		return result[i].Code < result[j].Code
	})
	return result
}

func mergeSubtitleEntries(groups ...[]SubtitleLang) []SubtitleLang {
	merged := make([]SubtitleLang, 0)
	seen := make(map[string]int)
	for _, group := range groups {
		for _, item := range group {
			selector := item.Selector
			if selector == "" {
				selector = subtitleSelector(item.Code, item.Auto)
				item.Selector = selector
			}
			if idx, ok := seen[selector]; ok {
				if merged[idx].Name == merged[idx].Code && item.Name != "" {
					merged[idx].Name = item.Name
				}
				continue
			}
			seen[selector] = len(merged)
			merged = append(merged, item)
		}
	}
	return merged
}

func subtitleSelector(code string, auto bool) string {
	if auto {
		return "auto:" + code
	}
	return "manual:" + code
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
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" {
		return PlaylistInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(s.i18n, videoURL, settings); err != nil {
		s.emitLog("[GetPlaylistInfo] preflight failed: %v", err)
		return PlaylistInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := s.newYtdlpCommand().SetExecutable(ytdlpPath).
		IgnoreConfig().FlatPlaylist().DumpSingleJSON().NoWarnings()
	if settings.Proxy != "" {
		cmd.Proxy(settings.Proxy)
	}
	applyCookiesArgs(cmd, settings)
	s.applyMediaCommand(cmd)

	s.emitLog("[GetPlaylistInfo] fetching playlist for URL: %s", videoURL)
	s.logCmd("GetPlaylistInfo", cmd, ctx, videoURL)

	result, err := cmd.Run(ctx, videoURL)
	if err != nil {
		errMsg := normalizeYtDlpError(s.i18n, toUTF8([]byte(result.Stderr)), settings)
		s.emitLog("[GetPlaylistInfo] failed: err=%s, output=%s", toUTF8([]byte(err.Error())), errMsg)
		if errMsg != "" {
			return PlaylistInfo{}, fmt.Errorf("%s", errMsg)
		}
		return PlaylistInfo{}, fmt.Errorf("failed to get playlist info: %w", err)
	}

	infoList, parseErr := result.GetExtractedInfo()
	if parseErr != nil || len(infoList) == 0 {
		s.emitLog("[GetPlaylistInfo] JSON parse failed: %v", parseErr)
		return PlaylistInfo{}, fmt.Errorf("failed to parse playlist info: %w", parseErr)
	}

	raw := infoList[0]
	playlistResult := PlaylistInfo{URL: videoURL, Kind: detectCollectionKind(videoURL)}

	if raw.Title != nil {
		playlistResult.Title = *raw.Title
	}
	if raw.PlaylistTitle != nil && *raw.PlaylistTitle != "" {
		playlistResult.Title = *raw.PlaylistTitle
	}
	if raw.Uploader != nil {
		playlistResult.Uploader = *raw.Uploader
	} else if raw.PlaylistUploader != nil {
		playlistResult.Uploader = *raw.PlaylistUploader
	} else if raw.Channel != nil {
		playlistResult.Uploader = *raw.Channel
	}
	if playlistResult.Kind == "playlist" {
		if raw.ExtractorKey != nil && strings.Contains(strings.ToLower(*raw.ExtractorKey), "tab") {
			lowerTitle := strings.ToLower(playlistResult.Title)
			if strings.Contains(lowerTitle, "channel") || strings.Contains(lowerTitle, "videos") {
				playlistResult.Kind = "channel"
			}
		}
	}
	for _, entry := range raw.Entries {
		info := extractVideoInfoFromExtracted(entry, "")
		if info.URL != "" || info.ID != "" {
			playlistResult.Videos = append(playlistResult.Videos, info)
		}
	}
	playlistResult.Count = len(playlistResult.Videos)
	return playlistResult, nil
}

func (s *Service) GetFormats(rawInput string) (FormatInfo, error) {
	videoURL := extractURLFromText(rawInput)
	if isDouyinURL(videoURL) {
		return s.GetDouyinFormats(videoURL)
	}
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" {
		return FormatInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(s.i18n, videoURL, settings); err != nil {
		s.emitLog("[GetFormats] preflight failed: %v", err)
		return FormatInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := s.newYtdlpCommand().SetExecutable(ytdlpPath).
		IgnoreConfig().DumpJSON().SkipDownload().NoWarnings().NoPlaylist()
	if settings.Proxy != "" {
		cmd.Proxy(settings.Proxy)
	}
	applyCookiesArgs(cmd, settings)
	s.applyMediaCommand(cmd)

	s.emitLog("[GetFormats] fetching formats for URL: %s", videoURL)
	s.logCmd("GetFormats", cmd, ctx, videoURL)

	result, err := cmd.Run(ctx, videoURL)
	if err != nil {
		errMsg := normalizeYtDlpError(s.i18n, toUTF8([]byte(result.Stderr)), settings)
		s.emitLog("[GetFormats] failed: err=%s, output=%s", toUTF8([]byte(err.Error())), errMsg)
		if errMsg != "" {
			return FormatInfo{}, fmt.Errorf("%s", errMsg)
		}
		return FormatInfo{}, fmt.Errorf("failed to get formats: %w", err)
	}

	infoList, parseErr := result.GetExtractedInfo()
	if parseErr != nil || len(infoList) == 0 {
		s.emitLog("[GetFormats] JSON parse failed: %v", parseErr)
		return FormatInfo{}, fmt.Errorf("failed to parse JSON: %w", parseErr)
	}

	raw := infoList[0]
	fmtResult := FormatInfo{URL: videoURL}
	if raw.Title != nil {
		fmtResult.Title = *raw.Title
	}
	for _, f := range raw.Formats {
		format := Format{}
		if f.FormatID != nil {
			format.FormatID = *f.FormatID
		}
		if f.Extension != nil {
			format.Ext = *f.Extension
		}
		if f.Resolution != nil {
			format.Resolution = *f.Resolution
		}
		if f.FPS != nil {
			format.FPS = *f.FPS
		}
		if f.VCodec != nil {
			format.VCodec = *f.VCodec
			format.HasVideo = *f.VCodec != "" && *f.VCodec != "none"
		}
		if f.ACodec != nil {
			format.ACodec = *f.ACodec
			format.HasAudio = *f.ACodec != "" && *f.ACodec != "none"
		}
		if f.FileSize != nil {
			format.Filesize = int64(*f.FileSize)
		} else if f.FileSizeApprox != nil {
			format.Filesize = int64(*f.FileSizeApprox)
		}
		if f.TBR != nil {
			format.TBR = *f.TBR
		}
		if f.FormatNote != nil {
			format.Note = *f.FormatNote
		}
		fmtResult.Formats = append(fmtResult.Formats, format)
	}
	return fmtResult, nil
}

// --- Thumbnail helpers ---

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

func (s *Service) resolveVideoThumbnailFallback(info VideoInfo, raw *ytdlp.ExtractedInfo) string {
	if info.Thumbnail != "" && !isPlaceholderThumbnailURL(info.Thumbnail) {
		return ""
	}
	if !isBilibiliVideoExtracted(info, raw) {
		return ""
	}
	bvid := extractBilibiliBVIDExtracted(info, raw)
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

func isBilibiliVideoExtracted(info VideoInfo, raw *ytdlp.ExtractedInfo) bool {
	platform := strings.ToLower(info.Platform)
	videoURL := strings.ToLower(info.URL)
	if strings.Contains(platform, "bilibili") || strings.Contains(videoURL, "bilibili.com") {
		return true
	}
	if raw.ExtractorKey != nil && strings.Contains(strings.ToLower(*raw.ExtractorKey), "bilibili") {
		return true
	}
	return false
}

func extractBilibiliBVIDExtracted(info VideoInfo, raw *ytdlp.ExtractedInfo) string {
	if id := strings.TrimSpace(info.ID); strings.HasPrefix(strings.ToUpper(id), "BV") {
		return id
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
