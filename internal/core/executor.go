package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/go-ytdlp"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// toUTF8 converts bytes from various encodings (GB18030, Windows-1252) to UTF-8.
//
// yt-dlp's stderr buffer can mix encodings: yt-dlp's own Python output is UTF-8
// (we force PYTHONIOENCODING=utf-8), but subprocesses it spawns (ffmpeg, ffprobe)
// inherit the Windows OEM codepage and emit GBK/CP936 on Chinese systems. Decoding
// the whole blob as one unit corrupts the UTF-8 lines once any GBK byte appears,
// so we split on newlines and decode each line independently.
func toUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	if !bytes.ContainsAny(b, "\r\n") {
		return decodeLine(b)
	}
	var out strings.Builder
	out.Grow(len(b))
	start := 0
	for i := 0; i < len(b); i++ {
		if c := b[i]; c == '\n' || c == '\r' {
			out.WriteString(decodeLine(b[start:i]))
			out.WriteByte(c)
			start = i + 1
		}
	}
	if start < len(b) {
		out.WriteString(decodeLine(b[start:]))
	}
	return out.String()
}

func decodeLine(b []byte) string {
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

// newYtdlpCommand creates a new go-ytdlp Command with common environment settings.
func (s *Service) newYtdlpCommand() *ytdlp.Command {
	cmd := ytdlp.New()
	cmd.SetEnvVar("PYTHONIOENCODING", "utf-8")
	cmd.SetEnvVar("PYTHONUTF8", "1")
	// Disable Python stdout/stderr buffering so progress lines stream
	// in real time when piped to our lineWriter (instead of arriving in 4KB chunks).
	cmd.SetEnvVar("PYTHONUNBUFFERED", "1")
	return cmd
}

func applyResolvedFFmpegLocation(cmd *ytdlp.Command, ffmpegPath string) *ytdlp.Command {
	if strings.TrimSpace(ffmpegPath) == "" {
		return cmd
	}
	cmd.FFmpegLocation(ffmpegPath)
	return cmd
}

// applyMediaCommand applies JS runtime auto-detection and common settings to the command.
func (s *Service) applyMediaCommand(cmd *ytdlp.Command) *ytdlp.Command {
	if jsRuntime := getPreferredJSRuntime(s.i18n); jsRuntime != "" {
		cmd.JsRuntimes(jsRuntime)
	}
	if ffmpegPath, _, found := detectFFmpeg(); found {
		applyResolvedFFmpegLocation(cmd, ffmpegPath)
	}
	return cmd
}

// logCmd emits a log message with the full command line.
func (s *Service) logCmd(tag string, cmd *ytdlp.Command, ctx context.Context, args ...string) {
	built := cmd.BuildCommand(ctx, args...)
	s.emitLog("[%s] exec: %s", tag, strings.Join(built.Args, " "))
}

// applyCookiesArgs applies cookie-related settings to the command builder.
func applyCookiesArgs(cmd *ytdlp.Command, settings Settings) *ytdlp.Command {
	if settings.CookiesFrom != "" {
		cmd.CookiesFromBrowser(settings.CookiesFrom)
	} else if settings.CookiesFile != "" {
		cmd.Cookies(settings.CookiesFile)
	}
	return cmd
}

// applyFormatArgs maps a quality preset to the go-ytdlp Format builder method.
func applyFormatArgs(cmd *ytdlp.Command, quality string) *ytdlp.Command {
	if len(quality) > 3 && quality[:3] == "fa:" {
		cmd.Format(quality[3:]).ExtractAudio()
		return cmd
	}
	if len(quality) > 3 && quality[:3] == "fv:" {
		cmd.Format(quality[3:])
		return cmd
	}
	if len(quality) > 2 && quality[:2] == "f:" {
		cmd.Format(quality[2:])
		return cmd
	}
	switch quality {
	case "1080p":
		cmd.Format("bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]")
	case "720p":
		cmd.Format("bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=720]+bestaudio/best[height<=720]")
	case "480p":
		cmd.Format("bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=480]+bestaudio/best[height<=480]")
	case "360p":
		cmd.Format("bestvideo[height<=360][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=360]+bestaudio/best[height<=360]")
	case "audio":
		cmd.Format("bestaudio/best").ExtractAudio()
	default:
		cmd.Format("bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best")
	}
	return cmd
}

// CheckYtDlp verifies yt-dlp availability and returns its status.
func (s *Service) CheckYtDlp() YtDlpStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var execPath string
	if s.ytdlpPath != "" {
		if info, err := os.Stat(s.ytdlpPath); err == nil && !info.IsDir() {
			execPath = s.ytdlpPath
			s.emitLog("yt-dlp: using YTGO_YTDLP_PATH=%s", execPath)
		} else {
			s.emitLog("yt-dlp: YTGO_YTDLP_PATH=%s not found, err=%v", s.ytdlpPath, err)
		}
	}
	if execPath == "" {
		resolved, err := ytdlp.Install(ctx, &ytdlp.InstallOptions{
			DisableDownload:      true,
			DisableSystem:        false,
			AllowVersionMismatch: true,
		})
		if err == nil && resolved.Executable != "" {
			execPath = resolved.Executable
			s.emitLog("yt-dlp: found via go-ytdlp=%s", execPath)
		} else {
			s.emitLog("yt-dlp: go-ytdlp install failed, err=%v", err)
		}
	}
	if execPath == "" {
		if p, err := exec.LookPath("yt-dlp"); err == nil {
			execPath = p
			s.emitLog("yt-dlp: found via LookPath=%s", execPath)
		} else {
			s.emitLog("yt-dlp: LookPath failed, err=%v", err)
		}
	}
	if execPath == "" {
		s.emitLog("yt-dlp: not found by any method")
		return YtDlpStatus{Available: false}
	}
	versionResult, vErr := ytdlp.New().SetExecutable(execPath).Version(ctx)
	if vErr != nil {
		s.emitLog("yt-dlp: version check failed, err=%v", vErr)
		return YtDlpStatus{Available: false}
	}
	return YtDlpStatus{Available: true, Version: strings.TrimSpace(versionResult.Stdout), Path: execPath}
}

// UpdateYtDlp runs yt-dlp self-update via go-ytdlp.
func (s *Service) UpdateYtDlp() (string, error) {
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" {
		return "", fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	result, err := ytdlp.New().SetExecutable(ytdlpPath).Update(ctx)
	output := ""
	if result != nil {
		output = strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	}
	output = toUTF8([]byte(output))
	if err != nil {
		return output, fmt.Errorf("update failed: %w", err)
	}
	return output, nil
}

// CheckYtDlpVersion compares the local yt-dlp version with the latest GitHub release.
func (s *Service) CheckYtDlpVersion() (YtDlpVersionCheck, error) {
	ytdlpPath := s.resolveYtDlp()
	if ytdlpPath == "" {
		return YtDlpVersionCheck{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := ytdlp.New().SetExecutable(ytdlpPath).Version(ctx)
	if err != nil {
		return YtDlpVersionCheck{}, fmt.Errorf("failed to get version: %w", err)
	}
	currentVersion := strings.TrimSpace(result.Stdout)

	httpClient := &http.Client{Timeout: 15 * time.Second}
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

// InstallYtDlp downloads yt-dlp if not already installed.
func (s *Service) InstallYtDlp() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resolved, err := ytdlp.Install(ctx, &ytdlp.InstallOptions{
		DisableDownload:      false,
		DisableSystem:        false,
		AllowVersionMismatch: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to install yt-dlp: %w", err)
	}
	return resolved.Executable, nil
}
