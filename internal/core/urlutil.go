package core

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// shareTextURLRe matches a URL inside a noisy share-text snippet.
var shareTextURLRe = regexp.MustCompile(`https?://\S+`)

// extractURLFromText returns the embedded URL from a share-text snippet.
// If the input has no whitespace it is already a plain URL and is returned trimmed as-is.
func extractURLFromText(input string) string {
	trimmed := strings.TrimSpace(input)
	if !strings.ContainsAny(trimmed, " \t\r\n") {
		return trimmed
	}
	m := shareTextURLRe.FindString(trimmed)
	if m == "" {
		return trimmed
	}
	return strings.TrimRight(m, ".,;:!?)]}，。；：！？、）】》」』")
}

// ValidateSingleDownloadURL requires a standalone HTTP or HTTPS link for quick downloads.
func (s *Service) ValidateSingleDownloadURL(rawURL string) error {
	value := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" || parsed.User != nil || strings.ContainsFunc(value, unicode.IsSpace) || strings.Contains(value, `\`) {
		return errors.New(s.i18n.T("download.single_link"))
	}
	return nil
}

// isYouTubeURL checks if the given URL points to YouTube.
func isYouTubeURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "youtube.com" || host == "www.youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be" || host == "www.youtu.be" || host == "youtube-nocookie.com" || strings.HasSuffix(host, ".youtube-nocookie.com")
}

func isWechatChannelsURL(rawURL string) bool {
	_, err := parseWechatChannelsInput(rawURL)
	return err == nil
}
