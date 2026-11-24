//go:build !windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"YT-GO/internal/platform"
)

func (a *App) launchDownloadedUpdate(updatePath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", updatePath)
	case "linux":
		if filepath.Ext(updatePath) == ".AppImage" {
			if err := os.Chmod(updatePath, 0o755); err != nil {
				return fmt.Errorf("failed to prepare downloaded update: %w", err)
			}
			cmd = exec.Command(updatePath)
		} else {
			cmd = exec.Command("xdg-open", updatePath)
		}
	default:
		return fmt.Errorf("automatic installation is not supported on %s", runtime.GOOS)
	}
	platform.HideCmdWindow(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open downloaded update: %w", err)
	}
	return nil
}
