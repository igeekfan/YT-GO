package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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

// applyDouyinCookies adds user-configured cookies to the request.
// This allows Douyin API and page requests to use login cookies.
func applyDouyinCookies(req *http.Request, settings Settings) {
	if settings.CookiesFile != "" {
		// Read cookies from the cookies.txt file and add them to the request.
		cookies, err := readCookiesFromFile(settings.CookiesFile, req.URL.Host)
		if err == nil {
			for _, c := range cookies {
				req.AddCookie(c)
			}
		}
	}
}

func (s *Service) resolveDouyinRedirect(shareURL string, settings Settings) (string, error) {
	client, err := s.newDouyinHTTPClient(settings, 30*time.Second)
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, shareURL, nil)
		if err != nil {
			return "", err
		}
		applyHeaders(req, douyinDefaultHeaders)
		applyDouyinCookies(req, settings)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			return resp.Request.URL.String(), nil
		}
		if attempt == 2 {
			return "", errors.New(fmt.Sprintf(s.i18n.T("douyin.link_parse_failed_err"), err))
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return "", errors.New(s.i18n.T("douyin.link_parse_failed"))
}

func (s *Service) fetchDouyinItemInfo(videoID string, resolvedURL string, settings Settings) (map[string]any, error) {
	itemInfo, err := s.fetchDouyinItemInfoViaAPI(videoID, settings)
	if err == nil {
		return itemInfo, nil
	}
	s.emitLog(s.i18n.T("douyin.api_fallback"), err)
	return s.fetchDouyinItemInfoViaSharePage(videoID, resolvedURL, settings)
}

func (s *Service) fetchDouyinItemInfoViaAPI(videoID string, settings Settings) (map[string]any, error) {
	client, err := s.newDouyinHTTPClient(settings, 30*time.Second)
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
		applyDouyinCookies(req, settings)
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
				return nil, errors.New(fmt.Sprintf(s.i18n.T("douyin.status_code"), resp.StatusCode))
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
			return nil, errors.New(s.i18n.T("douyin.api_empty"))
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return nil, errors.New(s.i18n.T("douyin.api_failed"))
}

func (s *Service) fetchDouyinItemInfoViaSharePage(videoID string, resolvedURL string, settings Settings) (map[string]any, error) {
	shareURL := resolvedURL
	if parsed, err := url.Parse(resolvedURL); err == nil {
		if !strings.Contains(strings.ToLower(parsed.Hostname()), "iesdouyin.com") {
			shareURL = fmt.Sprintf("https://www.iesdouyin.com/share/video/%s/", videoID)
		}
	}
	client, err := s.newDouyinHTTPClient(settings, 30*time.Second)
	if err != nil {
		return nil, err
	}
	html, err := fetchDouyinHTML(client, shareURL, douyinMobileHeaders, settings)
	if err != nil {
		return nil, err
	}
	if strings.Contains(html, "Please wait...") && strings.Contains(html, "wci=") && strings.Contains(html, "cs=") {
		html = solveDouyinWAF(client, html, shareURL, settings)
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
	return nil, errors.New(s.i18n.T("douyin.share_page_not_found"))
}

func fetchDouyinHTML(client *http.Client, pageURL string, headers map[string]string, settings Settings) (string, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	applyHeaders(req, headers)
	applyDouyinCookies(req, settings)
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
		return "", fmt.Errorf("douyin.status_code:%d", resp.StatusCode)
	}
	return string(body), nil
}

func (s *Service) newDouyinHTTPClient(settings Settings, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if settings.Proxy != "" {
		proxyURL, err := url.Parse(settings.Proxy)
		if err != nil {
			return nil, errors.New(fmt.Sprintf(s.i18n.T("douyin.proxy_invalid"), err))
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: s.configureOutboundTransport(transport),
	}, nil
}

func applyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}
