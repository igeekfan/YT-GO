package core

import "testing"

func TestSanitizeYTLineRemovesANSIEscapeCodes(t *testing.T) {
	raw := "\x1b[0;94m[download]\x1b[0m   4.2% of 1.23MiB at 123.4KiB/s ETA 00:09\x1b[K"
	cleaned := sanitizeYTLine(raw)

	if cleaned != "[download]   4.2% of 1.23MiB at 123.4KiB/s ETA 00:09" {
		t.Fatalf("unexpected sanitized line: %q", cleaned)
	}
	if progressRe.FindStringSubmatch(cleaned) == nil {
		t.Fatalf("expected progress regex to match sanitized line: %q", cleaned)
	}
}