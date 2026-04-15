package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DiagnosticInfo contains debug information about the application state.
type DiagnosticInfo struct {
	YtDlpPath     string `json:"ytdlpPath"`
	YtDlpVersion  string `json:"ytdlpVersion"`
	YtDlpFound    bool   `json:"ytdlpFound"`
	FFmpegPath    string `json:"ffmpegPath"`
	FFmpegVersion string `json:"ffmpegVersion"`
	FFmpegFound   bool   `json:"ffmpegFound"`
	NodeVersion   string `json:"nodeVersion"`
	AppVersion    string `json:"appVersion"`
	TestOutput    string `json:"testOutput"`
	Error         string `json:"error"`
}

// GetDiagnosticInfo returns debug information to help troubleshoot issues.
func (a *App) GetDiagnosticInfo() DiagnosticInfo {
	info := DiagnosticInfo{
		YtDlpPath:  a.ytdlpPath,
		YtDlpFound: a.ytdlpPath != "",
		AppVersion: WailsInfo.Info.ProductVersion,
	}

	if a.ytdlpPath == "" {
		a.ytdlpPath = a.findYtDlp()
		info.YtDlpPath = a.ytdlpPath
		info.YtDlpFound = a.ytdlpPath != ""
	}

	if a.ytdlpPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := a.ytdlpCmd(ctx, "--version").CombinedOutput()
		if err != nil {
			info.Error = fmt.Sprintf("version check failed: %v", err)
		} else {
			info.YtDlpVersion = strings.TrimSpace(string(out))
		}

		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		testOut, err := a.ytdlpCmd(ctx2, "--help").CombinedOutput()
		if err != nil {
			info.Error = fmt.Sprintf("help command failed: %v", err)
		} else if strings.Contains(string(testOut), "youtube") || strings.Contains(string(testOut), "Usage:") {
			info.TestOutput = "yt-dlp is working correctly"
		} else {
			info.TestOutput = fmt.Sprintf("unexpected output (first 200 chars): %s", string(testOut)[:min(200, len(testOut))])
		}
	} else {
		info.Error = "yt-dlp not found in PATH or common installation directories"
	}

	info.FFmpegPath, info.FFmpegVersion, info.FFmpegFound = detectFFmpeg()
	info.NodeVersion = getNodeVersion()

	return info
}

func detectFFmpeg() (path string, version string, found bool) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffmpegPath, "-version").CombinedOutput()
	if err != nil {
		return ffmpegPath, "", true
	}
	output := strings.TrimSpace(string(out))
	if lines := strings.SplitN(output, "\n", 2); len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "ffmpeg version ") {
			parts := strings.Fields(firstLine)
			if len(parts) >= 3 {
				return ffmpegPath, parts[2], true
			}
		}
		return ffmpegPath, firstLine, true
	}
	return ffmpegPath, output, true
}
