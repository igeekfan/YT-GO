package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	minimumDenoMajor = 2
	minimumNodeMajor = 20
)

type runtimeProbe struct {
	Name      string
	Path      string
	Version   string
	Supported bool
	Found     bool
	Reason    string
	Arg       string
}

func isNodeVersionSufficient(nodePath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, nodePath, "-v").CombinedOutput()
	if err != nil {
		return false
	}
	major, ok := parseRuntimeMajorVersion(strings.TrimSpace(toUTF8(out)))
	return ok && major >= minimumNodeMajor
}

type jsRuntimeSelection struct {
	Arg     string
	Name    string
	Version string
	Path    string
	Found   bool
	Reason  string
}

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

func probeDenoRuntime() runtimeProbe {
	probe := runtimeProbe{Name: "deno"}
	denoPath, err := exec.LookPath("deno")
	if err != nil || denoPath == "" {
		return probe
	}
	probe.Found = true
	probe.Path = denoPath
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, runErr := exec.CommandContext(ctx, denoPath, "--version").CombinedOutput()
	if runErr != nil {
		probe.Reason = fmt.Sprintf("当前检测到 Deno 路径 %s，但无法正常执行。", denoPath)
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
	probe.Reason = fmt.Sprintf("当前检测到 Deno %s，但版本过低，yt-dlp EJS 至少需要 Deno %d.0.0。", detectedVersion, minimumDenoMajor)
	return probe
}

func probeNodeRuntime() runtimeProbe {
	probe := runtimeProbe{Name: "node"}
	nodePath, err := exec.LookPath("node")
	if err != nil || nodePath == "" {
		return probe
	}
	probe.Found = true
	probe.Path = nodePath
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, runErr := exec.CommandContext(ctx, nodePath, "-v").CombinedOutput()
	if runErr != nil {
		probe.Reason = fmt.Sprintf("当前检测到 Node.js 路径 %s，但无法正常执行。", nodePath)
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
	probe.Reason = fmt.Sprintf("当前检测到 Node.js %s，但版本过低，yt-dlp EJS 至少需要 Node.js %d.0.0。", detectedVersion, minimumNodeMajor)
	return probe
}

func detectPreferredJSRuntime() jsRuntimeSelection {
	denoProbe := probeDenoRuntime()
	if denoProbe.Supported {
		return jsRuntimeSelection{Arg: denoProbe.Arg, Name: denoProbe.Name, Version: denoProbe.Version, Path: denoProbe.Path, Found: true}
	}

	nodeProbe := probeNodeRuntime()
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
		reasons = append(reasons, "当前未检测到可用的 Deno 或 Node.js。")
	}
	return jsRuntimeSelection{Reason: strings.Join(reasons, " ")}
}

func getPreferredJSRuntime() string {
	return detectPreferredJSRuntime().Arg
}

func (s *Service) ytdlpCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.ytdlpPath, args...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	if s.hooks.HideCommand != nil {
		s.hooks.HideCommand(cmd)
	}
	return cmd
}

func (s *Service) ytdlpMediaCmd(ctx context.Context, args ...string) *exec.Cmd {
	if jsRuntime := getPreferredJSRuntime(); jsRuntime != "" {
		args = append([]string{"--js-runtimes", jsRuntime}, args...)
	}
	return s.ytdlpCmd(ctx, args...)
}

func toUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), b)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	decoded, _, err = transform.Bytes(charmap.Windows1252.NewDecoder(), b)
	if err == nil {
		return string(decoded)
	}
	return strings.ToValidUTF8(string(b), "\ufffd")
}

func appendCookiesArgs(args []string, settings Settings) []string {
	if settings.CookiesFrom != "" {
		return append(args, "--cookies-from-browser", settings.CookiesFrom)
	}
	if settings.CookiesFile != "" {
		return append(args, "--cookies", settings.CookiesFile)
	}
	return args
}

func describeCookieSource(settings Settings) string {
	if settings.CookiesFrom != "" {
		return fmt.Sprintf("当前使用浏览器 Cookies: %s。", settings.CookiesFrom)
	}
	if settings.CookiesFile != "" {
		return fmt.Sprintf("当前使用 cookies 文件: %s。", settings.CookiesFile)
	}
	return ""
}

func isYouTubeURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "youtube.com" || host == "www.youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be" || host == "www.youtu.be" || host == "youtube-nocookie.com" || strings.HasSuffix(host, ".youtube-nocookie.com")
}

func ensureYouTubeJSRuntime(rawURL string, settings Settings) error {
	if !isYouTubeURL(rawURL) {
		return nil
	}
	selection := detectPreferredJSRuntime()
	if selection.Found {
		return nil
	}
	cookieHint := describeCookieSource(settings)
	reason := selection.Reason
	if reason == "" {
		reason = "当前缺少可用的 JS runtime。"
	}
	return fmt.Errorf("当前链接属于 YouTube，yt-dlp 需要可用的 JS runtime 才能完成签名/JS challenge 求解。%s%s请升级到 Deno >= %d.0.0（推荐）或 Node.js >= %d.0.0，并在安装后完全重启应用再重试。", cookieHint, reason, minimumDenoMajor, minimumNodeMajor)
}

func buildChallengeFailureHint(settings Settings) string {
	cookieHint := describeCookieSource(settings)
	selection := detectPreferredJSRuntime()
	if selection.Found {
		runtimeLabel := selection.Name
		if selection.Version != "" {
			runtimeLabel = fmt.Sprintf("%s %s", selection.Name, selection.Version)
		}
		return fmt.Sprintf("yt-dlp 已读取到 YouTube 页面，但当前环境仍未完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s当前已检测到可用的 %s。请先更新 yt-dlp 到最新版本；如果仍失败，再按 yt-dlp 的 EJS 指南检查 challenge solver 组件来源。", cookieHint, runtimeLabel)
	}
	return fmt.Sprintf("yt-dlp 已读取到 YouTube 页面，但当前环境无法完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s%s请升级到 Deno >= %d.0.0（推荐）或 Node.js >= %d.0.0，并在安装后重启应用。", cookieHint, selection.Reason, minimumDenoMajor, minimumNodeMajor)
}

func normalizeYtDlpError(errMsg string, settings Settings) string {
	errMsg = strings.TrimSpace(errMsg)
	if errMsg == "" {
		return errMsg
	}
	if strings.Contains(errMsg, "Fresh cookies") && strings.Contains(errMsg, "Douyin") {
		cookieHint := "当前未配置抖音 Cookies。"
		if hint := describeCookieSource(settings); hint != "" {
			cookieHint = hint
		}
		return fmt.Sprintf("抖音需要有效的登录 Cookies 才能访问该视频。%s请登录 www.douyin.com 后，使用浏览器扩展导出 cookies.txt 并在设置中配置，或改用\u300c从浏览器导入 Cookies\u300d。原始错误: %s", cookieHint, errMsg)
	}
	if strings.Contains(errMsg, "Sign in to confirm") || strings.Contains(errMsg, "not a bot") {
		cookieHint := "当前未配置 Cookies。"
		if hint := describeCookieSource(settings); hint != "" {
			cookieHint = hint
		}
		return fmt.Sprintf("YouTube 拒绝了当前访问，请求被判定为需要登录验证。%s这通常表示 Cookies 已过期、导出不完整、账号未登录 YouTube，或当前代理/IP 风险较高。请重新导出最新的 YouTube cookies.txt，或改用“从浏览器导入 Cookies”。原始错误: %s", cookieHint, errMsg)
	}
	if strings.Contains(errMsg, "Failed to decrypt with DPAPI") {
		if settings.CookiesFrom != "" {
			return fmt.Sprintf("无法读取浏览器 Cookies（DPAPI 解密失败）。当前浏览器来源: %s。请先关闭该浏览器，并确保 YT-GO 不是以管理员身份运行；如果仍失败，请改用导出的 cookies.txt 文件。原始错误: %s", settings.CookiesFrom, errMsg)
		}
		return fmt.Sprintf("Cookies 解密失败（DPAPI）。请检查是否以相同 Windows 用户运行，并避免管理员身份运行；必要时改用导出的 cookies.txt 文件。原始错误: %s", errMsg)
	}
	if strings.Contains(errMsg, "Signature solving failed") || strings.Contains(errMsg, "n challenge solving failed") || strings.Contains(errMsg, "Only images are available for download") || strings.Contains(errMsg, "Requested format is not available") {
		return fmt.Sprintf("%s 原始错误: %s", buildChallengeFailureHint(settings), errMsg)
	}
	return errMsg
}

func getNodeVersion() string {
	denoProbe := probeDenoRuntime()
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
	out, err := exec.CommandContext(ctx, nodePath, "-v").CombinedOutput()
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

func (s *Service) logCmd(tag string, cmd *exec.Cmd) {
	s.emitLog("[%s] exec: %s", tag, strings.Join(cmd.Args, " "))
}

func (s *Service) findYtDlp() string {
	candidates := []string{"yt-dlp", "yt-dlp.exe"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		for _, name := range candidates {
			path := filepath.Join(execDir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	var extraDirs []string
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		wingetPackages := filepath.Join(localApp, "Microsoft", "WinGet", "Packages")
		if entries, err := os.ReadDir(wingetPackages); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && strings.HasPrefix(entry.Name(), "yt-dlp.yt-dlp") {
					extraDirs = append(extraDirs, filepath.Join(wingetPackages, entry.Name()))
				}
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		extraDirs = append(extraDirs, filepath.Join(home, "scoop", "shims"))
	}
	for _, dir := range extraDirs {
		for _, name := range candidates {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

var shareTextURLRe = regexp.MustCompile(`https?://\S+`)

// extractURLFromText returns the embedded URL from a noisy share-text snippet
// (e.g. a Douyin/TikTok copy-link share string).  If the input has no
// whitespace it is already a plain URL and is returned trimmed as-is.
func extractURLFromText(input string) string {
	trimmed := strings.TrimSpace(input)
	if !strings.ContainsAny(trimmed, " \t\r\n") {
		return trimmed
	}
	m := shareTextURLRe.FindString(trimmed)
	if m == "" {
		return trimmed
	}
	return strings.TrimRight(m, ".,;:!?)]}，。；：！？、）】》」』")
}

func (s *Service) CheckYtDlp() YtDlpStatus {
	if s.ytdlpPath == "" {
		s.ytdlpPath = s.findYtDlp()
	}
	if s.ytdlpPath == "" {
		return YtDlpStatus{Available: false}
	}
	cmd := exec.Command(s.ytdlpPath, "--version")
	if s.hooks.HideCommand != nil {
		s.hooks.HideCommand(cmd)
	}
	out, err := cmd.Output()
	if err != nil {
		return YtDlpStatus{Available: false}
	}
	return YtDlpStatus{Available: true, Version: strings.TrimSpace(string(out)), Path: s.ytdlpPath}
}

func (s *Service) UpdateYtDlp() (string, error) {
	if s.ytdlpPath == "" {
		return "", fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := s.ytdlpCmd(ctx, "-U")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(toUTF8(out))
	if err != nil {
		return output, fmt.Errorf("update failed: %w", err)
	}
	return output, nil
}

func (s *Service) UpdateDeno() (string, error) {
	denoProbe := probeDenoRuntime()
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

func (s *Service) GetVideoInfo(rawInput string) (VideoInfo, error) {
	url := extractURLFromText(rawInput)
	if isDouyinURL(url) {
		return s.GetDouyinVideoInfo(url)
	}
	if s.ytdlpPath == "" {
		return VideoInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(url, settings); err != nil {
		s.emitLog("[GetVideoInfo] preflight failed: %v", err)
		return VideoInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"--ignore-config", "--dump-json", "--no-playlist", "--no-warnings"}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, url)
	cmd := s.ytdlpMediaCmd(ctx, args...)
	s.emitLog("[GetVideoInfo] fetching info for URL: %s", url)
	s.logCmd("GetVideoInfo", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(toUTF8(out), settings)
		s.emitLog("[GetVideoInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return VideoInfo{}, fmt.Errorf("%s", errMsg)
		}
		return VideoInfo{}, fmt.Errorf("failed to get video info: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		s.emitLog("[GetVideoInfo] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return VideoInfo{}, fmt.Errorf("failed to parse video info: %w", err)
	}
	info := VideoInfo{URL: url}
	if v, ok := raw["title"].(string); ok {
		info.Title = v
	}
	if v, ok := raw["id"].(string); ok {
		info.ID = v
	}
	info.Thumbnail = extractThumbnailURL(raw)
	if v, ok := raw["duration"].(float64); ok {
		info.Duration = v
	}
	if v, ok := raw["uploader"].(string); ok {
		info.Uploader = v
	} else if v, ok := raw["channel"].(string); ok {
		info.Uploader = v
	}
	if v, ok := raw["extractor_key"].(string); ok {
		info.Platform = v
	} else if v, ok := raw["extractor"].(string); ok {
		info.Platform = v
	}
	if v, ok := raw["webpage_url"].(string); ok && v != "" {
		info.URL = v
	}
	if fallback := s.resolveVideoThumbnailFallback(info, raw); fallback != "" {
		info.Thumbnail = fallback
	}
	info.Subtitles = extractSubtitleLangs(raw)
	return info, nil
}

func extractSubtitleLangs(raw map[string]interface{}) []SubtitleLang {
	var result []SubtitleLang
	seen := make(map[string]bool)
	if subs, ok := raw["subtitles"].(map[string]interface{}); ok {
		for code := range subs {
			if seen[code] {
				continue
			}
			seen[code] = true
			name := code
			if arr, ok := subs[code].([]interface{}); ok && len(arr) > 0 {
				if obj, ok := arr[0].(map[string]interface{}); ok {
					if nameValue, ok := obj["name"].(string); ok && nameValue != "" {
						name = nameValue
					}
				}
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: false})
		}
	}
	if autoCaptions, ok := raw["automatic_captions"].(map[string]interface{}); ok {
		for code := range autoCaptions {
			if seen[code] {
				continue
			}
			seen[code] = true
			name := code
			if arr, ok := autoCaptions[code].([]interface{}); ok && len(arr) > 0 {
				if obj, ok := arr[0].(map[string]interface{}); ok {
					if nameValue, ok := obj["name"].(string); ok && nameValue != "" {
						name = nameValue
					}
				}
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: true})
		}
	}
	return result
}

func detectCollectionKind(url string) string {
	lower := strings.ToLower(url)
	if strings.Contains(lower, "/@") || strings.Contains(lower, "/channel/") || strings.Contains(lower, "/user/") || strings.Contains(lower, "/c/") {
		return "channel"
	}
	if strings.Contains(lower, "bilibili.com") && (strings.Contains(lower, "/favlist") || strings.Contains(lower, "/medialist") || strings.Contains(lower, "/channel/seriesdetail")) {
		return "channel"
	}
	return "playlist"
}

func (s *Service) GetPlaylistInfo(rawInput string) (PlaylistInfo, error) {
	url := extractURLFromText(rawInput)
	if isDouyinURL(url) {
		info, err := s.GetDouyinVideoInfo(url)
		if err == nil {
			return PlaylistInfo{
				URL:    url,
				Kind:   "playlist",
				Title:  info.Title,
				Count:  1,
				Videos: []VideoInfo{info},
			}, nil
		}
		// Custom Douyin handler failed; fall through to yt-dlp
		s.emitLog("[GetPlaylistInfo] Douyin custom handler failed (%v), falling back to yt-dlp", err)
	}
	if s.ytdlpPath == "" {
		return PlaylistInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(url, settings); err != nil {
		s.emitLog("[GetPlaylistInfo] preflight failed: %v", err)
		return PlaylistInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	args := []string{"--ignore-config", "--flat-playlist", "--dump-single-json", "--no-warnings"}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, url)
	cmd := s.ytdlpMediaCmd(ctx, args...)
	s.emitLog("[GetPlaylistInfo] fetching playlist for URL: %s", url)
	s.logCmd("GetPlaylistInfo", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(toUTF8(out), settings)
		s.emitLog("[GetPlaylistInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return PlaylistInfo{}, fmt.Errorf("%s", errMsg)
		}
		return PlaylistInfo{}, fmt.Errorf("failed to get playlist info: %w", err)
	}
	result := PlaylistInfo{URL: url, Kind: detectCollectionKind(url)}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		s.emitLog("[GetPlaylistInfo] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return PlaylistInfo{}, fmt.Errorf("failed to parse playlist info: %w", err)
	}
	if v, ok := raw["title"].(string); ok {
		result.Title = v
	}
	if v, ok := raw["playlist_title"].(string); ok && v != "" {
		result.Title = v
	}
	if v, ok := raw["uploader"].(string); ok {
		result.Uploader = v
	} else if v, ok := raw["playlist_uploader"].(string); ok {
		result.Uploader = v
	} else if v, ok := raw["channel"].(string); ok {
		result.Uploader = v
	}
	if result.Kind == "playlist" {
		if extractor, ok := raw["extractor_key"].(string); ok && strings.Contains(strings.ToLower(extractor), "tab") {
			lowerTitle := strings.ToLower(result.Title)
			if strings.Contains(lowerTitle, "channel") || strings.Contains(lowerTitle, "videos") {
				result.Kind = "channel"
			}
		}
	}
	if entries, ok := raw["entries"].([]interface{}); ok {
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			info := VideoInfo{}
			if v, ok := entryMap["webpage_url"].(string); ok {
				info.URL = v
			} else if v, ok := entryMap["url"].(string); ok {
				info.URL = v
			}
			if v, ok := entryMap["title"].(string); ok {
				info.Title = v
			}
			if v, ok := entryMap["id"].(string); ok {
				info.ID = v
			}
			info.Thumbnail = extractThumbnailURL(entryMap)
			if v, ok := entryMap["duration"].(float64); ok {
				info.Duration = v
			}
			if v, ok := entryMap["uploader"].(string); ok {
				info.Uploader = v
			} else if v, ok := entryMap["channel"].(string); ok {
				info.Uploader = v
			}
			if info.URL != "" || info.ID != "" {
				result.Videos = append(result.Videos, info)
			}
		}
	}
	result.Count = len(result.Videos)
	return result, nil
}

func (s *Service) GetFormats(rawInput string) (FormatInfo, error) {
	url := extractURLFromText(rawInput)
	if isDouyinURL(url) {
		return s.GetDouyinFormats(url)
	}
	if s.ytdlpPath == "" {
		return FormatInfo{}, fmt.Errorf("yt-dlp not found")
	}
	settings := s.GetSettings()
	if err := ensureYouTubeJSRuntime(url, settings); err != nil {
		s.emitLog("[GetFormats] preflight failed: %v", err)
		return FormatInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"--ignore-config", "--dump-json", "--no-download", "--no-warnings", "--no-playlist"}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, url)
	cmd := s.ytdlpMediaCmd(ctx, args...)
	s.emitLog("[GetFormats] fetching formats for URL: %s", url)
	s.logCmd("GetFormats", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(toUTF8(out), settings)
		s.emitLog("[GetFormats] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return FormatInfo{}, fmt.Errorf("%s", errMsg)
		}
		return FormatInfo{}, fmt.Errorf("failed to get formats: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		s.emitLog("[GetFormats] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return FormatInfo{}, fmt.Errorf("failed to parse JSON: %w", err)
	}
	result := FormatInfo{URL: url}
	if v, ok := raw["title"].(string); ok {
		result.Title = v
	}
	if formatsRaw, ok := raw["formats"].([]interface{}); ok {
		for _, rawFormat := range formatsRaw {
			formatMap, ok := rawFormat.(map[string]interface{})
			if !ok {
				continue
			}
			format := Format{}
			if v, ok := formatMap["format_id"].(string); ok {
				format.FormatID = v
			}
			if v, ok := formatMap["ext"].(string); ok {
				format.Ext = v
			}
			if v, ok := formatMap["resolution"].(string); ok {
				format.Resolution = v
			}
			if v, ok := formatMap["fps"].(float64); ok {
				format.FPS = v
			}
			if v, ok := formatMap["vcodec"].(string); ok {
				format.VCodec = v
				format.HasVideo = v != "none" && v != ""
			}
			if v, ok := formatMap["acodec"].(string); ok {
				format.ACodec = v
				format.HasAudio = v != "none" && v != ""
			}
			if v, ok := formatMap["filesize"].(float64); ok {
				format.Filesize = int64(v)
			} else if v, ok := formatMap["filesize_approx"].(float64); ok {
				format.Filesize = int64(v)
			}
			if v, ok := formatMap["tbr"].(float64); ok {
				format.TBR = v
			}
			if v, ok := formatMap["format_note"].(string); ok {
				format.Note = v
			}
			result.Formats = append(result.Formats, format)
		}
	}
	return result, nil
}

func qualityArgs(quality string) []string {
	if len(quality) > 3 && quality[:3] == "fa:" {
		return []string{"-f", quality[3:], "-x"}
	}
	if len(quality) > 3 && quality[:3] == "fv:" {
		return []string{"-f", quality[3:]}
	}
	if len(quality) > 2 && quality[:2] == "f:" {
		return []string{"-f", quality[2:]}
	}
	switch quality {
	case "1080p":
		return []string{"-f", "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]"}
	case "720p":
		return []string{"-f", "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=720]+bestaudio/best[height<=720]"}
	case "480p":
		return []string{"-f", "bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=480]+bestaudio/best[height<=480]"}
	case "360p":
		return []string{"-f", "bestvideo[height<=360][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=360]+bestaudio/best[height<=360]"}
	case "audio":
		return []string{"-f", "bestaudio/best", "-x"}
	default:
		return []string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best"}
	}
}

func extractThumbnailURL(raw map[string]interface{}) string {
	for _, key := range []string{"thumbnail", "pic", "cover"} {
		if value, ok := raw[key].(string); ok {
			if normalized := normalizeThumbnailURL(value); normalized != "" {
				if isPlaceholderThumbnailURL(normalized) {
					continue
				}
				return normalized
			}
		}
	}
	thumbnails, ok := raw["thumbnails"].([]interface{})
	if !ok {
		return ""
	}
	for index := len(thumbnails) - 1; index >= 0; index-- {
		thumbMap, ok := thumbnails[index].(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range []string{"url", "src"} {
			if value, ok := thumbMap[key].(string); ok {
				if normalized := normalizeThumbnailURL(value); normalized != "" {
					if isPlaceholderThumbnailURL(normalized) {
						continue
					}
					return normalized
				}
			}
		}
	}
	return ""
}

func normalizeThumbnailURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	return value
}

func isPlaceholderThumbnailURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return lower == "" || strings.Contains(lower, "/transparent.png")
}

func (s *Service) resolveVideoThumbnailFallback(info VideoInfo, raw map[string]interface{}) string {
	if info.Thumbnail != "" && !isPlaceholderThumbnailURL(info.Thumbnail) {
		return ""
	}
	if !isBilibiliVideo(info, raw) {
		return ""
	}
	bvid := extractBilibiliBVID(info, raw)
	if bvid == "" {
		return ""
	}
	thumbnail, err := s.fetchBilibiliThumbnail(bvid)
	if err != nil {
		s.emitLog("[GetVideoInfo] bilibili thumbnail fallback failed: %v", err)
		return ""
	}
	return thumbnail
}

func isBilibiliVideo(info VideoInfo, raw map[string]interface{}) bool {
	platform := strings.ToLower(info.Platform)
	videoURL := strings.ToLower(info.URL)
	if strings.Contains(platform, "bilibili") || strings.Contains(videoURL, "bilibili.com") {
		return true
	}
	if extractor, ok := raw["extractor_key"].(string); ok && strings.Contains(strings.ToLower(extractor), "bilibili") {
		return true
	}
	return false
}

func extractBilibiliBVID(info VideoInfo, raw map[string]interface{}) string {
	if id := strings.TrimSpace(info.ID); strings.HasPrefix(strings.ToUpper(id), "BV") {
		return id
	}
	for _, key := range []string{"bvid", "display_id"} {
		if value, ok := raw[key].(string); ok {
			value = strings.TrimSpace(value)
			if strings.HasPrefix(strings.ToUpper(value), "BV") {
				return value
			}
		}
	}
	match := regexp.MustCompile(`(?i)BV[0-9A-Za-z]+`).FindString(info.URL)
	return strings.TrimSpace(match)
}

func (s *Service) fetchBilibiliThumbnail(bvid string) (string, error) {
	settings := s.GetSettings()
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if settings.Proxy != "" {
		proxyURL, err := url.Parse(settings.Proxy)
		if err != nil {
			return "", fmt.Errorf("invalid proxy for bilibili thumbnail: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	endpoint := "https://api.bilibili.com/x/web-interface/view?bvid=" + url.QueryEscape(bvid)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/video/"+bvid)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Pic string `json:"pic"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	thumbnail := normalizeThumbnailURL(payload.Data.Pic)
	if payload.Code != 0 || thumbnail == "" || isPlaceholderThumbnailURL(thumbnail) {
		return "", fmt.Errorf("empty thumbnail in bilibili api")
	}
	return thumbnail, nil
}
