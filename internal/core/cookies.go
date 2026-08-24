package core

import (
	"net/http"
	"os"
	"strings"
)

// readCookiesFromFile reads Netscape-format cookies.txt and returns http.Cookies
// that match the given host. The format is one cookie per line:
// domain\tinclude_subdomains\tpath\tsecure\texpiry\tname\tvalue
func readCookiesFromFile(filePath string, host string) ([]*http.Cookie, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	host = strings.ToLower(host)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#HttpOnly_") {
			line = strings.TrimPrefix(line, "#HttpOnly_")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		domain := strings.TrimSpace(fields[0])
		// Check if cookie domain matches the host.
		domainLower := strings.ToLower(domain)
		domainMatch := strings.TrimPrefix(domainLower, ".")
		if domainMatch != host && !strings.HasSuffix(host, "."+domainMatch) {
			continue
		}
		secure := strings.TrimSpace(fields[3]) == "TRUE"
		name := strings.TrimSpace(fields[5])
		value := strings.TrimSpace(fields[6])
		cookies = append(cookies, &http.Cookie{
			Name:  name,
			Value: value,
		})
		_ = secure // secure flag used for matching but not needed in request cookie
	}
	return cookies, nil
}
