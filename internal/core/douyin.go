package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	douyinURLPattern      = regexp.MustCompile(`(?i)https?://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)
	douyinLooseURLPattern = regexp.MustCompile(`(?i)(?:https?://)?(?:v\.douyin\.com|iesdouyin\.com|(?:www\.|m\.)?douyin\.com)/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]*`)
	douyinIDPattern       = regexp.MustCompile(`\b\d{15,24}\b`)
	douyinParamIDPattern  = regexp.MustCompile(`(?i)(?:modal_id|item_ids|group_id|aweme_id)\s*=\s*(\d{8,24})`)
	douyinRouterDataMark  = "window._ROUTER_DATA = "
)

var douyinDefaultHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Accept":          "text/html,application/json,*/*",
	"Accept-Language": "en-US,en;q=0.9",
	"Connection":      "keep-alive",
	"Referer":         "https://www.douyin.com/",
}

var douyinMobileHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
	"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	"Referer":         "https://www.douyin.com/",
}

func isDouyinURL(rawURL string) bool {
	_, _, err := extractDouyinTarget(rawURL)
	return err == nil
}

func (s *Service) GetDouyinVideoInfo(rawURL string) (VideoInfo, error) {
	itemInfo, videoID, finalURL, err := s.resolveDouyinItemInfo(rawURL)
	if err != nil {
		return VideoInfo{}, err
	}
	video := VideoInfo{
		ID:       videoID,
		URL:      finalURL,
		Title:    douyinString(itemInfo, "desc"),
		Duration: douyinVideoDurationSeconds(itemInfo),
		Platform: "抖音",
	}
	if video.Title == "" {
		video.Title = "抖音视频_" + videoID
	}
	if author, ok := itemInfo["author"].(map[string]any); ok {
		video.Uploader = douyinString(author, "nickname")
	}
	if thumb := douyinCoverURL(itemInfo); thumb != "" {
		video.Thumbnail = thumb
	}
	return video, nil
}

func (s *Service) GetDouyinFormats(rawURL string) (FormatInfo, error) {
	itemInfo, _, finalURL, err := s.resolveDouyinItemInfo(rawURL)
	if err != nil {
		return FormatInfo{}, err
	}
	if _, err := douyinVideoURL(itemInfo); err != nil {
		return FormatInfo{}, err
	}
	width, height := douyinVideoDimensions(itemInfo)
	resolution := "原始"
	if width > 0 && height > 0 {
		resolution = fmt.Sprintf("%dx%d", width, height)
	}
	note := "抖音无水印"
	if height > 0 {
		note = fmt.Sprintf("抖音无水印 %dp", height)
	}
	title := douyinString(itemInfo, "desc")
	if title == "" {
		title = "抖音视频"
	}
	return FormatInfo{
		URL:   finalURL,
		Title: title,
		Formats: []Format{{
			FormatID:   "douyin_nowm",
			Ext:        "mp4",
			Resolution: resolution,
			VCodec:     "h264",
			ACodec:     "aac",
			HasVideo:   true,
			HasAudio:   true,
			Note:       note,
			TBR:        1,
		}},
	}, nil
}

func (s *Service) resolveDouyinItemInfo(rawInput string) (map[string]any, string, string, error) {
	shareURL, directVideoID, err := extractDouyinTarget(rawInput)
	if err != nil {
		return nil, "", "", err
	}
	settings := s.GetSettings()
	resolvedURL := shareURL
	videoID := directVideoID
	if videoID == "" {
		resolvedURL, err = s.resolveDouyinRedirect(shareURL, settings)
		if err != nil {
			return nil, "", "", err
		}
		videoID, err = extractDouyinVideoID(resolvedURL)
		if err != nil {
			return nil, "", "", err
		}
	}
	itemInfo, err := s.fetchDouyinItemInfo(videoID, resolvedURL, settings)
	if err != nil {
		return nil, "", "", err
	}
	return itemInfo, videoID, resolvedURL, nil
}

func extractDouyinTarget(input string) (string, string, error) {
	normalized := normalizeDouyinInput(input)
	if normalized == "" {
		return "", "", fmt.Errorf("未找到有效的抖音链接")
	}
	for _, match := range douyinURLPattern.FindAllString(normalized, -1) {
		candidate := cleanDouyinURLCandidate(match)
		if candidate == "" || !isDouyinShareURL(candidate) {
			continue
		}
		if videoID, err := extractDouyinVideoID(candidate); err == nil {
			return canonicalDouyinVideoURL(videoID), videoID, nil
		}
		return candidate, "", nil
	}
	for _, match := range douyinLooseURLPattern.FindAllString(normalized, -1) {
		candidate := cleanDouyinURLCandidate(match)
		if candidate == "" || !isDouyinShareURL(candidate) {
			continue
		}
		if videoID, err := extractDouyinVideoID(candidate); err == nil {
			return canonicalDouyinVideoURL(videoID), videoID, nil
		}
		return candidate, "", nil
	}
	if videoID := extractDouyinInputID(normalized); videoID != "" {
		return canonicalDouyinVideoURL(videoID), videoID, nil
	}
	return "", "", fmt.Errorf("未找到有效的抖音链接")
}

func normalizeDouyinInput(input string) string {
	replacer := strings.NewReplacer(
		"\u00a0", " ",
		"\u3000", " ",
		"\r", " ",
		"\n", " ",
		"\t", " ",
		"“", `"`,
		"”", `"`,
		"‘", `'`,
		"’", `'`,
	)
	return strings.TrimSpace(strings.Join(strings.Fields(replacer.Replace(input)), " "))
}

func cleanDouyinURLCandidate(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	candidate = strings.Trim(candidate, `"'<>`)
	candidate = strings.TrimRight(candidate, ".,;:!?)]}，。；：！？、）】》」』")
	if candidate == "" {
		return ""
	}
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	return candidate
}

func isDouyinShareURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, domain := range []string{"douyin.com", "iesdouyin.com", "v.douyin.com", "www.douyin.com", "m.douyin.com"} {
		if strings.Contains(host, domain) {
			return true
		}
	}
	return false
}

func extractDouyinInputID(input string) string {
	if match := douyinParamIDPattern.FindStringSubmatch(input); len(match) > 1 {
		return match[1]
	}
	trimmed := strings.TrimSpace(input)
	if douyinIDPattern.MatchString(trimmed) && strings.Trim(trimmed, "0123456789") == "" {
		return trimmed
	}
	if match := regexp.MustCompile(`(?i)/(?:video|note)/(\d{8,24})`).FindStringSubmatch(input); len(match) > 1 {
		return match[1]
	}
	return ""
}

func canonicalDouyinVideoURL(videoID string) string {
	return fmt.Sprintf("https://www.douyin.com/video/%s", videoID)
}

func (s *Service) resolveDouyinRedirect(shareURL string, settings Settings) (string, error) {
	client, err := newDouyinHTTPClient(settings, 30*time.Second)
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, shareURL, nil)
		if err != nil {
			return "", err
		}
		applyHeaders(req, douyinDefaultHeaders)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return resp.Request.URL.String(), nil
		}
		if attempt == 2 {
			return "", fmt.Errorf("抖音链接解析失败: %w", err)
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return "", fmt.Errorf("抖音链接解析失败")
}

func extractDouyinVideoID(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for _, key := range []string{"modal_id", "item_ids", "group_id", "aweme_id"} {
		if value := query.Get(key); value != "" {
			if match := regexp.MustCompile(`(\d{8,24})`).FindStringSubmatch(value); len(match) > 1 {
				return match[1], nil
			}
		}
	}
	for _, pattern := range []string{`/video/(\d{8,24})`, `/note/(\d{8,24})`, `/(\d{8,24})(?:/|$)`} {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(parsed.Path); len(match) > 1 {
			return match[1], nil
		}
	}
	if match := regexp.MustCompile(`(\d{15,24})`).FindStringSubmatch(rawURL); len(match) > 1 {
		return match[1], nil
	}
	return "", fmt.Errorf("无法从抖音链接中提取视频 ID")
}

func (s *Service) fetchDouyinItemInfo(videoID string, resolvedURL string, settings Settings) (map[string]any, error) {
	itemInfo, err := s.fetchDouyinItemInfoViaAPI(videoID, settings)
	if err == nil {
		return itemInfo, nil
	}
	s.emitLog("[Douyin] API 获取失败，回退分享页解析: %v", err)
	return s.fetchDouyinItemInfoViaSharePage(videoID, resolvedURL, settings)
}

func (s *Service) fetchDouyinItemInfoViaAPI(videoID string, settings Settings) (map[string]any, error) {
	client, err := newDouyinHTTPClient(settings, 30*time.Second)
	if err != nil {
		return nil, err
	}
	endpoint := "https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/?item_ids=" + url.QueryEscape(videoID)
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		applyHeaders(req, douyinDefaultHeaders)
		resp, err := client.Do(req)
		if err != nil {
			if attempt == 2 {
				return nil, err
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt == 2 {
				return nil, readErr
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if attempt == 2 {
				return nil, fmt.Errorf("状态码 %d", resp.StatusCode)
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			if attempt == 2 {
				return nil, err
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if items, ok := payload["item_list"].([]any); ok && len(items) > 0 {
			if item, ok := items[0].(map[string]any); ok {
				return item, nil
			}
		}
		if attempt == 2 {
			return nil, fmt.Errorf("抖音 API 返回空数据")
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return nil, fmt.Errorf("抖音 API 请求失败")
}

func (s *Service) fetchDouyinItemInfoViaSharePage(videoID string, resolvedURL string, settings Settings) (map[string]any, error) {
	shareURL := resolvedURL
	if parsed, err := url.Parse(resolvedURL); err == nil {
		if !strings.Contains(strings.ToLower(parsed.Hostname()), "iesdouyin.com") {
			shareURL = fmt.Sprintf("https://www.iesdouyin.com/share/video/%s/", videoID)
		}
	}
	client, err := newDouyinHTTPClient(settings, 30*time.Second)
	if err != nil {
		return nil, err
	}
	html, err := fetchDouyinHTML(client, shareURL, douyinMobileHeaders)
	if err != nil {
		return nil, err
	}
	if strings.Contains(html, "Please wait...") && strings.Contains(html, "wci=") && strings.Contains(html, "cs=") {
		html = solveDouyinWAF(client, html, shareURL)
	}
	routerData, err := extractDouyinRouterData(html)
	if err != nil {
		return nil, err
	}
	loaderData, _ := routerData["loaderData"].(map[string]any)
	for _, node := range loaderData {
		nodeMap, ok := node.(map[string]any)
		if !ok {
			continue
		}
		videoInfoRes, _ := nodeMap["videoInfoRes"].(map[string]any)
		itemList, _ := videoInfoRes["item_list"].([]any)
		if len(itemList) == 0 {
			continue
		}
		if item, ok := itemList[0].(map[string]any); ok {
			return item, nil
		}
	}
	return nil, fmt.Errorf("分享页中未找到抖音视频信息")
}

func fetchDouyinHTML(client *http.Client, pageURL string, headers map[string]string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	applyHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	return string(body), nil
}

func solveDouyinWAF(client *http.Client, html string, pageURL string) string {
	match := regexp.MustCompile(`wci="([^"]+)"\s*,\s*cs="([^"]+)"`).FindStringSubmatch(html)
	if len(match) < 3 {
		return html
	}
	cookieName := match[1]
	challengeBlob := match[2]
	decoded, err := decodeDouyinBase64(challengeBlob)
	if err != nil {
		return html
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return html
	}
	vMap, ok := payload["v"].(map[string]any)
	if !ok {
		return html
	}
	prefixRaw, okA := vMap["a"].(string)
	expectedRaw, okC := vMap["c"].(string)
	if !okA || !okC {
		return html
	}
	prefix, err := decodeDouyinBase64(prefixRaw)
	if err != nil {
		return html
	}
	expected, err := decodeDouyinBase64(expectedRaw)
	if err != nil {
		return html
	}
	expectedHex := fmt.Sprintf("%x", expected)
	for candidate := 0; candidate <= 1000000; candidate++ {
		sum := sha256Hex(append(prefix, []byte(strconv.Itoa(candidate))...))
		if sum != expectedHex {
			continue
		}
		payload["d"] = base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(candidate)))
		cookieBody, err := json.Marshal(payload)
		if err != nil {
			return html
		}
		cookieValue := base64.StdEncoding.EncodeToString(cookieBody)
		req, err := http.NewRequest(http.MethodGet, pageURL, nil)
		if err != nil {
			return html
		}
		applyHeaders(req, douyinMobileHeaders)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookieValue, Path: "/"})
		resp, err := client.Do(req)
		if err != nil {
			return html
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return html
		}
		return string(body)
	}
	return html
}

func extractDouyinRouterData(html string) (map[string]any, error) {
	start := strings.Index(html, douyinRouterDataMark)
	if start < 0 {
		return nil, fmt.Errorf("未找到抖音分享页路由数据")
	}
	idx := start + len(douyinRouterDataMark)
	for idx < len(html) && (html[idx] == ' ' || html[idx] == '\n' || html[idx] == '\r' || html[idx] == '\t') {
		idx++
	}
	if idx >= len(html) || html[idx] != '{' {
		return nil, fmt.Errorf("抖音分享页路由数据格式错误")
	}
	depth := 0
	inString := false
	escaped := false
	end := -1
	for pos := idx; pos < len(html); pos++ {
		ch := html[pos]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			depth++
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				end = pos + 1
				break
			}
		}
	}
	if end <= idx {
		return nil, fmt.Errorf("抖音分享页路由数据不完整")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(html[idx:end]), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func decodeDouyinBase64(value string) ([]byte, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(value, "-", "+"), "_", "/")
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}
	return base64.StdEncoding.DecodeString(normalized)
}

func newDouyinHTTPClient(settings Settings, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if settings.Proxy != "" {
		proxyURL, err := url.Parse(settings.Proxy)
		if err != nil {
			return nil, fmt.Errorf("代理配置无效: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

func applyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func douyinString(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

func douyinCoverURL(itemInfo map[string]any) string {
	video, _ := itemInfo["video"].(map[string]any)
	cover, _ := video["cover"].(map[string]any)
	urlList, _ := cover["url_list"].([]any)
	if len(urlList) == 0 {
		return ""
	}
	coverURL, _ := urlList[0].(string)
	return coverURL
}

func douyinVideoURL(itemInfo map[string]any) (string, error) {
	video, _ := itemInfo["video"].(map[string]any)
	playAddr, _ := video["play_addr"].(map[string]any)
	urlList, _ := playAddr["url_list"].([]any)
	if len(urlList) == 0 {
		return "", fmt.Errorf("未找到抖音视频播放地址")
	}
	playURL, _ := urlList[0].(string)
	playURL = strings.Replace(playURL, "playwm", "play", 1)
	if playURL == "" {
		return "", fmt.Errorf("未找到抖音无水印播放地址")
	}
	return playURL, nil
}

func douyinVideoDurationSeconds(itemInfo map[string]any) float64 {
	video, _ := itemInfo["video"].(map[string]any)
	duration, _ := video["duration"].(float64)
	if duration > 1000 {
		return duration / 1000
	}
	return duration
}

func douyinVideoDimensions(itemInfo map[string]any) (int, int) {
	video, _ := itemInfo["video"].(map[string]any)
	width, _ := video["width"].(float64)
	height, _ := video["height"].(float64)
	return int(width), int(height)
}

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

func (s *Service) runDouyinDownload(taskID string, req DownloadRequest, ctx context.Context) {
	settings := s.GetSettings()
	itemInfo, videoID, _, err := s.resolveDouyinItemInfo(req.URL)
	if err != nil {
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	if req.Quality == "audio" {
		s.finalizeDouyinDownloadError(taskID, fmt.Errorf("当前抖音专用下载仅支持视频，不支持音频提取"))
		return
	}
	videoURL, err := douyinVideoURL(itemInfo)
	if err != nil {
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	title := douyinString(itemInfo, "desc")
	if title == "" {
		title = "douyin_" + videoID
	}
	safeTitle := safeDouyinTitle(title, "douyin_"+videoID)
	outputPath := filepath.Join(req.OutputDir, safeTitle+".mp4")
	tempPath := outputPath + ".part"
	client, err := newDouyinHTTPClient(settings, 0)
	if err != nil {
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	applyHeaders(reqHTTP, douyinDefaultHeaders)
	resp, err := client.Do(reqHTTP)
	if err != nil {
		if ctx.Err() != nil {
			s.finalizeDouyinCancelled(taskID)
			return
		}
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.finalizeDouyinDownloadError(taskID, fmt.Errorf("抖音视频下载失败，状态码 %d", resp.StatusCode))
		return
	}
	file, err := os.Create(tempPath)
	if err != nil {
		s.finalizeDouyinDownloadError(taskID, err)
		return
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
				s.finalizeDouyinDownloadError(taskID, err)
				return
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
				s.finalizeDouyinCancelled(taskID)
				return
			}
			s.finalizeDouyinDownloadError(taskID, readErr)
			return
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		s.finalizeDouyinDownloadError(taskID, err)
		return
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		s.finalizeDouyinDownloadError(taskID, err)
		return
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
	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		task.Status = "completed"
		task.Progress = 100
		task.OutputPath = outputPath
		task.Size = formatDouyinBytes(written)
		copy := *task
		s.mu.Unlock()
		s.emitDownloadLog(taskID, "[Douyin] 下载完成: "+outputPath)
		s.emitDownloadUpdate(&copy)
		go s.upsertRecord(&copy)
		return
	}
	s.mu.Unlock()
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
		s.emitDownloadUpdate(updated)
	}
}

func (s *Service) finalizeDouyinDownloadError(taskID string, err error) {
	s.emitDownloadLog(taskID, "[Douyin] 下载失败: "+err.Error())
	s.mu.Lock()
	if task, ok := s.downloads[taskID]; ok {
		task.Status = "error"
		task.Error = err.Error()
		copy := *task
		s.mu.Unlock()
		s.emitDownloadUpdate(&copy)
		go s.upsertRecord(&copy)
		return
	}
	s.mu.Unlock()
}

func (s *Service) finalizeDouyinCancelled(taskID string) {
	s.mu.Lock()
	delete(s.cancelFns, taskID)
	_, ok := s.downloads[taskID]
	if ok {
		delete(s.downloads, taskID)
	}
	s.mu.Unlock()
	if ok {
		s.emitDownloadRemove(taskID)
		go s.deleteRecords([]string{taskID})
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
		return fmt.Errorf("状态码 %d", resp.StatusCode)
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

func sha256Hex(input []byte) string {
	sum := sha256.Sum256(input)
	return fmt.Sprintf("%x", sum[:])
}
