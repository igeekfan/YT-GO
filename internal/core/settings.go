package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) GetSettings() Settings {
	defaults := Settings{
		OutputDir:         s.GetDefaultDownloadDir(),
		Quality:           "best",
		Language:          "zh-CN",
		Theme:             "dark",
		Proxy:             "",
		RateLimit:         "",
		MaxConcurrent:     3,
		Notifications:     true,
		SaveDescription:   false,
		SaveThumbnail:     false,
		WriteSubtitles:    false,
		SubtitleLangs:     "",
		EmbedSubtitles:    false,
		EmbedChapters:     false,
		SponsorBlock:      false,
		FilenameTemplate:  "",
		MergeOutputFormat: "",
		AudioFormat:       "",
	}
	if s.db == nil {
		return defaults
	}
	var rec SettingsRecord
	if err := s.db.First(&rec, 1).Error; err != nil {
		return defaults
	}
	if rec.OutputDir != "" {
		defaults.OutputDir = rec.OutputDir
	}
	if rec.Quality != "" {
		defaults.Quality = rec.Quality
	}
	if rec.Language != "" {
		defaults.Language = rec.Language
	}
	if rec.Theme != "" {
		defaults.Theme = rec.Theme
	}
	defaults.Proxy = rec.Proxy
	defaults.RateLimit = rec.RateLimit
	if rec.MaxConcurrent > 0 {
		defaults.MaxConcurrent = rec.MaxConcurrent
	}
	defaults.Notifications = rec.Notifications
	defaults.SaveDescription = rec.SaveDescription
	defaults.SaveThumbnail = rec.SaveThumbnail
	defaults.WriteSubtitles = rec.WriteSubtitles
	defaults.SubtitleLangs = rec.SubtitleLangs
	defaults.EmbedSubtitles = rec.EmbedSubtitles
	defaults.EmbedChapters = rec.EmbedChapters
	defaults.SponsorBlock = rec.SponsorBlock
	defaults.FilenameTemplate = rec.FilenameTemplate
	defaults.MergeOutputFormat = rec.MergeOutputFormat
	defaults.AudioFormat = rec.AudioFormat
	defaults.CookiesFrom = rec.CookiesFrom
	defaults.CookiesFile = rec.CookiesFile
	return defaults
}

func (s *Service) IsFirstRun() bool {
	if s.db == nil {
		return true
	}
	var rec SettingsRecord
	if err := s.db.First(&rec, 1).Error; err != nil {
		return true
	}
	return false
}

func (s *Service) NeedsCookieConfig() bool {
	settings := s.GetSettings()
	return settings.CookiesFrom == "" && settings.CookiesFile == "" && settings.Proxy == ""
}

func (s *Service) SaveSettings(settings Settings) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}
	// Validate OutputDir: reject empty, root, or sensitive system paths.
	if settings.OutputDir != "" {
		cleaned := filepath.Clean(settings.OutputDir)
		lower := strings.ToLower(cleaned)
		sensitivePaths := []string{"/", "/etc", "/proc", "/sys", "/dev", "/boot", "/usr", "/bin", "/sbin", "/var", "c:\\", "c:/", "c:\\windows", "c:\\program files"}
		for _, sp := range sensitivePaths {
			if lower == sp || lower == sp+"/" || lower == sp+"\\" {
				return fmt.Errorf("output directory cannot be a system path: %s", cleaned)
			}
		}
	}

	// Validate Proxy: only allow http/https/socks5 schemes to prevent SSRF.
	if settings.Proxy != "" {
		proxyURL, err := url.Parse(settings.Proxy)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		scheme := strings.ToLower(proxyURL.Scheme)
		if scheme != "http" && scheme != "https" && scheme != "socks5" && scheme != "socks5h" {
			return fmt.Errorf("proxy scheme must be http, https, or socks5, got: %s", proxyURL.Scheme)
		}
	}

	// Validate MaxConcurrent: enforce range [1, 10].
	if settings.MaxConcurrent < 1 || settings.MaxConcurrent > 10 {
		return fmt.Errorf("max concurrent downloads must be between 1 and 10, got: %d", settings.MaxConcurrent)
	}

	rec := SettingsRecord{
		ID:                1,
		OutputDir:         settings.OutputDir,
		Quality:           settings.Quality,
		Language:          settings.Language,
		Theme:             settings.Theme,
		Proxy:             settings.Proxy,
		RateLimit:         settings.RateLimit,
		MaxConcurrent:     settings.MaxConcurrent,
		Notifications:     settings.Notifications,
		SaveDescription:   settings.SaveDescription,
		SaveThumbnail:     settings.SaveThumbnail,
		WriteSubtitles:    settings.WriteSubtitles,
		SubtitleLangs:     settings.SubtitleLangs,
		EmbedSubtitles:    settings.EmbedSubtitles,
		EmbedChapters:     settings.EmbedChapters,
		SponsorBlock:      settings.SponsorBlock,
		FilenameTemplate:  settings.FilenameTemplate,
		MergeOutputFormat: settings.MergeOutputFormat,
		AudioFormat:       settings.AudioFormat,
		CookiesFrom:       settings.CookiesFrom,
		CookiesFile:       settings.CookiesFile,
	}
	return s.db.Save(&rec).Error
}

func (s *Service) ResetSettings() error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return s.db.Where("id = ?", 1).Delete(&SettingsRecord{}).Error
}

func (s *Service) GetDefaultDownloadDir() string {
	// Priority: YTGO_DOWNLOAD_DIR env > ~/Downloads > ~/
	if s.downloadDir != "" {
		return s.downloadDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, "Downloads")
	if _, err := os.Stat(dir); err != nil {
		return home
	}
	return dir
}

func (s *Service) SetLang(lang string) {
	s.i18n.SetLang(Lang(lang))
}

func (s *Service) GetLang() string {
	return string(s.i18n.GetLang())
}
