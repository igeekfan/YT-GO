package core

import (
	"encoding/json"
	"testing"
)

func TestSettingDefinitionsAreIndependent(t *testing.T) {
	definitions := SettingDefinitions()
	if len(definitions) != 20 {
		t.Fatalf("definitions=%d, want 20", len(definitions))
	}
	definitions[0].Key = "changed"
	if SettingDefinitions()[0].Key == "changed" {
		t.Fatal("registry was mutated through returned slice")
	}
}

func TestMergeSettingsPatchPreservesUnchangedValues(t *testing.T) {
	current := Settings{OutputDir: "C:/Downloads", Quality: "best", MaxConcurrent: 3, Notifications: true}
	quality, _ := json.Marshal("audio")
	updated, err := MergeSettingsPatch(current, SettingsPatch{"quality": quality})
	if err != nil {
		t.Fatalf("merge patch: %v", err)
	}
	if updated.Quality != "audio" || updated.OutputDir != current.OutputDir || updated.MaxConcurrent != current.MaxConcurrent || !updated.Notifications {
		t.Fatalf("patch did not preserve settings: %+v", updated)
	}
}

func TestMergeSettingsPatchRejectsUnknownKey(t *testing.T) {
	value, _ := json.Marshal("value")
	if _, err := MergeSettingsPatch(Settings{MaxConcurrent: 3}, SettingsPatch{"unknown": value}); err == nil {
		t.Fatal("unknown setting was accepted")
	}
}

func TestMergeSettingsPatchRejectsInvalidValue(t *testing.T) {
	value, _ := json.Marshal(0)
	if _, err := MergeSettingsPatch(Settings{MaxConcurrent: 3}, SettingsPatch{"maxConcurrent": value}); err == nil {
		t.Fatal("invalid maxConcurrent was accepted")
	}
}
