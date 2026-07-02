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

	builder := s.newYtdlpCommand().SetExecutable(ytdlpPath)
	builder.SetSeparateProcessGroup(true)
	builder.SetCancelMaxWait(3 * time.Second)
	applyFormatArgs(builder, req.Quality)
	builder.IgnoreConfig()

	if runtime.GOOS == "windows" {
		builder.WindowsFilenames()
	}

	outputTemplate := "%(title)s.%(ext)s"
	if settings.FilenameTemplate != "" {
		outputTemplate = settings.FilenameTemplate
	}
	if req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "" {
		outputTemplate = strings.TrimSpace(req.Options.FilenameTemplate)
	}
	builder.Newline()
	builder.Progress().ProgressDelta(0.5).ProgressTemplate(structuredProgressPrefix + "%()j")
	builder.Print("after_move:[YT-GO-OUTPUT]%(filepath)s")
	builder.Output(filepath.Join(req.OutputDir, outputTemplate))
	builder.NoPlaylist()

	if settings.RateLimit != "" {
		builder.LimitRate(settings.RateLimit)
	}
	if settings.Proxy != "" {
		builder.Proxy(settings.Proxy)
	}
	if settings.MergeOutputFormat != "" && shouldApplyMergeOutputFormat(req.Quality) {
		builder.MergeOutputFormat(settings.MergeOutputFormat)
	}
	if requiresAudioExtraction(req.Quality) {
		audioFmt := settings.AudioFormat
		if audioFmt == "" && req.Quality == "audio" {
			audioFmt = "mp3"
		}
		if audioFmt != "" {
			builder.AudioFormat(audioFmt)
		}
	}

	optSaveDescription := settings.SaveDescription
	optSaveThumbnail := settings.SaveThumbnail
	subtitleCfg := resolveSubtitleDownloadConfig(settings, req.Options)
	optEmbedChapters := settings.EmbedChapters
	optSponsorBlock := settings.SponsorBlock
	if req.Options != nil {
		if req.Options.SaveDescription != nil {
			optSaveDescription = *req.Options.SaveDescription
		}
		if req.Options.SaveThumbnail != nil {
			optSaveThumbnail = *req.Options.SaveThumbnail
		}
		if req.Options.EmbedChapters != nil {
			optEmbedChapters = *req.Options.EmbedChapters
		}
		if req.Options.SponsorBlock != nil {
			optSponsorBlock = *req.Options.SponsorBlock
		}
	}

	if optSaveDescription {
		builder.WriteDescription()
	}
	if optSaveThumbnail {
		builder.WriteThumbnail()
	}
	applySubtitleDownloadConfig(builder, subtitleCfg, req.URL)
	if optEmbedChapters {
		builder.EmbedChapters()
	}
	if optSponsorBlock {
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
