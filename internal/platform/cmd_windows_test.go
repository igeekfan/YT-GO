//go:build windows

package platform

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureCmdWindowSetsHiddenBackgroundFlags(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "echo", "ok")

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
}