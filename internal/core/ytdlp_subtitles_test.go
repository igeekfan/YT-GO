package core

import (
	"strings"
	"testing"

	"github.com/lrstanley/go-ytdlp"
)

func TestExtractSubtitleLangsFromExtractedKeepsManualAndAutoVariants(t *testing.T) {
	manualName := "English"
	autoName := "English (auto-generated)"
	raw := &ytdlp.ExtractedInfo{
		Subtitles: map[string][]*ytdlp.ExtractedSubtitle{
			"en": {{Name: &manualName}},
		},
		AutomaticCaptions: map[string][]*ytdlp.ExtractedSubtitle{
			"en": {{Name: &autoName}},
		},
	}

	langs := extractSubtitleLangsFromExtracted(raw)
	if len(langs) != 2 {
		t.Fatalf("expected 2 subtitle options, got %d", len(langs))
	}

	if langs[0].Code != "en" || langs[0].Auto || langs[0].Selector != "manual:en" {
		t.Fatalf("unexpected manual subtitle entry: %+v", langs[0])
	}
	if langs[1].Code != "en" || !langs[1].Auto || langs[1].Selector != "auto:en" {
		t.Fatalf("unexpected auto subtitle entry: %+v", langs[1])
	}
}

func TestParseListSubsOutputIncludesTranslatedAutoSubtitles(t *testing.T) {
	output := strings.TrimSpace(`
[info] Available automatic captions for _CKmuMFCxQ8:
Language        Name                                            Formats
zh-Hans                                                         vtt
en-zh-Hans      English from Chinese (Simplified)               vtt, srt, ttml, srv3, srv2, srv1, json3
[info] Available subtitles for _CKmuMFCxQ8:
Language Name                 Formats
zh-Hans  Chinese (Simplified) vtt, srt, ttml, srv3, srv2, srv1, json3
`)

	langs := parseListSubsOutput(output)
	if len(langs) != 3 {
		t.Fatalf("expected 3 subtitle options, got %d (%+v)", len(langs), langs)
	}

	if langs[0].Selector != "manual:zh-Hans" || langs[0].Auto {
		t.Fatalf("unexpected first subtitle: %+v", langs[0])
	}
	if langs[1].Selector != "auto:en-zh-Hans" || !langs[1].Auto {
		t.Fatalf("unexpected translated subtitle: %+v", langs[1])
	}
	if langs[2].Selector != "auto:zh-Hans" || !langs[2].Auto {
		t.Fatalf("unexpected auto source subtitle: %+v", langs[2])
	}
	if langs[1].Name != "English from Chinese (Simplified)" {
		t.Fatalf("unexpected translated subtitle name: %+v", langs[1])
	}
}

func TestMergeSubtitleEntriesKeepsListedTranslations(t *testing.T) {
	merged := mergeSubtitleEntries(
		[]SubtitleLang{{Code: "zh-Hans", Name: "Chinese (Simplified)", Auto: false, Selector: "manual:zh-Hans"}},
		[]SubtitleLang{{Code: "en-zh-Hans", Name: "English from Chinese (Simplified)", Auto: true, Selector: "auto:en-zh-Hans"}},
	)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged subtitles, got %d", len(merged))
	}
	if merged[1].Code != "en-zh-Hans" || !merged[1].Auto {
		t.Fatalf("unexpected merged translated subtitle: %+v", merged[1])
	}
}
