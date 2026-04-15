package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	openPath := path
	if runtime.GOOS != "windows" && isFile {
		openPath = filepath.Dir(path)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if isFile {
			cmd = exec.Command("explorer", "/select,", path)
		} else {
			cmd = exec.Command("explorer", path)
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
