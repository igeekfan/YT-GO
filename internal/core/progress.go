package core

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

// Progress regex patterns for parsing yt-dlp stderr output.
// Used by douyin download and fallback progress parsing.
//
// yt-dlp emits two progress line shapes:
//
//	[download]  12.0% of   46.39MiB at    2.31MiB/s ETA 00:19          (HTTP)
//	[download] 100.0% of ~ 383.04KiB at  48.16KiB/s ETA 01:28 (frag 3/4) (HLS/DASH; size prefixed by ~ for approximate)
//	[download] 100%  of  383.34KiB in  00:00:01 at 203.71KiB/s          (final completion line: "in TIME at SPEED")
//
// The `~?\s*` allows the optional approximate-size marker before the size token.
var (
	structuredProgressPrefix = "progress:"
	ansiEscapeRe   = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	progressRe     = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*(\S+)\s+at\s+(\S+)(?:\s+ETA\s+(\S+))?`)
	progressDoneRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*(\S+)\s+in\s+\S+\s+at\s+(\S+)`)
	destRe1        = regexp.MustCompile(`^\[download\] Destination: (.+)$`)
	destRe2        = regexp.MustCompile(`Merging formats into "(.+)"`)
	destRe3        = regexp.MustCompile(`^\[ExtractAudio\] Destination: (.+)$`)
	finalPathRe    = regexp.MustCompile(`^\[YT-GO-OUTPUT\](.+)$`)
)

type structuredProgressUpdate struct {
	Status          string
	DownloadedBytes int64
	TotalBytes      int64
	FragmentIndex   int
	FragmentCount   int
	Filename        string
}

type structuredProgressPayload struct {
	Progress struct {
		Status             string  `json:"status"`
		TotalBytes         int64   `json:"total_bytes,omitempty"`
		TotalBytesEstimate float64 `json:"total_bytes_estimate,omitempty"`
		DownloadedBytes    int64   `json:"downloaded_bytes"`
		Filename           string  `json:"filename,omitempty"`
		TmpFilename        string  `json:"tmpfilename,omitempty"`
		FragmentIndex      int     `json:"fragment_index,omitempty"`
		FragmentCount      int     `json:"fragment_count,omitempty"`
	} `json:"progress"`
}

func sanitizeYTLine(line string) string {
	cleaned := ansiEscapeRe.ReplaceAllString(line, "")
	return strings.TrimSpace(cleaned)
}

func parseStructuredProgressLine(line string) (structuredProgressUpdate, bool) {
	if !strings.HasPrefix(line, structuredProgressPrefix) {
		return structuredProgressUpdate{}, false
	}

	var payload structuredProgressPayload
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, structuredProgressPrefix)), &payload); err != nil {
		return structuredProgressUpdate{}, false
	}

	totalBytes := payload.Progress.TotalBytes
	if totalBytes == 0 && payload.Progress.TotalBytesEstimate > 0 {
		totalBytes = int64(payload.Progress.TotalBytesEstimate)
	}

	filename := payload.Progress.Filename
	if filename == "" {
		filename = payload.Progress.TmpFilename
	}

	return structuredProgressUpdate{
		Status:          payload.Progress.Status,
		DownloadedBytes: payload.Progress.DownloadedBytes,
		TotalBytes:      totalBytes,
		FragmentIndex:   payload.Progress.FragmentIndex,
		FragmentCount:   payload.Progress.FragmentCount,
		Filename:        filename,
	}, true
}

func isSidecarProgressFile(filename string) bool {
	if filename == "" {
		return false
	}

	normalized := strings.ToLower(filename)
	for _, suffix := range []string{".part", ".temp", ".tmp"} {
		normalized = strings.TrimSuffix(normalized, suffix)
	}

	base := filepath.Base(normalized)
	if strings.HasSuffix(base, ".info.json") {
		return true
	}

	switch filepath.Ext(base) {
	case ".description", ".json3", ".jpg", ".jpeg", ".lrc", ".png", ".sbv", ".srt", ".srv1", ".srv2", ".srv3", ".ttml", ".vtt", ".webp", ".txt":
		return true
	default:
		return false
	}
}

// shouldApplyMergeOutputFormat returns true if --merge-output-format should be applied for the given quality.
func shouldApplyMergeOutputFormat(quality string) bool {
	if quality == "audio" || strings.HasPrefix(quality, "fa:") || strings.HasPrefix(quality, "fv:") || strings.HasPrefix(quality, "f:") {
		return false
	}
	return true
}

// requiresAudioExtraction returns true if the quality requires audio extraction (-x).
func requiresAudioExtraction(quality string) bool {
	return quality == "audio" || strings.HasPrefix(quality, "fa:")
}
