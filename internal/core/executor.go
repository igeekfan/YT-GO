package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// toUTF8 converts bytes from various encodings (GB18030, Windows-1252) to UTF-8.
func toUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), b)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	decoded, _, err = transform.Bytes(charmap.Windows1252.NewDecoder(), b)
	if err == nil {
		return string(decoded)
	}
	return strings.ToValidUTF8(string(b), "\ufffd")
}

// ytdlpCmd builds a basic yt-dlp command with context and environment.
func (s *Service) ytdlpCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.ytdlpPath, args...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	if s.hooks.HideCommand != nil {
		s.hooks.HideCommand(cmd)
	}
	return cmd
}

// ytdlpMediaCmd builds a yt-dlp command with JS runtime auto-detection.
func (s *Service) ytdlpMediaCmd(ctx context.Context, args ...string) *exec.Cmd {
	if jsRuntime := getPreferredJSRuntime(); jsRuntime != "" {
		args = append([]string{"--js-runtimes", jsRuntime}, args...)
	}
	return s.ytdlpCmd(ctx, args...)
}

// logCmd emits a log message with the full command line.
func (s *Service) logCmd(tag string, cmd *exec.Cmd) {
	s.emitLog("[%s] exec: %s", tag, strings.Join(cmd.Args, " "))
}

// appendCookiesArgs appends cookie-related arguments based on settings.
func appendCookiesArgs(args []string, settings Settings) []string {
	if settings.CookiesFrom != "" {
		return append(args, "--cookies-from-browser", settings.CookiesFrom)
	}
	if settings.CookiesFile != "" {
		return append(args, "--cookies", settings.CookiesFile)
	}
	return args
}

// qualityArgs maps a quality preset to yt-dlp format selection arguments.
func qualityArgs(quality string) []string {
	if len(quality) > 3 && quality[:3] == "fa:" {
		return []string{"-f", quality[3:], "-x"}
	}
	if len(quality) > 3 && quality[:3] == "fv:" {
		return []string{"-f", quality[3:]}
	}
	if len(quality) > 2 && quality[:2] == "f:" {
		return []string{"-f", quality[2:]}
	}
	switch quality {
	case "1080p":
		return []string{"-f", "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]"}
	case "720p":
		return []string{"-f", "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=720]+bestaudio/best[height<=720]"}
	case "480p":
		return []string{"-f", "bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=480]+bestaudio/best[height<=480]"}
	case "360p":
		return []string{"-f", "bestvideo[height<=360][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=360]+bestaudio/best[height<=360]"}
	case "audio":
		return []string{"-f", "bestaudio/best", "-x"}
	default:
		return []string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best"}
	}
}

// findYtDlp searches for yt-dlp in PATH, app directory, and common install locations.
func (s *Service) findYtDlp() string {
	candidates := []string{"yt-dlp", "yt-dlp.exe"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		for _, name := range candidates {
			path := filepath.Join(execDir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	var extraDirs []string
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		wingetPackages := filepath.Join(localApp, "Microsoft", "WinGet", "Packages")
		if entries, err := os.ReadDir(wingetPackages); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && strings.HasPrefix(entry.Name(), "yt-dlp.yt-dlp") {
					extraDirs = append(extraDirs, filepath.Join(wingetPackages, entry.Name()))
				}
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		extraDirs = append(extraDirs, filepath.Join(home, "scoop", "shims"))
	}
	for _, dir := range extraDirs {
		for _, name := range candidates {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

// CheckYtDlp verifies yt-dlp availability and returns its status.
func (s *Service) CheckYtDlp() YtDlpStatus {
	if s.ytdlpPath == "" {
		s.ytdlpPath = s.findYtDlp()
	}
	if s.ytdlpPath == "" {
		return YtDlpStatus{Available: false}
	}
	cmd := exec.Command(s.ytdlpPath, "--version")
	if s.hooks.HideCommand != nil {
		s.hooks.HideCommand(cmd)
	}
	out, err := cmd.Output()
	if err != nil {
		return YtDlpStatus{Available: false}
	}
	return YtDlpStatus{Available: true, Version: strings.TrimSpace(string(out)), Path: s.ytdlpPath}
}

// UpdateYtDlp runs yt-dlp self-update.
func (s *Service) UpdateYtDlp() (string, error) {
	if s.ytdlpPath == "" {
		return "", fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := s.ytdlpCmd(ctx, "-U")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(toUTF8(out))
	if err != nil {
		return output, fmt.Errorf("update failed: %w", err)
	}
	return output, nil
}

// CheckYtDlpVersion compares the local yt-dlp version with the latest GitHub release.
func (s *Service) CheckYtDlpVersion() (YtDlpVersionCheck, error) {
	if s.ytdlpPath == "" {
		return YtDlpVersionCheck{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := s.ytdlpCmd(ctx, "--version").CombinedOutput()
	if err != nil {
		return YtDlpVersionCheck{}, fmt.Errorf("failed to get version: %w", err)
	}
	currentVersion := strings.TrimSpace(toUTF8(out))

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Get("https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest")
	if err != nil {
		return YtDlpVersionCheck{CurrentVersion: currentVersion}, fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return YtDlpVersionCheck{CurrentVersion: currentVersion}, fmt.Errorf("failed to parse response: %w", err)
	}
	tagName, _ := data["tag_name"].(string)
	latestVersion := strings.TrimPrefix(tagName, "v")
	return YtDlpVersionCheck{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		IsLatest:       currentVersion == latestVersion,
	}, nil
}
