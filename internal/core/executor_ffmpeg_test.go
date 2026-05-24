package core

import (
	"context"
	"strings"
	"testing"
)

func TestApplyResolvedFFmpegLocationAddsFlag(t *testing.T) {
	builder := NewService("test").newYtdlpCommand().SetExecutable("yt-dlp")
	applyResolvedFFmpegLocation(builder, `E:\tools\ffmpeg\bin\ffmpeg.exe`)

	built := builder.BuildCommand(context.Background(), "https://example.com/video")
	args := strings.Join(built.Args, " ")

	if !strings.Contains(args, `--ffmpeg-location E:\tools\ffmpeg\bin\ffmpeg.exe`) {
		t.Fatalf("expected command to contain ffmpeg location flag, got: %s", args)
	}
}

func TestApplyResolvedFFmpegLocationSkipsEmptyPath(t *testing.T) {
	builder := NewService("test").newYtdlpCommand().SetExecutable("yt-dlp")
	applyResolvedFFmpegLocation(builder, "")

	built := builder.BuildCommand(context.Background(), "https://example.com/video")
	args := strings.Join(built.Args, " ")

	if strings.Contains(args, "--ffmpeg-location") {
		t.Fatalf("did not expect ffmpeg location flag for empty path, got: %s", args)
	}
}
