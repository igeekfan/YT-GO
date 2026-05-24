package core

import "testing"

func TestParseStructuredProgressLine(t *testing.T) {
	line := `progress:{"progress":{"status":"downloading","downloaded_bytes":5242880,"total_bytes":10485760,"fragment_index":2,"fragment_count":4}}`

	update, ok := parseStructuredProgressLine(line)
	if !ok {
		t.Fatal("expected structured progress line to parse")
	}
	if update.Status != "downloading" {
		t.Fatalf("unexpected status: %s", update.Status)
	}
	if update.DownloadedBytes != 5242880 {
		t.Fatalf("unexpected downloaded bytes: %d", update.DownloadedBytes)
	}
	if update.TotalBytes != 10485760 {
		t.Fatalf("unexpected total bytes: %d", update.TotalBytes)
	}
	if update.FragmentIndex != 2 || update.FragmentCount != 4 {
		t.Fatalf("unexpected fragment progress: %d/%d", update.FragmentIndex, update.FragmentCount)
	}
}