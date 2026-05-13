package core

import (
	"net/url"
	"regexp"
	"strings"
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

// isYouTubeURL checks if the given URL points to YouTube.
func isYouTubeURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "youtube.com" || host == "www.youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be" || host == "www.youtu.be" || host == "youtube-nocookie.com" || strings.HasSuffix(host, ".youtube-nocookie.com")
}
