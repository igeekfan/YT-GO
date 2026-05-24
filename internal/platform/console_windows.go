//go:build windows

package platform

import "syscall"

const cpUTF8 = 65001

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleOutCP   = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleInputCP = kernel32.NewProc("SetConsoleCP")
)

// EnableUTF8Console switches the attached Windows console to UTF-8 (chcp 65001)
// so fmt.Println / log output containing Chinese characters is rendered correctly
// instead of mojibake under the default OEM codepage (e.g. CP936).
//
// Safe to call when no console is attached (Wails GUI build): the syscalls just
// return without effect.
func EnableUTF8Console() {
	procSetConsoleOutCP.Call(uintptr(cpUTF8))
	procSetConsoleInputCP.Call(uintptr(cpUTF8))
}
