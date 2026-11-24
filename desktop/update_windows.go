package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shellExecuteNoAsync = 0x00000100
	shellExecuteNoUI    = 0x00000400
)

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	window     windows.Handle
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instance   windows.Handle
	idList     unsafe.Pointer
	class      *uint16
	classKey   windows.Handle
	hotKey     uint32
	icon       windows.Handle
	process    windows.Handle
}

func (a *App) launchDownloadedUpdate(updatePath string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("%s: %w", a.updateText("update.open_failed"), err)
	}
	if !strings.EqualFold(filepath.Base(executable), "YT-GO.exe") {
		return errors.New(a.updateText("update.windows_executable_name"))
	}
	window := windows.Handle(uintptr(a.window.NativeWindow()))
	err = runUpdateInstaller(window, updatePath, filepath.Dir(executable), os.Getpid())
	if errors.Is(err, windows.ERROR_CANCELLED) {
		return errors.New(a.updateText("update.elevation_cancelled"))
	}
	if err != nil {
		return fmt.Errorf("%s: %w", a.updateText("update.open_failed"), err)
	}
	return nil
}

func runUpdateInstaller(window windows.Handle, updatePath, installDir string, pid int) error {
	file, err := windows.UTF16PtrFromString(updatePath)
	if err != nil {
		return err
	}
	parameters, err := windows.UTF16PtrFromString(fmt.Sprintf("/UPDATEPID=%d /D=%s", pid, installDir))
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && err != syscall.Errno(windows.S_FALSE) {
		return err
	}
	defer windows.CoUninitialize()

	info := shellExecuteInfo{
		mask:       shellExecuteNoAsync | shellExecuteNoUI,
		window:     window,
		verb:       verb,
		file:       file,
		parameters: parameters,
		show:       windows.SW_SHOWNORMAL,
	}
	info.size = uint32(unsafe.Sizeof(info))
	if started, _, err := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info))); started == 0 {
		return err
	}
	return nil
}
