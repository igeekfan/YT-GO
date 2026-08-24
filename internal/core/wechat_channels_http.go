package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

func (s *Service) newWechatHTTPClient(settings Settings) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if strings.TrimSpace(settings.Proxy) != "" {
		if proxyURL, err := url.Parse(settings.Proxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport, CheckRedirect: s.configureOutboundTransport(transport)}
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
