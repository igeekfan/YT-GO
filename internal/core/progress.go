package core

import (
	"regexp"
	"strings"
)

// Progress regex patterns for parsing yt-dlp stderr output.
// Used by douyin download and fallback progress parsing.
var (
	progressRe  = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+at\s+(\S+)(?:\s+ETA\s+(\S+))?`)
	destRe1     = regexp.MustCompile(`^\[download\] Destination: (.+)$`)
	destRe2     = regexp.MustCompile(`Merging formats into "(.+)"`)
	destRe3     = regexp.MustCompile(`^\[ExtractAudio\] Destination: (.+)$`)
	finalPathRe = regexp.MustCompile(`^\[YT-GO-OUTPUT\](.+)$`)
)

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
