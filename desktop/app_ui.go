package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"YT-GO/internal/platform"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const releasePageURL = "https://github.com/igeekfan/YT-GO/releases"

func (a *App) SelectFolder() string {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Download Directory",
	})
	if err != nil {
		return ""
	}
	return dir
}

func (a *App) SelectCookiesFile() string {
	file, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Cookies File",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return file
}

func (a *App) OpenFolder(path string) error {
	path = filepath.Clean(path)
	fileInfo, err := os.Stat(path)
	isFile := err == nil && !fileInfo.IsDir()

	// If path doesn't exist, try parent directory
	openPath := path
	if err != nil {
		parent := filepath.Dir(path)
		if _, parentErr := os.Stat(parent); parentErr == nil {
			openPath = parent
		} else {
			// Neither path nor parent exists, return error
			return fmt.Errorf("path does not exist: %s", path)
		}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if isFile {
			cmd = exec.Command("explorer", "/select,", path)
		} else {
			cmd = exec.Command("explorer", openPath)
		}
	case "darwin":
		if isFile {
			cmd = exec.Command("open", "-R", path)
		} else {
			cmd = exec.Command("open", openPath)
		}
	default:
		cmd = exec.Command("xdg-open", openPath)
	}
	return cmd.Start()
}

func (a *App) OpenFile(path string) error {
	path = filepath.Clean(path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func (a *App) OpenReleasePage() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", releasePageURL)
	case "darwin":
		cmd = exec.Command("open", releasePageURL)
	default:
		cmd = exec.Command("xdg-open", releasePageURL)
	}
	return cmd.Start()
}

func (a *App) downloadAndInstallUpdate() error {
	updatePath, err := a.service.DownloadUpdate()
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(updatePath)
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
		_ = os.Remove(updatePath)
		return fmt.Errorf("automatic installation is not supported on %s", runtime.GOOS)
	}
	platform.HideCmdWindow(cmd)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(updatePath)
		return fmt.Errorf("failed to open downloaded update: %w", err)
	}
	if runtime.GOOS == "windows" && a.ctx != nil {
		go func() {
			time.Sleep(750 * time.Millisecond)
			wailsRuntime.Quit(a.ctx)
		}()
	}
	return nil
}
