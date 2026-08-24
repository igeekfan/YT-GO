//go:build windows

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func HideCmdWindow(cmd *exec.Cmd) {
	ConfigureCmdWindow(cmd, false)
}

func ConfigureCmdWindow(cmd *exec.Cmd, separateProcessGroup bool) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x08000000
	cmd.SysProcAttr.HideWindow = true
	if separateProcessGroup {
		cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	}
}

// TerminateProcessTree forcefully terminates pid and every process below it.
func TerminateProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F")
	ConfigureCmdWindow(cmd, false)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.ToLower(string(output))
	if strings.Contains(message, "not found") || strings.Contains(message, "没有找到") {
		return nil
	}
	return fmt.Errorf("terminate process tree %d: %w: %s", pid, err, strings.TrimSpace(string(output)))
}
