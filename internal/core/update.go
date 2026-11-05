package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	githubOwner = "igeekfan"
	githubRepo  = "YT-GO"

	githubLatestReleaseURL = "https://api.github.com/repos/igeekfan/YT-GO/releases/latest"
	maxUpdateDownloadSize  = int64(2 * 1024 * 1024 * 1024)
)

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt string               `json:"published_at"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
}

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
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "YT-GO-update-check")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to parse response: %w", err)
	}

	latestVersion := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	info := UpdateInfo{
		HasUpdate:      compareVersion(latestVersion, s.appVersion) > 0,
		CurrentVersion: s.appVersion,
		LatestVersion:  latestVersion,
		ReleaseName:    release.Name,
		ReleaseBody:    release.Body,
		HTMLURL:        release.HTMLURL,
		PublishedAt:    release.PublishedAt,
	}
	if !info.HasUpdate {
		return info, nil
	}

	asset, ok := selectUpdateAsset(runtime.GOOS, runtime.GOARCH, release.Assets)
	if !ok {
		return info, nil
	}
	info.AssetName = asset.Name
	info.AssetURL = asset.BrowserDownloadURL
	info.AssetSize = asset.Size
	info.AssetDigest = asset.Digest
	info.CanInstall = isAllowedUpdateURL(asset.BrowserDownloadURL)
	return info, nil
}

func selectUpdateAsset(goos, goarch string, assets []githubReleaseAsset) (githubReleaseAsset, bool) {
	suffixes := updateAssetSuffixes(goos, goarch)
	for _, suffix := range suffixes {
		for _, asset := range assets {
			if strings.HasSuffix(asset.Name, suffix) && asset.BrowserDownloadURL != "" {
				return asset, true
			}
		}
	}
	return githubReleaseAsset{}, false
}

func updateAssetSuffixes(goos, goarch string) []string {
	switch goos {
	case "windows":
		arch := map[string]string{"amd64": "x64", "arm64": "arm64", "386": "x86"}[goarch]
		if arch == "" {
			return nil
		}
		// The installer can replace an installed desktop app. A portable ZIP
		// cannot be applied safely while the running executable is locked.
		return []string{"_windows_" + arch + ".exe"}
	case "darwin":
		arch := map[string]string{"amd64": "intel", "arm64": "arm64"}[goarch]
		if arch == "" {
			return nil
		}
		return []string{"_mac_" + arch + ".dmg"}
	case "linux":
		appImageArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64", "386": "i386"}[goarch]
		debArch := map[string]string{"amd64": "amd64", "arm64": "arm64", "386": "i386"}[goarch]
		if appImageArch == "" || debArch == "" {
			return nil
		}
		return []string{"_" + appImageArch + ".AppImage", "_linux_" + debArch + ".deb"}
	default:
		return nil
	}
}

func isAllowedUpdateURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func (s *Service) DownloadUpdate() (string, error) {
	info, err := s.CheckForUpdate()
	if err != nil {
		return "", err
	}
	if !info.HasUpdate {
		return "", fmt.Errorf("no update is available")
	}
	if !info.CanInstall || info.AssetURL == "" || info.AssetName == "" {
		return "", fmt.Errorf("no installable update is available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	suffix := filepath.Ext(info.AssetName)
	temporaryFile, err := os.CreateTemp("", "ytgo-update-*"+suffix)
	if err != nil {
		return "", fmt.Errorf("failed to create update file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	removeTemporaryFile := true
	defer func() {
		_ = temporaryFile.Close()
		if removeTemporaryFile {
			_ = os.Remove(temporaryPath)
		}
	}()

	client := &http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !isAllowedUpdateURL(req.URL.String()) {
				return fmt.Errorf("update redirect points to an untrusted host")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, info.AssetURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create update download request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "YT-GO-update-downloader")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update download returned status code: %d", resp.StatusCode)
	}
	if resp.ContentLength > maxUpdateDownloadSize {
		return "", fmt.Errorf("update package is too large")
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporaryFile, hash), io.LimitReader(resp.Body, maxUpdateDownloadSize+1))
	if err != nil {
		return "", fmt.Errorf("failed to save update: %w", err)
	}
	if written > maxUpdateDownloadSize {
		return "", fmt.Errorf("update package is too large")
	}
	if info.AssetSize > 0 && written != info.AssetSize {
		return "", fmt.Errorf("downloaded update size does not match release metadata")
	}
	if expected, ok := updateDigest(info.AssetDigest); ok && !strings.EqualFold(expected, hex.EncodeToString(hash.Sum(nil))) {
		return "", fmt.Errorf("downloaded update checksum does not match release metadata")
	}
	if err := temporaryFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close update file: %w", err)
	}
	removeTemporaryFile = false
	return temporaryPath, nil
}

func updateDigest(digest string) (string, bool) {
	algorithm, value, ok := strings.Cut(strings.TrimSpace(digest), ":")
	if !ok || !strings.EqualFold(algorithm, "sha256") || len(value) != sha256.Size*2 {
		return "", false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", false
	}
	return value, true
}
