//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

func HideCmdWindow(cmd *exec.Cmd) {
	ConfigureCmdWindow(cmd, false)
}

func ConfigureCmdWindow(cmd *exec.Cmd, separateProcessGroup bool) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
	if separateProcessGroup {
		cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	}
}