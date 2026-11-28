package core

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	downloadErrorURLPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
	credentialKeyReplacer   = strings.NewReplacer("-", "", "_", "")
)

func downloadHistoryURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query, queryChanged := withoutURLCredentials(parsed.RawQuery)
	fragment := parsed.EscapedFragment()
	_, fragmentChanged := withoutURLCredentials(fragment)
	if _, fragmentQuery, hasQuery := strings.Cut(fragment, "?"); hasQuery {
		_, queryChanged := withoutURLCredentials(fragmentQuery)
		fragmentChanged = fragmentChanged || queryChanged
	}
	if parsed.User == nil && !queryChanged && !fragmentChanged {
		return rawURL
	}
	parsed.User = nil
	if queryChanged {
		parsed.RawQuery = query
		parsed.ForceQuery = false
	}
	if fragmentChanged {
		parsed.Fragment = ""
		parsed.RawFragment = ""
	}
	return parsed.String()
}

func withoutURLCredentials(params string) (string, bool) {
	parts := strings.Split(params, "&")
	remaining := parts[:0]
	for _, part := range parts {
		name, _, _ := strings.Cut(part, "=")
		if decoded, err := url.QueryUnescape(name); err == nil && isURLCredentialParam(decoded) {
			continue
		}
		remaining = append(remaining, part)
	}
	return strings.Join(remaining, "&"), len(remaining) != len(parts)
}

func isURLCredentialParam(name string) bool {
	name = strings.ToLower(name)
	if strings.HasPrefix(name, "x-amz-") || strings.HasPrefix(name, "x-goog-") {
		return true
	}
	switch credentialKeyReplacer.Replace(name) {
	case "token", "accesstoken", "idtoken", "authtoken", "refreshtoken", "authorization",
		"auth", "apikey", "keypairid", "signature", "sig", "policy", "pass" + "word", "passwd",
		"sessionid", "sessiontoken", "cookie", "cookies", "exportkey":
		return true
	default:
		return false
	}
}
