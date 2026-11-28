package core

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"YT-GO/internal/platform"
)

func (s *Service) buildDownloadCommand(ctx context.Context, req DownloadRequest, ytdlpPath string) (*exec.Cmd, error) {
	settings := s.GetSettings()
	options := req.mediaOptions
	if options == nil {
		resolved := resolveDownloadMediaOptions(settings, req)
		options = &resolved
	}

	builder := s.newYtdlpCommand().SetExecutable(ytdlpPath)
	builder.SetSeparateProcessGroup(true)
	builder.SetCancelMaxWait(3 * time.Second)
	applyFormatArgs(builder, req.Quality)
	builder.IgnoreConfig()

	if runtime.GOOS == "windows" {
		builder.WindowsFilenames()
	}

	builder.Newline()
	builder.Progress().ProgressDelta(0.5).ProgressTemplate(structuredProgressPrefix + "%()j")
	if isYouTubeURL(req.URL) {
		builder.Print("after_move:" + youtubeOutputPrefix + "%(.{id,extractor_key,filepath})j")
	} else {
		builder.Print("after_move:[YT-GO-OUTPUT]%(filepath)s")
	}
	builder.Output(filepath.Join(req.OutputDir, options.FilenameTemplate))
	builder.NoPlaylist()
	if options.SingleItem {
		builder.PlaylistItems("1")
	}

	if options.RateLimit != "" {
		builder.LimitRate(options.RateLimit)
	}
	if settings.Proxy != "" {
		builder.Proxy(settings.Proxy)
	}
	if options.VideoContainer != "" {
		builder.RecodeVideo(options.VideoContainer)
	}
	if options.MergeOutputFormat != "" {
		builder.MergeOutputFormat(options.MergeOutputFormat)
	}
	if options.AudioFormat != "" {
		builder.AudioFormat(options.AudioFormat)
	}

	if options.SaveDescription {
		builder.WriteDescription()
	}
	if options.SaveThumbnail {
		builder.WriteThumbnail()
	}
	applySubtitleDownloadConfig(builder, options.Subtitles, req.URL)
	if options.EmbedChapters {
		builder.EmbedChapters()
	}
	if options.SponsorBlock {
		builder.SponsorblockMark("all")
	}

	builder.IgnoreErrors()

	applyCookiesArgs(builder, settings)
	s.applyMediaCommand(builder)

	execCmd := builder.BuildCommand(ctx, req.URL)
	if execCmd == nil {
		return nil, fmt.Errorf("failed to build download command")
	}
	platform.ConfigureCmdWindow(execCmd, true)
	s.emitLog("[runDownload] exec: %s", strings.Join(execCmd.Args, " "))

	return execCmd, nil
}
