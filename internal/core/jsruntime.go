package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"YT-GO/internal/platform"
)

const (
	minimumDenoMajor = 2
	minimumNodeMajor = 20
)

// runtimeProbe holds the result of probing for a JS runtime (deno or node).
type runtimeProbe struct {
	Name      string
	Path      string
	Version   string
	Supported bool
	Found     bool
	Reason    string
	Arg       string
}

// jsRuntimeSelection holds the preferred JS runtime selection result.
type jsRuntimeSelection struct {
	Arg     string
	Name    string
	Version string
	Path    string
	Found   bool
	Reason  string
}

// extractSemanticVersion extracts the first semantic version substring (e.g. "1.2.3") from input.
func extractSemanticVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	start := -1
	for index, char := range trimmed {
		if char >= '0' && char <= '9' {
			start = index
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := start
	for end < len(trimmed) {
		char := trimmed[end]
		if (char < '0' || char > '9') && char != '.' {
			break
		}
		end++
	}
	return trimmed[start:end]
}

// parseRuntimeMajorVersion extracts the major version number from a version string.
func parseRuntimeMajorVersion(version string) (int, bool) {
	trimmed := extractSemanticVersion(version)
	if dot := strings.Index(trimmed, "."); dot > 0 {
		trimmed = trimmed[:dot]
	}
	major := 0
	if trimmed == "" {
		return 0, false
	}
	for _, c := range trimmed {
		if c < '0' || c > '9' {
			return 0, false
		}
		major = major*10 + int(c-'0')
	}
	return major, true
}

// isNodeVersionSufficient checks if the node at the given path meets the minimum version requirement.
func isNodeVersionSufficient(nodePath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	nodeCheckCmd := exec.CommandContext(ctx, nodePath, "-v")
	platform.HideCmdWindow(nodeCheckCmd)
	out, err := nodeCheckCmd.CombinedOutput()
	if err != nil {
		return false
	}
	major, ok := parseRuntimeMajorVersion(strings.TrimSpace(toUTF8(out)))
	return ok && major >= minimumNodeMajor
}

// probeDenoRuntime probes for deno and checks if it meets version requirements.
func probeDenoRuntime(i *I18n) runtimeProbe {
	probe := runtimeProbe{Name: "deno"}
	denoPath, err := exec.LookPath("deno")
	if err != nil || denoPath == "" {
		return probe
	}
	probe.Found = true
	probe.Path = denoPath
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	denoCmd := exec.CommandContext(ctx, denoPath, "--version")
	platform.HideCmdWindow(denoCmd)
	out, runErr := denoCmd.CombinedOutput()
	if runErr != nil {
		probe.Reason = fmt.Sprintf(i.T("runtime.deno.found_but_failed"), denoPath)
		return probe
	}
	firstLine := strings.TrimSpace(strings.SplitN(toUTF8(out), "\n", 2)[0])
	probe.Version = firstLine
	if major, ok := parseRuntimeMajorVersion(firstLine); ok && major >= minimumDenoMajor {
		probe.Supported = true
		probe.Arg = "deno:" + denoPath
		return probe
	}
	detectedVersion := extractSemanticVersion(firstLine)
	if detectedVersion == "" {
		detectedVersion = firstLine
	}
	probe.Reason = fmt.Sprintf(i.T("runtime.deno.too_old"), detectedVersion, minimumDenoMajor)
	return probe
}

// probeNodeRuntime probes for node and checks if it meets version requirements.
func probeNodeRuntime(i *I18n) runtimeProbe {
	probe := runtimeProbe{Name: "node"}
	nodePath, err := exec.LookPath("node")
	if err != nil || nodePath == "" {
		return probe
	}
	probe.Found = true
	probe.Path = nodePath
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	nodeCmd := exec.CommandContext(ctx, nodePath, "-v")
	platform.HideCmdWindow(nodeCmd)
	out, runErr := nodeCmd.CombinedOutput()
	if runErr != nil {
		probe.Reason = fmt.Sprintf(i.T("runtime.node.found_but_failed"), nodePath)
		return probe
	}
	version := strings.TrimSpace(toUTF8(out))
	probe.Version = version
	if major, ok := parseRuntimeMajorVersion(version); ok && major >= minimumNodeMajor {
		probe.Supported = true
		probe.Arg = "node:" + nodePath
		return probe
	}
	detectedVersion := extractSemanticVersion(version)
	if detectedVersion == "" {
		detectedVersion = version
	}
	probe.Reason = fmt.Sprintf(i.T("runtime.node.too_old"), detectedVersion, minimumNodeMajor)
	return probe
}

// detectPreferredJSRuntime detects the best available JS runtime for yt-dlp.
func detectPreferredJSRuntime(i *I18n) jsRuntimeSelection {
	denoProbe := probeDenoRuntime(i)
	if denoProbe.Supported {
		return jsRuntimeSelection{Arg: denoProbe.Arg, Name: denoProbe.Name, Version: denoProbe.Version, Path: denoProbe.Path, Found: true}
	}

	nodeProbe := probeNodeRuntime(i)
	if nodeProbe.Supported {
		return jsRuntimeSelection{Arg: nodeProbe.Arg, Name: nodeProbe.Name, Version: nodeProbe.Version, Path: nodeProbe.Path, Found: true}
	}

	var reasons []string
	if denoProbe.Reason != "" {
		reasons = append(reasons, denoProbe.Reason)
	}
	if nodeProbe.Reason != "" {
		reasons = append(reasons, nodeProbe.Reason)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, i.T("runtime.none_found"))
	}
	return jsRuntimeSelection{Reason: strings.Join(reasons, " ")}
}

// getPreferredJSRuntime returns the --js-runtimes argument value for yt-dlp.
func getPreferredJSRuntime(i *I18n) string {
	return detectPreferredJSRuntime(i).Arg
}

// ensureYouTubeJSRuntime checks if a YouTube URL has a usable JS runtime; returns error if not.
func ensureYouTubeJSRuntime(i *I18n, rawURL string, settings Settings) error {
	if !isYouTubeURL(rawURL) {
		return nil
	}
	selection := detectPreferredJSRuntime(i)
	if selection.Found {
		return nil
	}
	cookieHint := describeCookieSource(i, settings)
	reason := selection.Reason
	if reason == "" {
		reason = i.T("runtime.missing_generic")
	}
	return fmt.Errorf(i.T("runtime.youtube.need_js"), cookieHint, reason, minimumDenoMajor, minimumNodeMajor)
}

// getNodeVersion returns a human-readable JS runtime version string.
func getNodeVersion() string {
	denoProbe := probeDenoRuntime(fallbackI18n())
	if denoProbe.Found {
		version := denoProbe.Version
		if version == "" {
			version = "deno"
		}
		if denoProbe.Supported {
			return fmt.Sprintf("%s (%s)", version, denoProbe.Path)
		}
		return fmt.Sprintf("%s (%s, unsupported: need >= %d.0.0)", version, denoProbe.Path, minimumDenoMajor)
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return "missing"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nodeVerCmd := exec.CommandContext(ctx, nodePath, "-v")
	platform.HideCmdWindow(nodeVerCmd)
	out, err := nodeVerCmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("found at %s but failed to run: %v", nodePath, err)
	}
	version := strings.TrimSpace(toUTF8(out))
	if version == "" {
		return fmt.Sprintf("found at %s but returned empty version", nodePath)
	}
	if major, ok := parseRuntimeMajorVersion(version); !ok || major < minimumNodeMajor {
		return fmt.Sprintf("%s (%s, unsupported: need >= %d.0.0)", version, nodePath, minimumNodeMajor)
	}
	return fmt.Sprintf("%s (%s)", version, nodePath)
}

// UpdateDeno installs or upgrades deno.
func (s *Service) UpdateDeno() (string, error) {
	denoProbe := probeDenoRuntime(s.i18n)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	var action string
	if denoProbe.Found && denoProbe.Path != "" {
		action = "upgrade"
		cmd = exec.CommandContext(ctx, denoProbe.Path, "upgrade")
	} else {
		action = "install"
		switch runtime.GOOS {
		case "windows":
			cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm https://deno.land/install.ps1 | iex")
		case "darwin", "linux":
			cmd = exec.CommandContext(ctx, "sh", "-c", "curl -fsSL https://deno.land/install.sh | sh")
		default:
			return "", fmt.Errorf("automatic Deno installation is not supported on %s", runtime.GOOS)
		}
	}

	cmd.Env = append(os.Environ(), "DENO_INSTALL_PROMPT=0")
	if s.hooks.HideCommand != nil {
		s.hooks.HideCommand(cmd)
	}
	if action == "upgrade" {
		s.emitLog("[UpdateDeno] upgrading Deno from: %s", denoProbe.Path)
	} else {
		s.emitLog("[UpdateDeno] installing Deno for OS: %s", runtime.GOOS)
	}
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(toUTF8(out))
	if output == "" {
		if action == "upgrade" {
			output = fmt.Sprintf("Deno %s finished. Please restart the app to refresh runtime detection.", action)
		} else {
			output = "Deno installation finished. Please restart the app to refresh runtime detection."
		}
	}
	if err != nil {
		return output, fmt.Errorf("deno %s failed: %w", action, err)
	}
	if !strings.Contains(strings.ToLower(output), "restart") {
		output = output + "\n\nPlease restart the app to refresh runtime detection."
	}
	return output, nil
}

// fallbackI18n returns a default I18n instance for contexts without a Service (e.g. diagnostics).
func fallbackI18n() *I18n {
	return NewI18n()
}
