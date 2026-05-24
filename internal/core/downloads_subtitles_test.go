package core

import (
	"context"
	"strings"
	"testing"
)

func TestResolveSubtitleDownloadConfigClearsDisabledGroups(t *testing.T) {
	writeSubtitles := true
	writeManualSubs := false
	writeAutoSubs := true
	embedSubtitles := true

	settings := Settings{
		WriteSubtitles: true,
		SubtitleLangs:  "zh-Hans",
		EmbedSubtitles: false,
	}
	opts := &DownloadOptions{
		WriteSubtitles:    &writeSubtitles,
		WriteManualSubs:   &writeManualSubs,
		WriteAutoSubs:     &writeAutoSubs,
		AutoSubtitleLangs: "en-zh-Hans",
		EmbedSubtitles:    &embedSubtitles,
	}

	cfg := resolveSubtitleDownloadConfig(settings, opts)
	if cfg.SubtitleLangs != "" {
		t.Fatalf("expected manual subtitle langs to be cleared, got %q", cfg.SubtitleLangs)
	}
	if cfg.AutoSubtitleLangs != "en-zh-Hans" {
		t.Fatalf("expected auto subtitle langs to be preserved, got %q", cfg.AutoSubtitleLangs)
	}
	if !cfg.EmbedSubtitles {
		t.Fatal("expected embed subtitles to follow request option")
	}
}

func TestApplySubtitleDownloadConfigAutoOnlyYouTube(t *testing.T) {
	builder := NewService("test").newYtdlpCommand().SetExecutable("yt-dlp")
	applySubtitleDownloadConfig(builder, subtitleDownloadConfig{
		WriteSubtitles:    true,
		WriteManualSubs:   false,
		WriteAutoSubs:     true,
		AutoSubtitleLangs: "en-zh-Hans",
		EmbedSubtitles:    true,
		ExplicitMode:      true,
	}, "https://www.youtube.com/watch?v=_CKmuMFCxQ8")

	built := builder.BuildCommand(context.Background(), "https://www.youtube.com/watch?v=_CKmuMFCxQ8")
	args := strings.Join(built.Args, " ")

	if strings.Contains(args, "--write-subs") {
		t.Fatalf("did not expect manual subtitles flag in auto-only command: %s", args)
	}
	for _, want := range []string{
		"--write-auto-subs",
		"--sub-langs en-zh-Hans",
		"--sleep-requests 0.75",
		"--sleep-subtitles 1.5",
		"--retries 5",
		"--extractor-retries 5",
		"--fragment-retries 5",
		"--embed-subs",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected command to contain %q, got: %s", want, args)
		}
	}
}
