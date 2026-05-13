package core

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
)

// Progress regex patterns for parsing yt-dlp stderr output.
var (
	progressRe  = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+at\s+(\S+)(?:\s+ETA\s+(\S+))?`)
	destRe1     = regexp.MustCompile(`^\[download\] Destination: (.+)$`)
	destRe2     = regexp.MustCompile(`Merging formats into "(.+)"`)
	destRe3     = regexp.MustCompile(`^\[ExtractAudio\] Destination: (.+)$`)
	finalPathRe = regexp.MustCompile(`^\[YT-GO-OUTPUT\](.+)$`)
)

// lineWriter buffers bytes into complete lines and calls handler for each.
type lineWriter struct {
	mu      sync.Mutex
	buf     []byte
	handler func(string)
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.buf = append(lw.buf, p...)
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(toUTF8(lw.buf[:idx]), "\r")
		lw.buf = lw.buf[idx+1:]
		if line != "" {
			lw.handler(line)
		}
	}
	return len(p), nil
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
