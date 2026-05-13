package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lrstanley/go-ytdlp"
	"YT-GO/internal/platform"
)

func (s *Service) GetDiagnosticInfo() DiagnosticInfo {
	ytdlpPath := s.resolveYtDlp()
	info := DiagnosticInfo{YtDlpPath: ytdlpPath, YtDlpFound: ytdlpPath != "", AppVersion: s.appVersion}

	if ytdlpPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result, err := ytdlp.New().SetExecutable(ytdlpPath).Version(ctx)
		if err != nil {
			info.Error = fmt.Sprintf("version check failed: %v", err)
		} else {
			info.YtDlpVersion = strings.TrimSpace(result.Stdout)
		}

		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		helpResult, helpErr := ytdlp.New().SetExecutable(ytdlpPath).Run(ctx2, "--help")
		if helpErr != nil {
			info.Error = fmt.Sprintf("help command failed: %v", helpErr)
		} else {
			helpOutput := helpResult.Stdout + helpResult.Stderr
			if strings.Contains(helpOutput, "youtube") || strings.Contains(helpOutput, "Usage:") {
				info.TestOutput = "yt-dlp is working correctly"
			} else {
				truncated := helpOutput
				if len(truncated) > 200 {
					truncated = truncated[:200]
				}
				info.TestOutput = fmt.Sprintf("unexpected output (first 200 chars): %s", truncated)
			}
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

	// yt-dlp - use go-ytdlp Install with download disabled to resolve from system.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resolved, err := ytdlp.Install(ctx, &ytdlp.InstallOptions{
		DisableDownload:      true,
		DisableSystem:        false,
		AllowVersionMismatch: true,
	})
	if err == nil && resolved.Executable != "" {
		status.YtDlp = DepItem{Found: true, Version: resolved.Version, Path: resolved.Executable}
	}

	// ffmpeg - use go-ytdlp InstallFFmpeg with download disabled.
	ffResolved, ffErr := ytdlp.InstallFFmpeg(ctx, &ytdlp.InstallFFmpegOptions{
		DisableDownload: true,
		DisableSystem:   false,
	})
	if ffErr == nil && ffResolved.Executable != "" {
		status.FFmpeg = DepItem{Found: true, Version: ffResolved.Version, Path: ffResolved.Executable}
	} else {
		// Fallback: manual PATH lookup
		ffPath, ffVersion, ffFound := detectFFmpegFallback()
		status.FFmpeg = DepItem{Found: ffFound, Version: ffVersion, Path: ffPath}
	}

	denoProbe := probeDenoRuntime(s.i18n)
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
		nodeProbe := probeNodeRuntime(s.i18n)
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

// detectFFmpeg tries go-ytdlp's InstallFFmpeg first, then falls back to manual lookup.
func detectFFmpeg() (path string, version string, found bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resolved, err := ytdlp.InstallFFmpeg(ctx, &ytdlp.InstallFFmpegOptions{
		DisableDownload: true,
		DisableSystem:   false,
	})
	if err == nil && resolved.Executable != "" {
		return resolved.Executable, resolved.Version, true
	}
	return detectFFmpegFallback()
}

// detectFFmpegFallback does a manual PATH lookup for ffmpeg.
func detectFFmpegFallback() (path string, version string, found bool) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ffCmd := exec.CommandContext(ctx, ffmpegPath, "-version")
	platform.HideCmdWindow(ffCmd)
	out, err := ffCmd.CombinedOutput()
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
