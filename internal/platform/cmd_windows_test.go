//go:build windows

package platform

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestConfigureCmdWindowSetsHiddenBackgroundFlags(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "echo", "ok")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010}

	ConfigureCmdWindow(cmd, true)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be configured")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("expected HideWindow to be enabled")
	}
	if cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("expected CREATE_NO_WINDOW to be enabled")
	}
	if cmd.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Fatal("expected CREATE_NEW_PROCESS_GROUP to be enabled")
	}
	if cmd.SysProcAttr.CreationFlags&0x00000010 == 0 {
		t.Fatal("expected existing creation flags to be preserved")
	}
}

func TestTerminateProcessTreeStopsRunningProcess(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping", "-n", "30", "127.0.0.1")
	ConfigureCmdWindow(cmd, true)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if err := TerminateProcessTree(cmd.Process.Pid); err != nil {
		t.Fatalf("terminate process tree: %v", err)
	}
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			t.Fatalf("wait returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("process was not terminated")
	}
}
