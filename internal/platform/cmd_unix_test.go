//go:build aix || android || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package platform

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestConfigureAndTerminateProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	ConfigureCmdWindow(cmd, true)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("expected a separate process group")
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if err := TerminateProcessTree(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			t.Fatalf("wait returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("process group was not terminated")
	}
}
