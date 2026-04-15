package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetSettings returns persisted user settings.
func (a *App) GetSettings() Settings {
	defaults := Settings{
		OutputDir:         a.GetDefaultDownloadDir(),
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
	if a.db == nil {
		return defaults
	}

	var rec SettingsRecord
	if err := a.db.First(&rec, 1).Error; err != nil {
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

// IsFirstRun returns true if this is the first time the app is run (no settings saved).
func (a *App) IsFirstRun() bool {
	if a.db == nil {
		return true
	}
	var rec SettingsRecord
	if err := a.db.First(&rec, 1).Error; err != nil {
		return true
	}
	return false
}

// NeedsCookieConfig returns true if user needs to configure cookies or proxy.
func (a *App) NeedsCookieConfig() bool {
	s := a.GetSettings()
	return s.CookiesFrom == "" && s.CookiesFile == "" && s.Proxy == ""
}

// SaveSettings persists user settings to the database.
func (a *App) SaveSettings(s Settings) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	rec := SettingsRecord{
		ID:                1,
		OutputDir:         s.OutputDir,
		Quality:           s.Quality,
		Language:          s.Language,
		Theme:             s.Theme,
		Proxy:             s.Proxy,
		RateLimit:         s.RateLimit,
		MaxConcurrent:     s.MaxConcurrent,
		Notifications:     s.Notifications,
		SaveDescription:   s.SaveDescription,
		SaveThumbnail:     s.SaveThumbnail,
		WriteSubtitles:    s.WriteSubtitles,
		SubtitleLangs:     s.SubtitleLangs,
		EmbedSubtitles:    s.EmbedSubtitles,
		EmbedChapters:     s.EmbedChapters,
		SponsorBlock:      s.SponsorBlock,
		FilenameTemplate:  s.FilenameTemplate,
		MergeOutputFormat: s.MergeOutputFormat,
		AudioFormat:       s.AudioFormat,
		CookiesFrom:       s.CookiesFrom,
		CookiesFile:       s.CookiesFile,
	}
	return a.db.Save(&rec).Error
}

// ResetSettings clears all settings, useful for testing first-run wizard.
func (a *App) ResetSettings() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.db.Where("id = ?", 1).Delete(&SettingsRecord{}).Error
}

// GetDefaultDownloadDir returns the user Downloads directory.
func (a *App) GetDefaultDownloadDir() string {
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

// SetLang sets the application language.
func (a *App) SetLang(lang string) {
	a.i18n.SetLang(Lang(lang))
}

// GetLang returns the current language.
func (a *App) GetLang() string {
	return string(a.i18n.GetLang())
}
