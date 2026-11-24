package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"YT-GO/internal/core"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const releasePageURL = "https://github.com/igeekfan/YT-GO/releases"

func (a *App) SelectFolder() string {
	dir, err := a.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Select Download Directory",
		CanChooseDirectories: true,
		CanCreateDirectories: true,
		Window:               a.window,
	}).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return dir
}

func (a *App) SelectCookiesFile() string {
	file, err := a.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:          "Select Cookies File",
		CanChooseFiles: true,
		Window:         a.window,
		Filters: []application.FileFilter{
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	}).PromptForSingleSelection()
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

func (a *App) downloadAndInstallUpdate() (err error) {
	if !a.installingUpdate.CompareAndSwap(false, true) {
		return errors.New(a.updateText("update.in_progress"))
	}
	defer func() {
		if err != nil || runtime.GOOS != "windows" {
			a.installingUpdate.Store(false)
		}
	}()

	updatePath, err := a.service.DownloadUpdate()
	if err != nil {
		return err
	}
	if err := a.launchDownloadedUpdate(updatePath); err != nil {
		_ = os.Remove(updatePath)
		return err
	}
	if runtime.GOOS == "windows" {
		go a.window.Close()
	}
	return nil
}

func (a *App) updateText(key string) string {
	translator := core.NewI18n()
	translator.SetLang(core.Lang(a.service.GetLang()))
	return translator.T(key)
}
