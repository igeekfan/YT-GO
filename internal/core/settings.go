package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) GetSettings() Settings {
	defaults := Settings{
		OutputDir:         s.GetDefaultDownloadDir(),
		Quality:           "best",
		Language:          string(defaultLang()),
		Theme:             "dark",
		Proxy:             "",
		UpdateProxy:       "",
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
	db := s.database()
	if db == nil {
		return defaults
	}
	var values []SettingValueRecord
	if err := db.Find(&values).Error; err != nil {
		return defaults
	}
	return applySettingValues(defaults, values)
}

func (s *Service) IsFirstRun() bool {
	db := s.database()
	if db == nil {
		return true
	}
	var count int64
	if err := db.Model(&SettingValueRecord{}).Count(&count).Error; err != nil {
		return true
	}
	return count == 0
}

func (s *Service) NeedsCookieConfig() bool {
	settings := s.GetSettings()
	return settings.CookiesFrom == "" && settings.CookiesFile == "" && settings.Proxy == ""
}

func (s *Service) SaveSettings(settings Settings) error {
	_, err := s.applySettingsFromRegistry(settings)
	return err
}

func (s *Service) persistSettings(settings Settings) error {
	if s.database() == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := s.ensureRuntime(); err != nil {
		return err
	}
	done := make(chan error, 1)
	if err := s.submitPersistence(persistenceOperation{kind: persistenceSaveSettings, settings: settings, done: done}); err != nil {
		return err
	}
	s.limiter.SetLimit(settings.MaxConcurrent)
	return nil
}

func applySettingValues(settings Settings, values []SettingValueRecord) Settings {
	payload, err := json.Marshal(settings)
	if err != nil {
		return settings
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		return settings
	}
	for _, value := range values {
		if _, ok := settingDefinition(value.Key); ok {
			object[value.Key] = json.RawMessage(value.Value)
		}
	}
	merged, err := json.Marshal(object)
	if err != nil {
		return settings
	}
	_ = json.Unmarshal(merged, &settings)
	return settings
}

func (s *Service) ResetSettings() error {
	if s.database() == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := s.ensureRuntime(); err != nil {
		return err
	}
	done := make(chan error, 1)
	if err := s.submitPersistence(persistenceOperation{kind: persistenceDeleteSettings, done: done}); err != nil {
		return err
	}
	s.limiter.SetLimit(defaultMaxConcurrentDownloads)
	return nil
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
