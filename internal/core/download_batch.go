package core

import (
	"net/url"
	"strings"
)

// DownloadURLReview describes a non-blank line in a multi-link submission.
type DownloadURLReview struct {
	Line   int    `json:"line"`
	URL    string `json:"url"`
	Key    string `json:"key"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// ReviewDownloadURLs checks each line and marks repeated valid URLs in input order.
func ReviewDownloadURLs(input string, validate func(string) (string, error)) []DownloadURLReview {
	input = strings.ReplaceAll(strings.ReplaceAll(input, "\r\n", "\n"), "\r", "\n")
	results := make([]DownloadURLReview, 0)
	seen := make(map[string]DownloadURLReview)
	for index, line := range strings.Split(input, "\n") {
		rawURL := extractURLFromText(line)
		if rawURL == "" {
			continue
		}
		result := DownloadURLReview{Line: index + 1, URL: rawURL, Status: "invalid"}
		if parsed, ok := parseDownloadURL(rawURL); ok {
			result.Key = downloadURLKey(parsed)
			if first, exists := seen[result.Key]; exists {
				result.Status, result.Error = first.Status, first.Error
				if first.Status == "valid" {
					result.Status = "duplicate"
				}
			} else {
				result.Status = "valid"
				if validate != nil {
					validatedURL, err := validate(rawURL)
					if err != nil {
						result.Status, result.Error = "invalid", err.Error()
					} else {
						result.URL = validatedURL
					}
				}
				seen[result.Key] = result
			}
		}
		results = append(results, result)
	}
	return results
}

func downloadURLKey(parsed *url.URL) string {
	normalized := *parsed
	port := parsed.Port()
	normalized.Host = strings.ToLower(parsed.Host)
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		normalized.Host = strings.TrimSuffix(normalized.Host, ":"+port)
	}
	if normalized.Path == "" {
		normalized.Path = "/"
	}
	return normalized.String()
}
