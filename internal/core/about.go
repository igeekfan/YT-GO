package core

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"

	"YT-GO/internal/platform"
)

const (
	githubRepoURL = "https://github.com/igeekfan/YT-GO"
	authorEmail   = "igeekfan@foxmail.com"
)

func (s *Service) GetAboutInfo() AboutInfo {
	return AboutInfo{
		AppVersion:    s.appVersion,
		SystemVersion: detectSystemVersion(),
		GithubRepo:    fmt.Sprintf("%s/%s", githubOwner, githubRepo),
		GithubURL:     githubRepoURL,
		AuthorEmail:   authorEmail,
	}
}

func detectSystemVersion() string {
	switch runtime.GOOS {
	case "windows":
		if version := commandOutputUTF16("cmd", "/u", "/c", "ver"); version != "" {
			return version
		}
	case "darwin":
		if version := commandOutput("sw_vers", "-productVersion"); version != "" {
			return "macOS " + version
		}
	case "linux":
		if version := commandOutput("uname", "-sr"); version != "" {
			return version
		}
	}

	if version := commandOutput("uname", "-a"); version != "" {
		return version
	}

	return runtime.GOOS
}

func commandOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	platform.HideCmdWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// commandOutputUTF16 runs a command and decodes its UTF-16LE output.
// On Windows, `cmd /u` forces UTF-16LE output, avoiding OEM codepage encoding issues.
func commandOutputUTF16(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	platform.HideCmdWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	// Skip UTF-16LE BOM if present (0xFF 0xFE)
	if len(out) >= 2 && out[0] == 0xFF && out[1] == 0xFE {
		out = out[2:]
	}
	// Decode UTF-16LE bytes to runes
	if len(out)%2 != 0 {
		return strings.TrimSpace(string(out))
	}
	u16 := make([]uint16, len(out)/2)
	for i := range u16 {
		u16[i] = uint16(out[2*i]) | uint16(out[2*i+1])<<8
	}
	return strings.TrimSpace(string(utf16.Decode(u16)))
}
