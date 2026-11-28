package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DownloadProfile stores the reusable choices for a download.
type DownloadProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Quality          string `json:"quality"`
	OutputDir        string `json:"outputDir"`
	FilenameTemplate string `json:"filenameTemplate"`
	VideoContainer   string `json:"videoContainer"`
	AudioFormat      string `json:"audioFormat"`
	WriteSubtitles   bool   `json:"writeSubtitles"`
	SubtitleLangs    string `json:"subtitleLangs"`
	EmbedSubtitles   bool   `json:"embedSubtitles"`
}

// DownloadProfiles stores the list and selection together. An empty selection uses app defaults.
type DownloadProfiles struct {
	Items      []DownloadProfile `json:"items"`
	SelectedID string            `json:"selectedId"`
}

var downloadProfileID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validateDownloadProfiles(profiles DownloadProfiles, language string) error {
	locale := NewI18n()
	locale.SetLang(Lang(language))
	ids := make(map[string]bool, len(profiles.Items))
	names := make(map[string]bool, len(profiles.Items))
	for _, profile := range profiles.Items {
		if !downloadProfileID.MatchString(profile.ID) || ids[profile.ID] {
			return fmt.Errorf(locale.T("profile.invalid_id"), profile.ID)
		}
		ids[profile.ID] = true
		name := strings.TrimSpace(profile.Name)
		if name != profile.Name || name == "" || utf8.RuneCountInString(name) > 80 || strings.ContainsFunc(name, unicode.IsControl) {
			return errors.New(locale.T("profile.invalid_name"))
		}
		key := strings.ToLower(name)
		if names[key] {
			return fmt.Errorf(locale.T("profile.duplicate_name"), name)
		}
		names[key] = true
		switch profile.Quality {
		case "best", "1080p", "720p", "480p", "360p", "audio":
		default:
			return fmt.Errorf(locale.T("profile.invalid_quality"), name, profile.Quality)
		}
		if err := validateDownloadFormats(locale, profile.VideoContainer, profile.AudioFormat); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if profile.OutputDir != strings.TrimSpace(profile.OutputDir) || strings.ContainsFunc(profile.OutputDir, unicode.IsControl) || validateSettingsOutputDir(profile.OutputDir) != nil {
			return fmt.Errorf(locale.T("profile.invalid_directory"), name)
		}
		if err := ValidateDownloadFilenameTemplate(profile.FilenameTemplate); err != nil {
			return fmt.Errorf(locale.T("profile.invalid_filename"), name)
		}
		if len(profile.SubtitleLangs) > 512 || strings.ContainsFunc(profile.SubtitleLangs, unicode.IsControl) {
			return fmt.Errorf(locale.T("profile.invalid_subtitle_langs"), name)
		}
	}
	if profiles.SelectedID != "" && !ids[profiles.SelectedID] {
		return errors.New(locale.T("profile.missing_selection"))
	}
	return nil
}

func validateDownloadFormats(locale *I18n, videoContainer, audioFormat string) error {
	switch videoContainer {
	case "", "auto", "mp4", "mkv", "webm":
	default:
		return fmt.Errorf(locale.T("download.invalid_container"), videoContainer)
	}
	switch audioFormat {
	case "", "default", "mp3", "m4a", "opus", "flac", "wav":
	default:
		return fmt.Errorf(locale.T("download.invalid_audio_format"), audioFormat)
	}
	return nil
}

// ValidateDownloadFilenameTemplate checks a filename-only template for profile storage and web downloads.
func ValidateDownloadFilenameTemplate(value string) error {
	switch {
	case value != strings.TrimSpace(value):
		return errors.New("filename template must not have leading or trailing whitespace")
	case len(value) > 512:
		return errors.New("filename template is too long")
	case value == "." || value == ".." || filepath.IsAbs(value) || filepath.VolumeName(value) != "":
		return errors.New("filename template must be a file name, not a path")
	case strings.ContainsAny(value, `/\`):
		return errors.New("filename template must not contain path separators")
	case strings.ContainsFunc(value, unicode.IsControl):
		return errors.New("filename template contains control characters")
	}
	return nil
}
