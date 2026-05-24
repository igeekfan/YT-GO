package core

import "testing"

func TestParseStructuredProgressLine(t *testing.T) {
	line := `progress:{"progress":{"status":"downloading","downloaded_bytes":5242880,"total_bytes":10485760,"filename":"video.mp4.part","fragment_index":2,"fragment_count":4}}`

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
	if update.Filename != "video.mp4.part" {
		t.Fatalf("unexpected filename: %s", update.Filename)
	}
}

func TestIsSidecarProgressFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{name: "subtitle sidecar", filename: "video.zh-Hans.vtt.part", want: true},
		{name: "thumbnail sidecar", filename: "video.webp", want: true},
		{name: "description sidecar", filename: "video.description", want: true},
		{name: "main media file", filename: "video.f137.mp4.part", want: false},
		{name: "audio stream file", filename: "video.f140.m4a.part", want: false},
	}

	for _, test := range tests {
		if got := isSidecarProgressFile(test.filename); got != test.want {
			t.Fatalf("%s: expected %v, got %v", test.name, test.want, got)
		}
	}
}