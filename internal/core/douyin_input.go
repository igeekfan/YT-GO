package core

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	douyinURLPattern      = regexp.MustCompile(`(?i)https?://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)
	douyinLooseURLPattern = regexp.MustCompile(`(?i)(?:https?://)?(?:v\.douyin\.com|iesdouyin\.com|(?:www\.|m\.)?douyin\.com)/[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]*`)
	douyinIDPattern       = regexp.MustCompile(`\b\d{15,24}\b`)
	douyinParamIDPattern  = regexp.MustCompile(`(?i)(?:modal_id|item_ids|group_id|aweme_id)\s*=\s*(\d{8,24})`)
)

func extractDouyinTarget(input string) (string, string, error) {
	normalized := normalizeDouyinInput(input)
	if normalized == "" {
		return "", "", fmt.Errorf("douyin.link_not_found")
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
	return "", "", fmt.Errorf("douyin.link_not_found")
}

func normalizeDouyinInput(input string) string {
	replacer := strings.NewReplacer(
		"\u00a0", " ",
		"\u3000", " ",
		"\r", " ",
		"\n", " ",
		"\t", " ",
		"\u201c", `"`,
		"\u201d", `"`,
		"\u2018", `'`,
		"\u2019", `'`,
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
	return "", fmt.Errorf("douyin.video_id_not_found")
}
