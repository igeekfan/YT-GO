package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	wechatChannelsPlatform      = "WeChat Channels"
	wechatChannelsDefaultFormat = "wechat:best"
	wechatChannelsPreviewFormat = "wechat:preview"
	wechatChannelsOriginFormat  = "wechat:origin"
	wechatChannelsCacheTTL      = 5 * time.Minute
)

type wechatChannelsInput struct {
	ShareURL    string
	PlayableURL string
	ID          string
	Token       string
	ExportID    string
}

type wechatChannelsResolved struct {
	Info      VideoInfo
	Formats   FormatInfo
	VideoURLs map[string]string
}

type wechatChannelsCacheEntry struct {
	Resolved  *wechatChannelsResolved
	ExpiresAt time.Time
}

type wechatParseResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data wechatParseData `json:"data"`
}

type wechatParseData struct {
	CoverURL string `json:"cover_url"`
	Author   string `json:"author"`
	Desc     string `json:"desc"`
	Playable string `json:"playable_url"`
}

type wechatFeedResponse struct {
	Data    wechatFeedData `json:"data"`
	ErrCode int            `json:"errCode"`
	ErrMsg  string         `json:"errMsg"`
}

type wechatFeedData struct {
	AuthorInfo wechatAuthorInfo `json:"authorInfo"`
	FeedInfo   wechatFeedInfo   `json:"feedInfo"`
}

type wechatAuthorInfo struct {
	Nickname string `json:"nickname"`
}

type wechatFeedInfo struct {
	VideoURL       string `json:"videoUrl"`
	OriginVideoURL string `json:"originVideoUrl"`
	Description    string `json:"description"`
	CoverURL       string `json:"coverUrl"`
}

var (
	wechatSphPathRe      = regexp.MustCompile(`^/sph/([A-Za-z0-9_-]+)/?$`)
	wechatInvalidFileRe  = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	wechatTemplateKeyRe  = regexp.MustCompile(`%\(([A-Za-z0-9_]+)\)s`)
	wechatWhitespaceRe   = regexp.MustCompile(`\s+`)
	wechatFormatPrefixRe = regexp.MustCompile(`^(?:f|fv|fa):`)
)

func parseWechatChannelsInput(rawInput string) (wechatChannelsInput, error) {
	rawURL := extractURLFromText(rawInput)
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return wechatChannelsInput{}, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return wechatChannelsInput{}, fmt.Errorf("unsupported URL scheme")
	}

	host := strings.ToLower(parsed.Hostname())
	path := parsed.EscapedPath()
	input := wechatChannelsInput{}

	switch host {
	case "weixin.qq.com":
		match := wechatSphPathRe.FindStringSubmatch(parsed.Path)
		if match == nil {
			return wechatChannelsInput{}, fmt.Errorf("not a WeChat Channels share URL")
		}
		input.ID = match[1]
		input.ShareURL = "https://weixin.qq.com/sph/" + input.ID
		return input, nil
	case "channels.weixin.qq.com":
		if match := wechatSphPathRe.FindStringSubmatch(parsed.Path); match != nil {
			input.ID = match[1]
			input.ShareURL = "https://weixin.qq.com/sph/" + input.ID
			return input, nil
		}
		if strings.EqualFold(parsed.Path, "/finder-preview/pages/sph") {
			id := strings.TrimSpace(parsed.Query().Get("id"))
			if id == "" {
				return wechatChannelsInput{}, fmt.Errorf("missing sph id")
			}
			input.ID = id
			input.ShareURL = "https://weixin.qq.com/sph/" + id
			return input, nil
		}
		if strings.EqualFold(parsed.Path, "/finder-preview/pages/feed") || strings.HasSuffix(path, "/finder-preview/pages/feed") {
			token := parsed.Query().Get("token")
			eid := parsed.Query().Get("eid")
			if token == "" || eid == "" {
				return wechatChannelsInput{}, fmt.Errorf("missing token or eid in WeChat Channels preview URL")
			}
			input.PlayableURL = parsed.String()
			input.Token = token
			input.ExportID = eid
			return input, nil
		}
	}

	return wechatChannelsInput{}, fmt.Errorf("not a WeChat Channels URL")
}

func (s *Service) GetWechatChannelsVideoInfo(rawInput string) (VideoInfo, error) {
	resolved, err := s.resolveWechatChannels(context.Background(), rawInput)
	if err != nil {
		return VideoInfo{}, err
	}
	return resolved.Info, nil
}

func (s *Service) GetWechatChannelsFormats(rawInput string) (FormatInfo, error) {
	resolved, err := s.resolveWechatChannels(context.Background(), rawInput)
	if err != nil {
		return FormatInfo{}, err
	}
	return resolved.Formats, nil
}

func (s *Service) resolveWechatChannels(parent context.Context, rawInput string) (*wechatChannelsResolved, error) {
	input, err := parseWechatChannelsInput(rawInput)
	if err != nil {
		return nil, err
	}
	cacheKey := firstNonEmpty(input.ShareURL, input.PlayableURL)
	if cached := s.getCachedWechatChannelsResolved(cacheKey); cached != nil {
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()

	settings := s.GetSettings()
	parseData := wechatParseData{}
	if input.PlayableURL == "" {
		cookieHeader, err := yuanbaoCookieHeader(settings)
		if err != nil {
			return nil, err
		}
		parseData, err = s.parseWechatChannelsShareURL(ctx, input.ShareURL, cookieHeader, settings)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(parseData.Playable) == "" {
			return nil, fmt.Errorf("yuanbao parse result did not include playable_url")
		}
		input.PlayableURL = parseData.Playable
		if err := fillWechatPlayableTokens(&input); err != nil {
			return nil, err
		}
		s.emitLog("[WechatChannels] Yuanbao parsed share URL: id=%s eid=%s", input.ID, input.ExportID)
	}

	feedResp, err := s.fetchWechatChannelsFeedInfo(ctx, input, settings)
	if err != nil {
		return nil, err
	}
	if feedResp.ErrCode != 0 {
		if feedResp.ErrMsg != "" {
			return nil, fmt.Errorf("WeChat Channels feed API error %d: %s", feedResp.ErrCode, feedResp.ErrMsg)
		}
		return nil, fmt.Errorf("WeChat Channels feed API error %d", feedResp.ErrCode)
	}

	resolved := buildWechatChannelsResolved(input, parseData, feedResp)
	if resolved.Info.Title == "" {
		resolved.Info.Title = "WeChat Channels Video"
	}
	if resolved.VideoURLs[wechatChannelsDefaultFormat] == "" {
		return nil, fmt.Errorf("WeChat Channels feed did not include a downloadable video URL; Yuanbao parsing succeeded, but WeChat preview API did not return videoUrl")
	}
	s.cacheWechatChannelsResolved(cacheKey, resolved)
	return resolved, nil
}

func (s *Service) getCachedWechatChannelsResolved(key string) *wechatChannelsResolved {
	s.wechatCacheMu.Lock()
	defer s.wechatCacheMu.Unlock()
	entry, ok := s.wechatCache[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.wechatCache, key)
		return nil
	}
	return entry.Resolved
}

func (s *Service) cacheWechatChannelsResolved(key string, resolved *wechatChannelsResolved) {
	if key == "" || resolved == nil {
		return
	}
	s.wechatCacheMu.Lock()
	defer s.wechatCacheMu.Unlock()
	s.wechatCache[key] = wechatChannelsCacheEntry{
		Resolved:  resolved,
		ExpiresAt: time.Now().Add(wechatChannelsCacheTTL),
	}
}

func yuanbaoCookieHeader(settings Settings) (string, error) {
	if strings.TrimSpace(settings.CookiesFile) == "" {
		if settings.CookiesFrom != "" {
			return "", fmt.Errorf("WeChat Channels share parsing needs Yuanbao cookies.txt; browser cookie import is only passed to yt-dlp")
		}
		return "", fmt.Errorf("WeChat Channels share parsing needs Yuanbao cookies.txt. Log in to yuanbao.tencent.com, export cookies.txt, then select it in Settings")
	}
	cookies, err := readCookiesFromFile(settings.CookiesFile, "yuanbao.tencent.com")
	if err != nil {
		return "", fmt.Errorf("failed to read Yuanbao cookies file: %w", err)
	}
	if len(cookies) == 0 {
		return "", fmt.Errorf("cookies file does not contain cookies for yuanbao.tencent.com")
	}
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name == "" {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("cookies file does not contain usable Yuanbao cookies")
	}
	return strings.Join(parts, "; "), nil
}

func fillWechatPlayableTokens(input *wechatChannelsInput) error {
	parsed, err := url.Parse(input.PlayableURL)
	if err != nil {
		return err
	}
	input.Token = parsed.Query().Get("token")
	input.ExportID = parsed.Query().Get("eid")
	if input.Token == "" || input.ExportID == "" {
		return fmt.Errorf("missing token or eid in playable_url")
	}
	return nil
}

func (s *Service) parseWechatChannelsShareURL(ctx context.Context, shareURL, cookieHeader string, settings Settings) (wechatParseData, error) {
	payload, err := json.Marshal(map[string]any{
		"type":  "video_channel_url",
		"url":   shareURL,
		"scene": 1,
	})
	if err != nil {
		return wechatParseData{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://yuanbao.tencent.com/api/weixin/get_parse_result", bytes.NewReader(payload))
	if err != nil {
		return wechatParseData{}, err
	}
	applyYuanbaoHeaders(req, cookieHeader)

	s.emitLog("[WechatChannels] parsing share URL via Yuanbao: %s", shareURL)
	resp, err := s.newWechatHTTPClient(settings).Do(req)
	if err != nil {
		return wechatParseData{}, fmt.Errorf("parse share url: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wechatParseData{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return wechatParseData{}, fmt.Errorf("yuanbao parse failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed wechatParseResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return wechatParseData{}, fmt.Errorf("parse yuanbao response: %w", err)
	}
	if parsed.Code != 0 {
		if parsed.Msg != "" {
			return wechatParseData{}, fmt.Errorf("yuanbao parse failed: %s", parsed.Msg)
		}
		return wechatParseData{}, fmt.Errorf("yuanbao parse failed with code %d", parsed.Code)
	}
	return parsed.Data, nil
}

func (s *Service) fetchWechatChannelsFeedInfo(ctx context.Context, input wechatChannelsInput, settings Settings) (wechatFeedResponse, error) {
	payload, err := json.Marshal(map[string]any{
		"baseReq":  map[string]string{"generalToken": input.Token},
		"exportId": input.ExportID,
	})
	if err != nil {
		return wechatFeedResponse{}, err
	}

	endpoint := "https://channels.weixin.qq.com/finder-preview/api/feed/get_feed_info?_rid=" + url.QueryEscape(generateWechatRid()) + "&_pageUrl=https:%2F%2Fchannels.weixin.qq.com%2Ffinder-preview%2Fpages%2Ffeed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return wechatFeedResponse{}, err
	}
	applyWechatPreviewHeaders(req, buildWechatFeedReferer(input.ExportID, input.Token))

	s.emitLog("[WechatChannels] fetching feed info: eid=%s", input.ExportID)
	resp, err := s.newWechatHTTPClient(settings).Do(req)
	if err != nil {
		return wechatFeedResponse{}, fmt.Errorf("get feed info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wechatFeedResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return wechatFeedResponse{}, fmt.Errorf("feed info failed with HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var feedResp wechatFeedResponse
	if err := json.Unmarshal(body, &feedResp); err != nil {
		return wechatFeedResponse{}, fmt.Errorf("parse feed info response: %w", err)
	}
	return feedResp, nil
}

func buildWechatChannelsResolved(input wechatChannelsInput, parseData wechatParseData, feedResp wechatFeedResponse) *wechatChannelsResolved {
	feed := feedResp.Data.FeedInfo
	author := feedResp.Data.AuthorInfo
	title := strings.TrimSpace(parseData.Desc)
	if title == "" {
		title = strings.TrimSpace(feed.Description)
	}
	uploader := strings.TrimSpace(parseData.Author)
	if uploader == "" {
		uploader = strings.TrimSpace(author.Nickname)
	}
	thumbnail := strings.TrimSpace(parseData.CoverURL)
	if thumbnail == "" {
		thumbnail = strings.TrimSpace(feed.CoverURL)
	}

	infoURL := firstNonEmpty(input.ShareURL, input.PlayableURL)
	originVideoURL := getWechatOriginalVideoURL(feed)
	previewVideoURL := strings.TrimSpace(feed.VideoURL)
	videoURLs := map[string]string{}
	formats := FormatInfo{URL: infoURL, Title: title}
	appendFormat := func(formatID, videoURL, note string) {
		if videoURL == "" {
			return
		}
		videoURLs[formatID] = videoURL
		formats.Formats = append(formats.Formats, Format{
			FormatID:   formatID,
			Ext:        "mp4",
			Resolution: "unknown",
			VCodec:     "unknown",
			ACodec:     "aac",
			Note:       note,
			HasVideo:   true,
			HasAudio:   true,
		})
	}
	appendFormat(wechatChannelsOriginFormat, originVideoURL, "Origin")
	appendFormat(wechatChannelsPreviewFormat, previewVideoURL, "Preview")
	videoURLs[wechatChannelsDefaultFormat] = firstNonEmpty(originVideoURL, previewVideoURL)

	info := VideoInfo{
		URL:       infoURL,
		ID:        firstNonEmpty(input.ID, input.ExportID),
		Title:     title,
		Thumbnail: thumbnail,
		Uploader:  uploader,
		Platform:  wechatChannelsPlatform,
	}

	return &wechatChannelsResolved{
		Info:      info,
		Formats:   formats,
		VideoURLs: videoURLs,
	}
}

func getWechatOriginalVideoURL(feed wechatFeedInfo) string {
	for _, rawURL := range []string{feed.VideoURL, feed.OriginVideoURL} {
		if cleaned := cleanWechatOriginalVideoURL(rawURL); cleaned != "" {
			return cleaned
		}
	}
	return strings.TrimSpace(feed.OriginVideoURL)
}

func cleanWechatOriginalVideoURL(rawURL string) string {
	decodedURL, err := url.PathUnescape(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(decodedURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	encFileKey := parsed.Query().Get("encfilekey")
	token := parsed.Query().Get("token")
	if encFileKey == "" || token == "" {
		return ""
	}

	cleaned := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   parsed.Path,
	}
	query := url.Values{}
	query.Set("encfilekey", encFileKey)
	query.Set("token", token)
	cleaned.RawQuery = query.Encode()
	return cleaned.String()
}

func (s *Service) runWechatChannelsDownload(taskID string, req DownloadRequest, ctx context.Context) {
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Starting WeChat Channels download: %s", req.URL))
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir))

	resolved, err := s.resolveWechatChannels(ctx, req.URL)
	if err != nil {
		s.finishWechatChannelsDownloadError(taskID, err)
		return
	}

	formatID := resolveWechatChannelsFormatID(req.Quality)
	videoURL := resolved.VideoURLs[formatID]
	if videoURL == "" {
		formatID = wechatChannelsDefaultFormat
		videoURL = resolved.VideoURLs[formatID]
	}
	if videoURL == "" {
		s.finishWechatChannelsDownloadError(taskID, fmt.Errorf("no downloadable WeChat Channels video URL found"))
		return
	}
	s.emitDownloadLog(taskID, fmt.Sprintf("[YT-GO] WeChat Channels format: %s", formatID))

	settings := s.GetSettings()
	outputPath, err := buildWechatChannelsOutputPath(req, settings, resolved.Info, videoURL)
	if err != nil {
		s.finishWechatChannelsDownloadError(taskID, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		s.finishWechatChannelsDownloadError(taskID, err)
		return
	}
	outputPath = ensureUniqueWechatFilePath(outputPath)
	partPath := outputPath + ".part"

	if err := s.downloadWechatChannelsFile(ctx, taskID, videoURL, partPath, outputPath, settings); err != nil {
		if ctx.Err() != nil {
			_ = os.Remove(partPath)
			s.finishWechatChannelsDownloadCancelled(taskID)
			return
		}
		_ = os.Remove(partPath)
		s.finishWechatChannelsDownloadError(taskID, err)
		return
	}

	s.saveWechatChannelsSidecars(ctx, taskID, outputPath, resolved.Info, settings, req.Options)
	s.finishWechatChannelsDownloadCompleted(taskID, outputPath)
}

func resolveWechatChannelsFormatID(quality string) string {
	value := strings.TrimSpace(quality)
	for {
		next := wechatFormatPrefixRe.ReplaceAllString(value, "")
		if next == value {
			break
		}
		value = next
	}
	if strings.Contains(value, "+") {
		value = strings.Split(value, "+")[0]
	}
	switch value {
	case wechatChannelsPreviewFormat, wechatChannelsOriginFormat, wechatChannelsDefaultFormat:
		return value
	default:
		return wechatChannelsDefaultFormat
	}
}

func buildWechatChannelsOutputPath(req DownloadRequest, settings Settings, info VideoInfo, videoURL string) (string, error) {
	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		return "", fmt.Errorf("output directory is required")
	}
	ext := "mp4"
	if fromURL := extensionFromURL(videoURL); fromURL != "" {
		ext = fromURL
	}

	template := "%(title)s.%(ext)s"
	if strings.TrimSpace(settings.FilenameTemplate) != "" {
		template = strings.TrimSpace(settings.FilenameTemplate)
	}
	if req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "" {
		template = strings.TrimSpace(req.Options.FilenameTemplate)
	}

	filename := applyWechatFilenameTemplate(template, info, ext)
	filename = sanitizeWechatFilename(filename)
	if filename == "" || filename == "." {
		filename = "WeChat Channels Video." + ext
	}
	if strings.ToLower(filepath.Ext(filename)) != "."+strings.ToLower(ext) {
		filename += "." + ext
	}
	return filepath.Join(outputDir, filename), nil
}

func applyWechatFilenameTemplate(template string, info VideoInfo, ext string) string {
	values := map[string]string{
		"id":          info.ID,
		"title":       firstNonEmpty(info.Title, "WeChat Channels Video"),
		"uploader":    info.Uploader,
		"channel":     info.Uploader,
		"extractor":   wechatChannelsPlatform,
		"platform":    wechatChannelsPlatform,
		"upload_date": "",
		"ext":         ext,
	}
	return wechatTemplateKeyRe.ReplaceAllStringFunc(template, func(match string) string {
		parts := wechatTemplateKeyRe.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return values[parts[1]]
	})
}

func sanitizeWechatFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = wechatInvalidFileRe.ReplaceAllString(name, " ")
	name = wechatWhitespaceRe.ReplaceAllString(name, " ")
	name = strings.TrimSpace(strings.Trim(name, "."))
	if len([]rune(name)) > 180 {
		runes := []rune(name)
		name = string(runes[:180])
		name = strings.TrimSpace(strings.Trim(name, "."))
	}
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if reserved[strings.ToUpper(base)] {
		name = "_" + name
	}
	return name
}

func extensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), ".")
	switch ext {
	case "mp4", "m4v", "mov":
		return ext
	default:
		return ""
	}
}

func ensureUniqueWechatFilePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
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
		s.emitDownloadUpdate(updated)
	}
}

func (s *Service) finishWechatChannelsDownloadCompleted(taskID, outputPath string) {
	s.clearActiveDownload(taskID)
	var updated *DownloadTask
	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		task.Status = "completed"
		task.Progress = 100
		task.ETA = ""
		task.OutputPath = outputPath
		copy := *task
		updated = &copy
	}
	s.mu.Unlock()
	if updated != nil {
		s.emitDownloadLog(taskID, "[YT-GO] WeChat Channels download completed")
		s.emitDownloadUpdate(updated)
		go s.upsertRecord(updated)
	}
}

func (s *Service) finishWechatChannelsDownloadCancelled(taskID string) {
	s.clearActiveDownload(taskID)
	s.mu.Lock()
	if _, ok := s.downloads[taskID]; ok {
		delete(s.downloads, taskID)
	}
	s.mu.Unlock()
	s.emitDownloadRemove(taskID)
	go s.deleteRecords([]string{taskID})
}

func (s *Service) finishWechatChannelsDownloadError(taskID string, err error) {
	s.clearActiveDownload(taskID)
	var updated *DownloadTask
	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		task.Status = "error"
		task.Error = err.Error()
		copy := *task
		updated = &copy
	}
	s.mu.Unlock()
	if updated != nil {
		s.emitDownloadLog(taskID, "[YT-GO] WeChat Channels download failed: "+err.Error())
		s.emitDownloadUpdate(updated)
		go s.upsertRecord(updated)
	}
}

func (s *Service) newWechatHTTPClient(settings Settings) *http.Client {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if strings.TrimSpace(settings.Proxy) != "" {
		if proxyURL, err := url.Parse(settings.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{Transport: transport}
}

func applyYuanbaoHeaders(req *http.Request, cookieHeader string) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://yuanbao.tencent.com")
	req.Header.Set("Referer", "https://yuanbao.tencent.com/chat/naQivTmsDa/cf4d0079-ed1b-4c55-a3f3-2ca1379727d1")
	req.Header.Set("User-Agent", wechatDesktopUserAgent())
	req.Header.Set("Sec-CH-UA", `"Chromium";v="140", "Google Chrome";v="140", "Not=A?Brand";v="99"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("T-Userid", "b9575f6b0a8c4a55a08096904a5ef20a")
	req.Header.Set("X-Agentid", "naQivTmsDa/cf4d0079-ed1b-4c55-a3f3-2ca1379727d1")
	req.Header.Set("X-Commit-Tag", "72282a0d")
	req.Header.Set("X-Device-Id", "1921b001708100d7fa31002b9646bd0cc15a3e2e1f")
	req.Header.Set("X-Hy106", "")
	req.Header.Set("X-Hy92", "e963067ffa31002b9646bd0c03000008b1951a")
	req.Header.Set("X-Hy93", "1921b001708100d7fa31002b9646bd0cc15a3e2e1f")
	req.Header.Set("X-Id", "b9575f6b0a8c4a55a08096904a5ef20a")
	req.Header.Set("X-Instance-Id", "5")
	req.Header.Set("X-Language", "zh-CN")
	req.Header.Set("X-Os_Version", "Windows(10)-Blink")
	req.Header.Set("X-Platform", "win")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Source", "web")
	req.Header.Set("X-Web-Third-Source", "main")
	req.Header.Set("X-Webdriver", "0")
	req.Header.Set("X-Webversion", "2.69.0")
	req.Header.Set("X-Ybuitest", "0")
	req.Header.Set("Cookie", cookieHeader)
}

func applyWechatPreviewHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://channels.weixin.qq.com")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-CH-UA", `"Chromium";v="140", "Google Chrome";v="140", "Not=A?Brand";v="99"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", wechatDesktopUserAgent())
}

func buildWechatFeedReferer(exportID, generalToken string) string {
	return "https://channels.weixin.qq.com/finder-preview/pages/feed" +
		"?entry_card_type=48&comment_scene=39&appid=0" +
		"&token=" + url.QueryEscape(generalToken) +
		"&entry_scene=0&eid=" + url.QueryEscape(exportID)
}

func wechatDesktopUserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
}

func generateWechatRid() string {
	var randomBytes [4]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return fmt.Sprintf("%x-00000000", time.Now().Unix())
	}
	return fmt.Sprintf("%x-%s", time.Now().Unix(), hex.EncodeToString(randomBytes[:]))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
