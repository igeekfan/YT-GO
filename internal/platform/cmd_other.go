//go:build js || plan9 || wasip1

package platform

import (
	"os"
	"os/exec"
)

func HideCmdWindow(cmd *exec.Cmd) {
}

func ConfigureCmdWindow(cmd *exec.Cmd, separateProcessGroup bool) {
}

func TerminateProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func EnableUTF8Console() {
}
