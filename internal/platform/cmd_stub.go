//go:build aix || android || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package platform

import (
	"errors"
	"os/exec"
	"syscall"
)

func HideCmdWindow(cmd *exec.Cmd) {
}

func ConfigureCmdWindow(cmd *exec.Cmd, separateProcessGroup bool) {
	if !separateProcessGroup {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// TerminateProcessTree terminates the process group created for pid.
func TerminateProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func EnableUTF8Console() {
}
