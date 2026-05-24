package core

import "testing"

func TestEmitDownloadLogAlsoMirrorsToAppLog(t *testing.T) {
	service := NewService("test")

	var appLogs []string
	var downloadLogs []struct {
		taskID string
		line   string
	}

	service.SetHooks(Hooks{
		AppLog: func(msg string) {
			appLogs = append(appLogs, msg)
		},
		DownloadLog: func(taskID string, line string) {
			downloadLogs = append(downloadLogs, struct {
				taskID string
				line   string
			}{taskID: taskID, line: line})
		},
	})

	service.emitDownloadLog("task-1", "[download] progress line")

	if len(downloadLogs) != 1 {
		t.Fatalf("expected 1 download log, got %d", len(downloadLogs))
	}
	if downloadLogs[0].taskID != "task-1" || downloadLogs[0].line != "[download] progress line" {
		t.Fatalf("unexpected download log payload: %+v", downloadLogs[0])
	}
	if len(appLogs) != 1 {
		t.Fatalf("expected mirrored app log, got %d entries", len(appLogs))
	}
	if appLogs[0] != "[task-1] [download] progress line" {
		t.Fatalf("unexpected mirrored app log: %q", appLogs[0])
	}
}