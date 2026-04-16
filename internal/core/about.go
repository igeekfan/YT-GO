package core

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

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
		if version := commandOutput("cmd", "/c", "ver"); version != "" {
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
