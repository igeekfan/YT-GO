package core

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestDownloadProfilesSurviveRestart(t *testing.T) {
	config := t.TempDir()
	t.Setenv("YTGO_DOWNLOAD_DIR", "")
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", config)
	case "darwin":
		t.Setenv("HOME", config)
	default:
		t.Setenv("XDG_CONFIG_HOME", config)
	}

	openService := func() *Service {
		t.Helper()
		service := NewService("test")
		if err := service.Startup(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = service.Close() })
		return service
	}
	saveProfiles := func(service *Service, profiles DownloadProfiles) Settings {
		t.Helper()
		data, err := json.Marshal(profiles)
		if err != nil {
			t.Fatal(err)
		}
		settings, err := service.ApplySettingsPatch(SettingsPatch{"downloadProfiles": data})
		if err != nil {
			t.Fatal(err)
		}
		return settings
	}

	service := openService()
	legacy := service.GetSettings()
	legacy.Quality = "480p"
	legacy.DownloadProfiles = DownloadProfiles{}
	if err := service.SaveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	defaults := service.GetSettings()
	if defaults.Quality != "480p" || defaults.DownloadProfiles.SelectedID != "" || len(defaults.DownloadProfiles.Items) != 0 {
		t.Fatalf("legacy defaults = %+v", defaults)
	}

	audio := DownloadProfile{ID: "audio", Name: "Audio archive", Quality: "audio", AudioFormat: "flac", OutputDir: filepath.Join(config, "archive")}
	video := DownloadProfile{ID: "video", Name: "学习资料", Quality: "720p", VideoContainer: "mkv", FilenameTemplate: "%(id)s.%(ext)s", WriteSubtitles: true, SubtitleLangs: "en,zh-Hans", EmbedSubtitles: true}
	saveProfiles(service, DownloadProfiles{Items: []DownloadProfile{audio, video}, SelectedID: audio.ID})
	video.Name = "学习资料 720p"
	want := DownloadProfiles{Items: []DownloadProfile{audio, video}, SelectedID: video.ID}
	saveProfiles(service, want)
	legacy.Theme = "light"
	if err := service.SaveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	service = openService()
	got := service.GetSettings()
	if got.Quality != "480p" || got.Theme != "light" || !reflect.DeepEqual(want, got.DownloadProfiles) {
		t.Fatalf("unexpected settings after restart: %+v", got)
	}

	deleted := saveProfiles(service, DownloadProfiles{Items: []DownloadProfile{audio}})
	if deleted.DownloadProfiles.SelectedID != "" || !slices.Equal(deleted.DownloadProfiles.Items, []DownloadProfile{audio}) {
		t.Fatalf("deleting the selected profile did not restore the default selection: %+v", deleted.DownloadProfiles)
	}
	if err := service.ResetSettings(); err != nil {
		t.Fatal(err)
	}
	if got := service.GetSettings().DownloadProfiles; got.SelectedID != "" || len(got.Items) != 0 {
		t.Fatalf("reset profiles = %+v", got)
	}
}

func TestInvalidDownloadProfilesAreRejected(t *testing.T) {
	valid := DownloadProfile{ID: "valid", Name: "Archive", Quality: "720p"}
	cases := []struct {
		name string
		edit func(*DownloadProfiles)
	}{
		{"missing name", func(profiles *DownloadProfiles) { profiles.Items[0].Name = "  " }},
		{"duplicate name", func(profiles *DownloadProfiles) {
			profiles.Items = append(profiles.Items, DownloadProfile{ID: "another", Name: "archive", Quality: "best"})
		}},
		{"duplicate ID", func(profiles *DownloadProfiles) { profiles.Items = append(profiles.Items, valid) }},
		{"missing selected profile", func(profiles *DownloadProfiles) { profiles.SelectedID = "missing" }},
		{"invalid quality", func(profiles *DownloadProfiles) { profiles.Items[0].Quality = "f:22" }},
		{"unsupported container", func(profiles *DownloadProfiles) { profiles.Items[0].VideoContainer = "mov" }},
		{"unsupported audio format", func(profiles *DownloadProfiles) { profiles.Items[0].AudioFormat = "ogg" }},
		{"filename path", func(profiles *DownloadProfiles) { profiles.Items[0].FilenameTemplate = "clips/%(title)s.%(ext)s" }},
		{"invalid directory", func(profiles *DownloadProfiles) { profiles.Items[0].OutputDir = "clips\narchive" }},
		{"invalid subtitle languages", func(profiles *DownloadProfiles) { profiles.Items[0].SubtitleLangs = strings.Repeat("a", 513) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			profiles := DownloadProfiles{Items: []DownloadProfile{valid}, SelectedID: valid.ID}
			test.edit(&profiles)
			if err := ValidateSettings(Settings{MaxConcurrent: 3, Language: "en-US", DownloadProfiles: profiles}); err == nil {
				t.Fatal("invalid profile was accepted")
			}
		})
	}
}

func TestDownloadCommandUsesExplicitOptions(t *testing.T) {
	service := newPersistenceTestServiceWithoutRuntime(t)
	outputDir := t.TempDir()
	settings := service.GetSettings()
	settings.OutputDir = outputDir
	settings.AudioFormat = "wav"
	settings.MergeOutputFormat = "mp4"
	settings.FilenameTemplate = "%(title)s.%(ext)s"
	settings.SubtitleLangs = "de"
	if err := service.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	check := func(req DownloadRequest, want, unwanted []string) {
		t.Helper()
		req.URL = "https://example.com/clip"
		req.OutputDir = outputDir
		command, err := service.buildDownloadCommand(context.Background(), req, "yt-dlp")
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Join(command.Args[1:], " ")
		for _, part := range want {
			if !strings.Contains(args, part) {
				t.Errorf("missing %q in %s", part, args)
			}
		}
		for _, part := range unwanted {
			if strings.Contains(args, part) {
				t.Errorf("unexpected %q in %s", part, args)
			}
		}
	}
	write, embed := true, true
	check(DownloadRequest{Quality: "audio", Options: &DownloadOptions{
		AudioFormat: "flac", FilenameTemplate: "archive-%(id)s.%(ext)s", SingleItem: true,
		WriteSubtitles: &write, SubtitleLangs: "en", AutoSubtitleLangs: "en", EmbedSubtitles: &embed,
	}}, []string{
		"--no-playlist", "--playlist-items 1", "--extract-audio", "--audio-format flac",
		"--write-subs", "--write-auto-subs", "--embed-subs", "--sub-langs en", filepath.Join(outputDir, "archive-%(id)s.%(ext)s"),
	}, []string{"--audio-format wav", "--merge-output-format", "--sub-langs de"})
	check(DownloadRequest{Quality: "f:22", Options: &DownloadOptions{VideoContainer: "mkv"}},
		[]string{"--format 22", "--recode-video mkv"}, []string{"--merge-output-format", "--playlist-items", "--write-subs"})
	check(DownloadRequest{Quality: "audio", Options: &DownloadOptions{AudioFormat: "default", VideoContainer: "mkv"}},
		[]string{"--audio-format mp3"}, []string{"--audio-format wav", "--recode-video"})
	check(DownloadRequest{Quality: "best", Options: &DownloadOptions{VideoContainer: "auto"}},
		[]string{"--no-playlist"}, []string{"--merge-output-format", "--recode-video"})
	check(DownloadRequest{Quality: "720p"}, []string{"[height<=720]", "--merge-output-format mp4"},
		[]string{"--playlist-items", "--recode-video"})
}

func TestDesktopQuickDownloadUsesQueue(t *testing.T) {
	service := NewService("test")
	service.downloadDir = t.TempDir()
	service.limiter.SetLimit(1)
	if err := service.limiter.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = service.Close()
		service.limiter.Release()
	})
	req := DownloadRequest{URL: "https://example.com/clip", Quality: "720p", OutputDir: t.TempDir(), Options: &DownloadOptions{SingleItem: true}}
	for _, invalid := range []string{"https:///clip", "ftp://example.com/clip", "See https://example.com/clip", "https://example.com/one https://example.com/two"} {
		rejected := req
		rejected.URL = invalid
		if _, err := service.StartDownload(rejected); err == nil {
			t.Fatalf("quick download accepted %q", invalid)
		}
	}
	id, err := service.StartDownload(req)
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.GetDownload(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.URL != req.URL || task.OutputDir != service.downloadDir || task.Quality != req.Quality || task.Status != "pending" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if err := service.CancelDownload(id); err != nil {
		t.Fatal(err)
	}
}
