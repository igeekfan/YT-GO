package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	githubOwner = "igeekfan"
	githubRepo  = "YT-GO"
)

func (s *Service) GetCurrentVersion() string { return s.appVersion }

func compareVersion(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	for i := 0; i < max(len(parts1), len(parts2)); i++ {
		n1, n2 := 0, 0
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

func (s *Service) CheckForUpdate() (UpdateInfo, error) {
	currentVersion := s.appVersion
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to parse response: %w", err)
	}
	tagName, _ := data["tag_name"].(string)
	latestVersion := strings.TrimPrefix(tagName, "v")
	if compareVersion(latestVersion, currentVersion) > 0 {
		htmlURL, _ := data["html_url"].(string)
		releaseName, _ := data["name"].(string)
		releaseBody, _ := data["body"].(string)
		publishedAt, _ := data["published_at"].(string)
		return UpdateInfo{HasUpdate: true, CurrentVersion: currentVersion, LatestVersion: latestVersion, ReleaseName: releaseName, ReleaseBody: releaseBody, HTMLURL: htmlURL, PublishedAt: publishedAt}, nil
	}
	return UpdateInfo{HasUpdate: false, CurrentVersion: currentVersion, LatestVersion: latestVersion}, nil
}
