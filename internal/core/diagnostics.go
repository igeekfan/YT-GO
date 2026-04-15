package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func (s *Service) GetDiagnosticInfo() DiagnosticInfo {
	info := DiagnosticInfo{YtDlpPath: s.ytdlpPath, YtDlpFound: s.ytdlpPath != "", AppVersion: s.appVersion}
	if s.ytdlpPath == "" {
		s.ytdlpPath = s.findYtDlp()
		info.YtDlpPath = s.ytdlpPath
		info.YtDlpFound = s.ytdlpPath != ""
	}
	if s.ytdlpPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := s.ytdlpCmd(ctx, "--version").CombinedOutput()
		if err != nil {
			info.Error = fmt.Sprintf("version check failed: %v", err)
		} else {
			info.YtDlpVersion = strings.TrimSpace(string(out))
		}
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		testOut, err := s.ytdlpCmd(ctx2, "--help").CombinedOutput()
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

func (s *Service) GetDepStatus() DepStatus {
	var status DepStatus

	// yt-dlp
	ytdlpPath := s.ytdlpPath
	if ytdlpPath == "" {
		ytdlpPath = s.findYtDlp()
	}
	if ytdlpPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, ytdlpPath, "--version").CombinedOutput()
		if err == nil {
			status.YtDlp = DepItem{Found: true, Version: strings.TrimSpace(string(out)), Path: ytdlpPath}
		} else {
			status.YtDlp = DepItem{Found: true, Path: ytdlpPath}
		}
	}

	// ffmpeg
	ffPath, ffVersion, ffFound := detectFFmpeg()
	status.FFmpeg = DepItem{Found: ffFound, Version: ffVersion, Path: ffPath}

	// JS runtime: try deno first, then node
	denoPath, err := exec.LookPath("deno")
	if err == nil && denoPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, runErr := exec.CommandContext(ctx, denoPath, "--version").CombinedOutput()
		ver := ""
		if runErr == nil {
			firstLine := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
			if strings.HasPrefix(firstLine, "deno ") {
				ver = strings.TrimPrefix(firstLine, "deno ")
			} else {
				ver = firstLine
			}
		}
		status.JSRuntime = DepItem{Found: true, Version: ver, Path: denoPath}
		status.JSRuntimeName = "deno"
	} else {
		nodePath, nodeErr := exec.LookPath("node")
		if nodeErr == nil && nodePath != "" && isNodeVersionSufficient(nodePath) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			out, runErr := exec.CommandContext(ctx, nodePath, "-v").CombinedOutput()
			ver := ""
			if runErr == nil {
				ver = strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
			}
			status.JSRuntime = DepItem{Found: true, Version: ver, Path: nodePath}
			status.JSRuntimeName = "node"
		}
	}

	return status
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
