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

	denoProbe := probeDenoRuntime()
	if denoProbe.Found {
		version := denoProbe.Version
		if version == "" {
			version = denoProbe.Reason
		}
		if !denoProbe.Supported && version != "" {
			version = fmt.Sprintf("%s (need >= %d.0.0)", version, minimumDenoMajor)
		}
		status.JSRuntime = DepItem{Found: denoProbe.Supported, Version: version, Path: denoProbe.Path}
		status.JSRuntimeName = "deno"
	} else {
		nodeProbe := probeNodeRuntime()
		if nodeProbe.Found {
			version := nodeProbe.Version
			if version == "" {
				version = nodeProbe.Reason
			}
			if !nodeProbe.Supported && version != "" {
				version = fmt.Sprintf("%s (need >= %d.0.0)", version, minimumNodeMajor)
			}
			status.JSRuntime = DepItem{Found: nodeProbe.Supported, Version: version, Path: nodeProbe.Path}
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
