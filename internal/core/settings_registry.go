package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const SettingsSchemaVersion = 1

// SettingDefinition describes one persisted setting without coupling callers to storage.
type SettingDefinition struct {
	Key        string `json:"key"`
	Type       string `json:"type"`
	Sensitive  bool   `json:"sensitive"`
	Resettable bool   `json:"resettable"`
}

var settingDefinitions = []SettingDefinition{
	{Key: "outputDir", Type: "string", Resettable: true},
	{Key: "quality", Type: "string", Resettable: true},
	{Key: "language", Type: "string", Resettable: true},
	{Key: "theme", Type: "string", Resettable: true},
	{Key: "proxy", Type: "string", Sensitive: true, Resettable: true},
	{Key: "updateProxy", Type: "string", Sensitive: true, Resettable: true},
	{Key: "rateLimit", Type: "string", Resettable: true},
	{Key: "maxConcurrent", Type: "number", Resettable: true},
	{Key: "notifications", Type: "boolean", Resettable: true},
	{Key: "saveDescription", Type: "boolean", Resettable: true},
	{Key: "saveThumbnail", Type: "boolean", Resettable: true},
	{Key: "writeSubtitles", Type: "boolean", Resettable: true},
	{Key: "subtitleLangs", Type: "string", Resettable: true},
	{Key: "embedSubtitles", Type: "boolean", Resettable: true},
	{Key: "embedChapters", Type: "boolean", Resettable: true},
	{Key: "sponsorBlock", Type: "boolean", Resettable: true},
	{Key: "filenameTemplate", Type: "string", Resettable: true},
	{Key: "mergeOutputFormat", Type: "string", Resettable: true},
	{Key: "audioFormat", Type: "string", Resettable: true},
	{Key: "cookiesFrom", Type: "string", Sensitive: true, Resettable: true},
	{Key: "cookiesFile", Type: "string", Sensitive: true, Resettable: true},
}

// SettingDefinitions returns a copy so callers cannot mutate the registry.
func SettingDefinitions() []SettingDefinition {
	return append([]SettingDefinition(nil), settingDefinitions...)
}

func settingDefinition(key string) (SettingDefinition, bool) {
	for _, definition := range settingDefinitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return SettingDefinition{}, false
}

// SettingsPatch contains only the settings that should change.
type SettingsPatch map[string]json.RawMessage

// MergeSettingsPatch validates keys and merges a partial update into current settings.
func MergeSettingsPatch(current Settings, patch SettingsPatch) (Settings, error) {
	known := make(map[string]struct{}, len(settingDefinitions))
	for _, definition := range settingDefinitions {
		known[definition.Key] = struct{}{}
	}

	merged, err := json.Marshal(current)
	if err != nil {
		return Settings{}, fmt.Errorf("marshal current settings: %w", err)
	}
	values := make(map[string]json.RawMessage, len(patch))
	for key, value := range patch {
		if _, ok := known[key]; !ok {
			return Settings{}, fmt.Errorf("unknown setting: %s", key)
		}
		if len(value) == 0 || string(value) == "null" {
			return Settings{}, fmt.Errorf("setting %s cannot be null", key)
		}
		values[key] = value
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(merged, &object); err != nil {
		return Settings{}, fmt.Errorf("decode current settings: %w", err)
	}
	for key, value := range values {
		object[key] = value
	}
	merged, err = json.Marshal(object)
	if err != nil {
		return Settings{}, fmt.Errorf("marshal merged settings: %w", err)
	}
	var result Settings
	if err := json.Unmarshal(merged, &result); err != nil {
		return Settings{}, fmt.Errorf("decode merged settings: %w", err)
	}
	return result, ValidateSettings(result)
}

// ValidateSettings applies storage-level validation shared by desktop and web callers.
func ValidateSettings(settings Settings) error {
	if settings.OutputDir != "" {
		cleaned := filepath.Clean(settings.OutputDir)
		lower := strings.ToLower(cleaned)
		sensitivePaths := []string{"/", "/etc", "/proc", "/sys", "/dev", "/boot", "/usr", "/bin", "/sbin", "/var", "c:\\", "c:/", "c:\\windows", "c:\\program files"}
		for _, path := range sensitivePaths {
			if lower == path || lower == path+"/" || lower == path+"\\" {
				return fmt.Errorf("output directory cannot be a system path: %s", cleaned)
			}
		}
	}
	if settings.Proxy != "" {
		if err := validateProxyValue(settings.Proxy, false); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	if settings.UpdateProxy != "" && !strings.EqualFold(strings.TrimSpace(settings.UpdateProxy), "direct") {
		if err := validateProxyValue(settings.UpdateProxy, false); err != nil {
			return fmt.Errorf("update proxy: %w", err)
		}
	}
	if settings.MaxConcurrent < 1 || settings.MaxConcurrent > 10 {
		return fmt.Errorf("max concurrent downloads must be between 1 and 10, got: %d", settings.MaxConcurrent)
	}
	return nil
}

func validateProxyValue(raw string, allowDirect bool) error {
	raw = strings.TrimSpace(raw)
	if allowDirect && strings.EqualFold(raw, "direct") {
		return nil
	}
	proxyURL, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	scheme := strings.ToLower(proxyURL.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" && scheme != "socks5h" {
		return fmt.Errorf("proxy scheme must be http, https, or socks5, got: %s", proxyURL.Scheme)
	}
	if proxyURL.Host == "" {
		return fmt.Errorf("proxy URL must include a host")
	}
	return nil
}

// ApplySettingsPatch validates and persists only the requested settings.
func (s *Service) ApplySettingsPatch(patch SettingsPatch) (Settings, error) {
	settings, err := MergeSettingsPatch(s.GetSettings(), patch)
	if err != nil {
		return Settings{}, err
	}
	return s.applySettingsFromRegistry(settings)
}

// applySettingsFromRegistry persists a complete Settings value through the registry path.
func (s *Service) applySettingsFromRegistry(settings Settings) (Settings, error) {
	if err := ValidateSettings(settings); err != nil {
		return Settings{}, err
	}
	if err := s.persistSettings(settings); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(), nil
}
