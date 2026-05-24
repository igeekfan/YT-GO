//go:build !windows

package platform

import "os/exec"

func HideCmdWindow(cmd *exec.Cmd) {
}

func ConfigureCmdWindow(cmd *exec.Cmd, separateProcessGroup bool) {
}
