package core

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var douyinRouterDataMark = "window._ROUTER_DATA = "

func solveDouyinWAF(client *http.Client, html string, pageURL string, settings Settings) string {
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
		applyDouyinCookies(req, settings)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookieValue, Path: "/"})
		resp, err := client.Do(req)
		if err != nil {
			return html
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close() // Close immediately after reading, not deferred in loop
		if readErr != nil {
			return html
		}
		return string(body)
	}
	return html
}

func extractDouyinRouterData(html string) (map[string]any, error) {
	start := strings.Index(html, douyinRouterDataMark)
	if start < 0 {
		return nil, fmt.Errorf("douyin.router_data_not_found")
	}
	idx := start + len(douyinRouterDataMark)
	for idx < len(html) && (html[idx] == ' ' || html[idx] == '\n' || html[idx] == '\r' || html[idx] == '\t') {
		idx++
	}
	if idx >= len(html) || html[idx] != '{' {
		return nil, fmt.Errorf("douyin.router_data_format")
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
		return nil, fmt.Errorf("douyin.router_data_incomplete")
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
		return "", fmt.Errorf("douyin.play_url_not_found")
	}
	playURL, _ := urlList[0].(string)
	playURL = strings.Replace(playURL, "playwm", "play", 1)
	if playURL == "" {
		return "", fmt.Errorf("douyin.no_watermark_not_found")
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

func sha256Hex(input []byte) string {
	sum := sha256.Sum256(input)
	return fmt.Sprintf("%x", sum[:])
}
