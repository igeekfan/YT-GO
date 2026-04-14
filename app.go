package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx         context.Context
	i18n        *I18n
	downloads   map[string]*DownloadTask
	cancelFns   map[string]context.CancelFunc
	mu          sync.RWMutex
	ytdlpPath   string
	db          *gorm.DB
	downloadSem chan struct{} // semaphore for concurrent download limiting
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		downloads:   make(map[string]*DownloadTask),
		cancelFns:   make(map[string]context.CancelFunc),
		i18n:        NewI18n(),
		downloadSem: make(chan struct{}, 3), // default max 3 concurrent downloads
	}
	app.ytdlpPath = app.findYtDlp()
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if db, err := openDB(); err == nil {
		a.db = db
		a.cleanupTransientDownloads()
		a.loadFromDB()
		// Initialize download semaphore based on settings
		settings := a.GetSettings()
		maxConcurrent := settings.MaxConcurrent
		if maxConcurrent < 1 {
			maxConcurrent = 3 // default
		}
		if maxConcurrent > 10 {
			maxConcurrent = 10 // cap at 10
		}
		a.downloadSem = make(chan struct{}, maxConcurrent)
	}
}

// cleanupTransientDownloads removes stale transient tasks left by a previous session.
// Pending/downloading/cancelled tasks should not survive an application restart.
func (a *App) cleanupTransientDownloads() {
	if a.db == nil {
		return
	}
	a.db.Where("status IN ?", []string{"pending", "downloading", "cancelled"}).Delete(&DownloadRecord{})
}

// loadFromDB loads all persisted download records into the in-memory map.
func (a *App) loadFromDB() {
	if a.db == nil {
		return
	}
	var records []DownloadRecord
	if err := a.db.Order("created_at desc").Find(&records).Error; err != nil {
		return
	}
	a.mu.Lock()
	for _, r := range records {
		t := recordToTask(r)
		a.downloads[t.ID] = t
	}
	a.mu.Unlock()
}

// upsertRecord saves or updates a single task in the database.
func (a *App) upsertRecord(t *DownloadTask) {
	if a.db == nil {
		return
	}
	rec := taskToRecord(t)
	a.db.Save(&rec)
}

// deleteRecords removes all completed/error/cancelled records from the database.
func (a *App) deleteRecords(ids []string) {
	if a.db == nil || len(ids) == 0 {
		return
	}
	a.db.Delete(&DownloadRecord{}, ids)
}

// emitLog sends a log message to the frontend via the "app:log" event.
func (a *App) emitLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "app:log", msg)
	}
}

func getPreferredJSRuntime() string {
	nodePath, err := exec.LookPath("node")
	if err == nil && nodePath != "" {
		return "node:" + nodePath
	}
	return ""
}

// ytdlpCmd creates an exec.Cmd for yt-dlp with UTF-8 output encoding on Windows.
func (a *App) ytdlpCmd(ctx context.Context, args ...string) *exec.Cmd {
	if runtime := getPreferredJSRuntime(); runtime != "" {
		args = append([]string{"--js-runtimes", runtime}, args...)
	}
	cmd := exec.CommandContext(ctx, a.ytdlpPath, args...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	return cmd
}

// toUTF8 ensures the byte slice is valid UTF-8.
// If it already is valid UTF-8, returns as-is.
// Otherwise tries common Windows encodings used by yt-dlp output.
func toUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	// Try GB18030/CP936 first for Chinese Windows consoles.
	decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), b)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	// Try Windows-1252 (superset of ISO-8859-1), which covers most Western yt-dlp output
	decoded, _, err = transform.Bytes(charmap.Windows1252.NewDecoder(), b)
	if err == nil {
		return string(decoded)
	}
	// Fallback: replace invalid bytes
	return strings.ToValidUTF8(string(b), "\ufffd")
}

// appendCookiesArgs adds --cookies-from-browser or --cookies flags based on settings.
func appendCookiesArgs(args []string, settings Settings) []string {
	if settings.CookiesFrom != "" {
		args = append(args, "--cookies-from-browser", settings.CookiesFrom)
	} else if settings.CookiesFile != "" {
		args = append(args, "--cookies", settings.CookiesFile)
	}
	return args
}

func normalizeYtDlpError(errMsg string, settings Settings) string {
	errMsg = strings.TrimSpace(errMsg)
	if errMsg == "" {
		return errMsg
	}

	if strings.Contains(errMsg, "Sign in to confirm") || strings.Contains(errMsg, "not a bot") {
		cookieHint := "当前未配置 Cookies。"
		if settings.CookiesFrom != "" {
			cookieHint = fmt.Sprintf("当前使用浏览器 Cookies: %s。", settings.CookiesFrom)
		} else if settings.CookiesFile != "" {
			cookieHint = fmt.Sprintf("当前使用 cookies 文件: %s。", settings.CookiesFile)
		}
		return fmt.Sprintf("YouTube 拒绝了当前访问，请求被判定为需要登录验证。%s这通常表示 Cookies 已过期、导出不完整、账号未登录 YouTube，或当前代理/IP 风险较高。请重新导出最新的 YouTube cookies.txt，或改用“从浏览器导入 Cookies”。原始错误: %s", cookieHint, errMsg)
	}

	if strings.Contains(errMsg, "Failed to decrypt with DPAPI") {
		if settings.CookiesFrom != "" {
			return fmt.Sprintf("无法读取浏览器 Cookies（DPAPI 解密失败）。当前浏览器来源: %s。请先关闭该浏览器，并确保 YT-GO 不是以管理员身份运行；如果仍失败，请改用导出的 cookies.txt 文件。原始错误: %s", settings.CookiesFrom, errMsg)
		}
		return fmt.Sprintf("Cookies 解密失败（DPAPI）。请检查是否以相同 Windows 用户运行，并避免管理员身份运行；必要时改用导出的 cookies.txt 文件。原始错误: %s", errMsg)
	}

	if strings.Contains(errMsg, "Signature solving failed") ||
		strings.Contains(errMsg, "n challenge solving failed") ||
		strings.Contains(errMsg, "Only images are available for download") ||
		strings.Contains(errMsg, "Requested format is not available") {
		cookieHint := ""
		if settings.CookiesFrom != "" {
			cookieHint = fmt.Sprintf("当前使用浏览器 Cookies: %s。", settings.CookiesFrom)
		} else if settings.CookiesFile != "" {
			cookieHint = fmt.Sprintf("当前使用 cookies 文件: %s。", settings.CookiesFile)
		}
		return fmt.Sprintf("yt-dlp 已读取到 YouTube 页面，但当前环境无法完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s请安装 Node.js LTS 并重启应用，或改用具备可用 JS runtime 的 yt-dlp 运行环境。原始错误: %s", cookieHint, errMsg)
	}

	return errMsg
}

func getNodeVersion() string {
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
	return fmt.Sprintf("%s (%s)", version, nodePath)
}

// logCmd logs the full yt-dlp command line for debugging.
func (a *App) logCmd(tag string, cmd *exec.Cmd) {
	a.emitLog("[%s] exec: %s", tag, strings.Join(cmd.Args, " "))
}

// GetSettings returns persisted user settings
func (a *App) GetSettings() Settings {
	defaults := Settings{
		OutputDir:         a.GetDefaultDownloadDir(),
		Quality:           "best",
		Language:          "zh-CN",
		Theme:             "dark",
		Proxy:             "",
		RateLimit:         "",
		MaxConcurrent:     3,
		Notifications:     true,
		SaveDescription:   false,
		SaveThumbnail:     false,
		WriteSubtitles:    false,
		SubtitleLangs:     "",
		EmbedSubtitles:    false,
		EmbedChapters:     false,
		SponsorBlock:      false,
		FilenameTemplate:  "",
		MergeOutputFormat: "",
		AudioFormat:       "",
	}
	if a.db == nil {
		return defaults
	}
	var rec SettingsRecord
	if err := a.db.First(&rec, 1).Error; err != nil {
		return defaults
	}
	// Merge stored values with defaults (empty fields use defaults)
	if rec.OutputDir != "" {
		defaults.OutputDir = rec.OutputDir
	}
	if rec.Quality != "" {
		defaults.Quality = rec.Quality
	}
	if rec.Language != "" {
		defaults.Language = rec.Language
	}
	if rec.Theme != "" {
		defaults.Theme = rec.Theme
	}
	defaults.Proxy = rec.Proxy
	defaults.RateLimit = rec.RateLimit
	if rec.MaxConcurrent > 0 {
		defaults.MaxConcurrent = rec.MaxConcurrent
	}
	defaults.Notifications = rec.Notifications
	defaults.SaveDescription = rec.SaveDescription
	defaults.SaveThumbnail = rec.SaveThumbnail
	defaults.WriteSubtitles = rec.WriteSubtitles
	defaults.SubtitleLangs = rec.SubtitleLangs
	defaults.EmbedSubtitles = rec.EmbedSubtitles
	defaults.EmbedChapters = rec.EmbedChapters
	defaults.SponsorBlock = rec.SponsorBlock
	defaults.FilenameTemplate = rec.FilenameTemplate
	defaults.MergeOutputFormat = rec.MergeOutputFormat
	defaults.AudioFormat = rec.AudioFormat
	defaults.CookiesFrom = rec.CookiesFrom
	defaults.CookiesFile = rec.CookiesFile
	return defaults
}

// IsFirstRun returns true if this is the first time the app is run (no settings saved)
func (a *App) IsFirstRun() bool {
	if a.db == nil {
		return true
	}
	var rec SettingsRecord
	if err := a.db.First(&rec, 1).Error; err != nil {
		return true
	}
	return false
}

// NeedsCookieConfig returns true if user needs to configure cookies or proxy
func (a *App) NeedsCookieConfig() bool {
	s := a.GetSettings()
	return s.CookiesFrom == "" && s.CookiesFile == "" && s.Proxy == ""
}

// SaveSettings persists user settings to the database
func (a *App) SaveSettings(s Settings) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	rec := SettingsRecord{
		ID:                1,
		OutputDir:         s.OutputDir,
		Quality:           s.Quality,
		Language:          s.Language,
		Theme:             s.Theme,
		Proxy:             s.Proxy,
		RateLimit:         s.RateLimit,
		MaxConcurrent:     s.MaxConcurrent,
		Notifications:     s.Notifications,
		SaveDescription:   s.SaveDescription,
		SaveThumbnail:     s.SaveThumbnail,
		WriteSubtitles:    s.WriteSubtitles,
		SubtitleLangs:     s.SubtitleLangs,
		EmbedSubtitles:    s.EmbedSubtitles,
		EmbedChapters:     s.EmbedChapters,
		SponsorBlock:      s.SponsorBlock,
		FilenameTemplate:  s.FilenameTemplate,
		MergeOutputFormat: s.MergeOutputFormat,
		AudioFormat:       s.AudioFormat,
		CookiesFrom:       s.CookiesFrom,
		CookiesFile:       s.CookiesFile,
	}
	return a.db.Save(&rec).Error
}

// DiagnosticInfo contains debug information about the application state
type DiagnosticInfo struct {
	YtDlpPath     string `json:"ytdlpPath"`
	YtDlpVersion  string `json:"ytdlpVersion"`
	YtDlpFound    bool   `json:"ytdlpFound"`
	FFmpegPath    string `json:"ffmpegPath"`
	FFmpegVersion string `json:"ffmpegVersion"`
	FFmpegFound   bool   `json:"ffmpegFound"`
	NodeVersion   string `json:"nodeVersion"`
	AppVersion    string `json:"appVersion"`
	TestOutput    string `json:"testOutput"`
	Error         string `json:"error"`
}

// GetDiagnosticInfo returns debug information to help troubleshoot issues
func (a *App) GetDiagnosticInfo() DiagnosticInfo {
	info := DiagnosticInfo{
		YtDlpPath:  a.ytdlpPath,
		YtDlpFound: a.ytdlpPath != "",
		AppVersion: AppVersion,
	}

	if a.ytdlpPath == "" {
		// Try to find it again
		a.ytdlpPath = a.findYtDlp()
		info.YtDlpPath = a.ytdlpPath
		info.YtDlpFound = a.ytdlpPath != ""
	}

	if a.ytdlpPath != "" {
		// Get version
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := a.ytdlpCmd(ctx, "--version").CombinedOutput()
		if err != nil {
			info.Error = fmt.Sprintf("version check failed: %v", err)
		} else {
			info.YtDlpVersion = strings.TrimSpace(string(out))
		}

		// Test a simple command
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		testOut, err := a.ytdlpCmd(ctx2, "--help").CombinedOutput()
		if err != nil {
			info.Error = fmt.Sprintf("help command failed: %v", err)
		} else {
			if strings.Contains(string(testOut), "youtube") || strings.Contains(string(testOut), "Usage:") {
				info.TestOutput = "yt-dlp is working correctly"
			} else {
				info.TestOutput = fmt.Sprintf("unexpected output (first 200 chars): %s", string(testOut)[:min(200, len(testOut))])
			}
		}
	} else {
		info.Error = "yt-dlp not found in PATH or common installation directories"
	}

	// FFmpeg detection
	info.FFmpegPath, info.FFmpegVersion, info.FFmpegFound = detectFFmpeg()

	// Node.js version
	info.NodeVersion = getNodeVersion()

	return info
}

// detectFFmpeg checks if FFmpeg is available and returns its path, version, and found status
func detectFFmpeg() (path string, version string, found bool) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffmpegPath, "-version").CombinedOutput()
	if err != nil {
		return ffmpegPath, "", true
	}
	output := strings.TrimSpace(string(out))
	// Extract version from first line: "ffmpeg version N-xxxxx-g... Copyright ..."
	if lines := strings.SplitN(output, "\n", 2); len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "ffmpeg version ") {
			parts := strings.Fields(firstLine)
			if len(parts) >= 3 {
				return ffmpegPath, parts[2], true
			}
		}
		return ffmpegPath, firstLine, true
	}
	return ffmpegPath, output, true
}

// SetLang sets the application language
func (a *App) SetLang(lang string) {
	a.i18n.SetLang(Lang(lang))
}

// GetLang returns the current language
func (a *App) GetLang() string {
	return string(a.i18n.GetLang())
}

// findYtDlp looks for yt-dlp in PATH and the executable directory
func (a *App) findYtDlp() string {
	candidates := []string{"yt-dlp", "yt-dlp.exe"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	// Check alongside the application binary
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		for _, name := range candidates {
			p := filepath.Join(execDir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	// Check common install locations not always in PATH
	var extraDirs []string
	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		// winget installs: %LOCALAPPDATA%\Microsoft\WinGet\Packages\yt-dlp.yt-dlp*\
		wingetPackages := filepath.Join(localApp, "Microsoft", "WinGet", "Packages")
		if entries, err := os.ReadDir(wingetPackages); err == nil {
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(e.Name(), "yt-dlp.yt-dlp") {
					extraDirs = append(extraDirs, filepath.Join(wingetPackages, e.Name()))
				}
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		// scoop: ~/scoop/shims/
		extraDirs = append(extraDirs, filepath.Join(home, "scoop", "shims"))
	}
	for _, dir := range extraDirs {
		for _, name := range candidates {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// CheckYtDlp verifies yt-dlp is available and returns its version
func (a *App) CheckYtDlp() YtDlpStatus {
	if a.ytdlpPath == "" {
		a.ytdlpPath = a.findYtDlp()
	}
	if a.ytdlpPath == "" {
		return YtDlpStatus{Available: false}
	}
	out, err := exec.Command(a.ytdlpPath, "--version").Output()
	if err != nil {
		return YtDlpStatus{Available: false}
	}
	return YtDlpStatus{
		Available: true,
		Version:   strings.TrimSpace(string(out)),
		Path:      a.ytdlpPath,
	}
}

// UpdateYtDlp runs yt-dlp -U to update to the latest version
func (a *App) UpdateYtDlp() (string, error) {
	if a.ytdlpPath == "" {
		return "", fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := a.ytdlpCmd(ctx, "-U")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(toUTF8(out))

	if err != nil {
		return output, fmt.Errorf("update failed: %w", err)
	}
	return output, nil
}

// GetVideoInfo fetches video metadata via yt-dlp --dump-json
func (a *App) GetVideoInfo(url string) (VideoInfo, error) {
	if a.ytdlpPath == "" {
		return VideoInfo{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{
		"--ignore-config",
		"--dump-json",
		"--no-playlist",
		"--no-warnings",
	}
	// Apply proxy and cookies from settings
	settings := a.GetSettings()
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, url)

	cmd := a.ytdlpCmd(ctx, args...)
	a.emitLog("[GetVideoInfo] fetching info for URL: %s", url)
	a.logCmd("GetVideoInfo", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(toUTF8(out), settings)
		a.emitLog("[GetVideoInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return VideoInfo{}, fmt.Errorf("%s", errMsg)
		}
		return VideoInfo{}, fmt.Errorf("failed to get video info: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		a.emitLog("[GetVideoInfo] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return VideoInfo{}, fmt.Errorf("failed to parse video info: %w", err)
	}

	info := VideoInfo{URL: url}
	if v, ok := raw["title"].(string); ok {
		info.Title = v
	}
	if v, ok := raw["id"].(string); ok {
		info.ID = v
	}
	if v, ok := raw["thumbnail"].(string); ok {
		info.Thumbnail = v
	}
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
	// Use webpage_url if available for more accurate URL tracking on generic sites
	if v, ok := raw["webpage_url"].(string); ok && v != "" {
		info.URL = v
	}
	// Extract available subtitle languages
	info.Subtitles = extractSubtitleLangs(raw)
	return info, nil
}

// extractSubtitleLangs extracts available subtitle languages from yt-dlp JSON output.
func extractSubtitleLangs(raw map[string]interface{}) []SubtitleLang {
	var result []SubtitleLang
	seen := make(map[string]bool)

	// Manual subtitles first
	if subs, ok := raw["subtitles"].(map[string]interface{}); ok {
		for code := range subs {
			if seen[code] {
				continue
			}
			seen[code] = true
			name := code
			if arr, ok := subs[code].([]interface{}); ok && len(arr) > 0 {
				if obj, ok := arr[0].(map[string]interface{}); ok {
					if n, ok := obj["name"].(string); ok && n != "" {
						name = n
					}
				}
			}
			result = append(result, SubtitleLang{Code: code, Name: name, Auto: false})
		}
	}

	// Auto-generated captions
	if autoCaptions, ok := raw["automatic_captions"].(map[string]interface{}); ok {
		for code := range autoCaptions {
			if seen[code] {
				continue
			}
			seen[code] = true
			name := code
			if arr, ok := autoCaptions[code].([]interface{}); ok && len(arr) > 0 {
				if obj, ok := arr[0].(map[string]interface{}); ok {
					if n, ok := obj["name"].(string); ok && n != "" {
						name = n
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
	// YouTube channel patterns
	if strings.Contains(lower, "/@") || strings.Contains(lower, "/channel/") || strings.Contains(lower, "/user/") || strings.Contains(lower, "/c/") {
		return "channel"
	}
	// Bilibili series/favorites
	if strings.Contains(lower, "bilibili.com") && (strings.Contains(lower, "/favlist") || strings.Contains(lower, "/medialist") || strings.Contains(lower, "/channel/seriesdetail")) {
		return "channel"
	}
	return "playlist"
}

// GetPlaylistInfo fetches all video entries from a playlist URL via yt-dlp --flat-playlist
func (a *App) GetPlaylistInfo(url string) (PlaylistInfo, error) {
	if a.ytdlpPath == "" {
		return PlaylistInfo{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	args := []string{
		"--ignore-config",
		"--flat-playlist",
		"--dump-single-json",
		"--no-warnings",
	}
	// Apply proxy and cookies from settings
	settings := a.GetSettings()
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, url)

	cmd := a.ytdlpCmd(ctx, args...)
	a.emitLog("[GetPlaylistInfo] fetching playlist for URL: %s", url)
	a.logCmd("GetPlaylistInfo", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(toUTF8(out), settings)
		a.emitLog("[GetPlaylistInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return PlaylistInfo{}, fmt.Errorf("%s", errMsg)
		}
		return PlaylistInfo{}, fmt.Errorf("failed to get playlist info: %w", err)
	}

	result := PlaylistInfo{URL: url, Kind: detectCollectionKind(url)}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		a.emitLog("[GetPlaylistInfo] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
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
			if v, ok := entryMap["thumbnail"].(string); ok {
				info.Thumbnail = v
			}
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

// GetFormats fetches all available formats for a video URL via yt-dlp --dump-json
func (a *App) GetFormats(url string) (FormatInfo, error) {
	if a.ytdlpPath == "" {
		return FormatInfo{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{
		"--ignore-config",
		"--dump-json",
		"--no-download",
		"--no-warnings",
		"--no-playlist",
	}
	// Apply proxy and cookies from settings
	settings := a.GetSettings()
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	args = appendCookiesArgs(args, settings)
	args = append(args, url)

	cmd := a.ytdlpCmd(ctx, args...)
	a.emitLog("[GetFormats] fetching formats for URL: %s", url)
	a.logCmd("GetFormats", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := normalizeYtDlpError(toUTF8(out), settings)
		a.emitLog("[GetFormats] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return FormatInfo{}, fmt.Errorf("%s", errMsg)
		}
		return FormatInfo{}, fmt.Errorf("failed to get formats: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		a.emitLog("[GetFormats] JSON parse failed: %v, raw output: %s", err, toUTF8(out))
		return FormatInfo{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	result := FormatInfo{URL: url}
	if v, ok := raw["title"].(string); ok {
		result.Title = v
	}

	// Parse formats array
	if formatsRaw, ok := raw["formats"].([]interface{}); ok {
		for _, fRaw := range formatsRaw {
			fMap, ok := fRaw.(map[string]interface{})
			if !ok {
				continue
			}
			f := Format{}
			if v, ok := fMap["format_id"].(string); ok {
				f.FormatID = v
			}
			if v, ok := fMap["ext"].(string); ok {
				f.Ext = v
			}
			if v, ok := fMap["resolution"].(string); ok {
				f.Resolution = v
			}
			if v, ok := fMap["fps"].(float64); ok {
				f.FPS = v
			}
			if v, ok := fMap["vcodec"].(string); ok {
				f.VCodec = v
				f.HasVideo = v != "none" && v != ""
			}
			if v, ok := fMap["acodec"].(string); ok {
				f.ACodec = v
				f.HasAudio = v != "none" && v != ""
			}
			if v, ok := fMap["filesize"].(float64); ok {
				f.Filesize = int64(v)
			} else if v, ok := fMap["filesize_approx"].(float64); ok {
				f.Filesize = int64(v)
			}
			if v, ok := fMap["tbr"].(float64); ok {
				f.TBR = v
			}
			if v, ok := fMap["format_note"].(string); ok {
				f.Note = v
			}
			result.Formats = append(result.Formats, f)
		}
	}

	return result, nil
}

// SelectFolder opens a folder picker dialog
func (a *App) SelectFolder() string {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Download Directory",
	})
	if err != nil {
		return ""
	}
	return dir
}

// SelectCookiesFile opens a file picker dialog for selecting a cookies file
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

// GetDefaultDownloadDir returns the user Downloads directory
func (a *App) GetDefaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, "Downloads")
	if _, err := os.Stat(dir); err != nil {
		return home
	}
	return dir
}

// qualityArgs returns yt-dlp format arguments for the selected quality
// If quality starts with "f:", treat it as a custom format ID
func qualityArgs(quality string) []string {
	// Custom format ID support (e.g., "f:137+140")
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
		return []string{"-f", "bestaudio/best", "-x", "--audio-format", "mp3"}
	default:
		return []string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best"}
	}
}

// StartDownload enqueues and starts a video download, returns the task ID
func (a *App) StartDownload(req DownloadRequest) (string, error) {
	if a.ytdlpPath == "" {
		return "", fmt.Errorf("yt-dlp not found")
	}
	taskID := uuid.New().String()
	task := &DownloadTask{
		ID:        taskID,
		URL:       req.URL,
		OutputDir: req.OutputDir,
		Quality:   req.Quality,
		Status:    "pending",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if req.VideoInfo != nil {
		task.Title = req.VideoInfo.Title
		task.Thumbnail = req.VideoInfo.Thumbnail
	}

	a.mu.Lock()
	a.downloads[taskID] = task
	a.mu.Unlock()

	go a.upsertRecord(task)
	wailsRuntime.EventsEmit(a.ctx, "download:update", task)
	go a.runDownload(taskID, req)
	return taskID, nil
}

// lineWriter calls handler for each complete line written to it
type lineWriter struct {
	mu      sync.Mutex
	buf     []byte
	handler func(string)
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.buf = append(lw.buf, p...)
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(toUTF8(lw.buf[:idx]), "\r")
		lw.buf = lw.buf[idx+1:]
		if line != "" {
			lw.handler(line)
		}
	}
	return len(p), nil
}

var (
	progressRe  = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+at\s+(\S+)(?:\s+ETA\s+(\S+))?`)
	destRe1     = regexp.MustCompile(`^\[download\] Destination: (.+)$`)
	destRe2     = regexp.MustCompile(`Merging formats into "(.+)"`)
	destRe3     = regexp.MustCompile(`^\[ExtractAudio\] Destination: (.+)$`)
	finalPathRe = regexp.MustCompile(`^\[YT-GO-OUTPUT\](.+)$`)
)

func (a *App) runDownload(taskID string, req DownloadRequest) {
	// Acquire semaphore slot (blocks if at max concurrent downloads)
	a.downloadSem <- struct{}{}
	defer func() { <-a.downloadSem }() // Release slot when done

	ctx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	a.cancelFns[taskID] = cancel
	a.downloads[taskID].Status = "downloading"
	{
		cp := *a.downloads[taskID]
		a.mu.Unlock()
		wailsRuntime.EventsEmit(a.ctx, "download:update", &cp)
	}

	args := qualityArgs(req.Quality)
	args = append(args, "--ignore-config")

	// Determine output filename template
	settings := a.GetSettings()
	outputTemplate := "%(title)s.%(ext)s"
	if settings.FilenameTemplate != "" {
		outputTemplate = settings.FilenameTemplate
	}

	args = append(args,
		"--newline",
		"--progress",
		"--print", "after_move:[YT-GO-OUTPUT]%(filepath)s",
		"-o", filepath.Join(req.OutputDir, outputTemplate),
		"--no-playlist",
	)

	// Apply settings: rate limit, proxy, cookies, and output options
	if settings.RateLimit != "" {
		args = append(args, "--rate-limit", settings.RateLimit)
	}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
	}
	if settings.MergeOutputFormat != "" {
		args = append(args, "--merge-output-format", settings.MergeOutputFormat)
	}
	if settings.AudioFormat != "" && req.Quality == "audio" {
		// Override audio format only when downloading audio
		args = append(args, "--audio-format", settings.AudioFormat)
	}

	// Resolve per-download overrides (req.Options) vs global settings
	optSaveDescription := settings.SaveDescription
	optSaveThumbnail := settings.SaveThumbnail
	optWriteSubtitles := settings.WriteSubtitles
	optSubtitleLangs := settings.SubtitleLangs
	optEmbedSubtitles := settings.EmbedSubtitles
	optEmbedChapters := settings.EmbedChapters
	optSponsorBlock := settings.SponsorBlock
	if req.Options != nil {
		if req.Options.SaveDescription != nil {
			optSaveDescription = *req.Options.SaveDescription
		}
		if req.Options.SaveThumbnail != nil {
			optSaveThumbnail = *req.Options.SaveThumbnail
		}
		if req.Options.WriteSubtitles != nil {
			optWriteSubtitles = *req.Options.WriteSubtitles
		}
		if req.Options.SubtitleLangs != "" {
			optSubtitleLangs = req.Options.SubtitleLangs
		}
		if req.Options.EmbedSubtitles != nil {
			optEmbedSubtitles = *req.Options.EmbedSubtitles
		}
		if req.Options.EmbedChapters != nil {
			optEmbedChapters = *req.Options.EmbedChapters
		}
		if req.Options.SponsorBlock != nil {
			optSponsorBlock = *req.Options.SponsorBlock
		}
	}

	if optSaveDescription {
		args = append(args, "--write-description")
	}
	if optSaveThumbnail {
		args = append(args, "--write-thumbnail")
	}
	if optWriteSubtitles {
		args = append(args, "--write-subs")
		if optSubtitleLangs != "" {
			args = append(args, "--sub-langs", optSubtitleLangs)
		}
		if optEmbedSubtitles {
			args = append(args, "--embed-subs")
		}
	}
	if optEmbedChapters {
		args = append(args, "--embed-chapters")
	}
	if optSponsorBlock {
		args = append(args, "--sponsorblock-mark", "all")
	}
	args = appendCookiesArgs(args, settings)

	args = append(args, req.URL)

	cmd := a.ytdlpCmd(ctx, args...)
	wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{
		"taskId": taskID,
		"line":   fmt.Sprintf("[YT-GO] Starting download: %s", req.URL),
	})
	wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{
		"taskId": taskID,
		"line":   fmt.Sprintf("[YT-GO] yt-dlp path: %s", a.ytdlpPath),
	})
	wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{
		"taskId": taskID,
		"line":   fmt.Sprintf("[YT-GO] Output dir: %s", req.OutputDir),
	})

	var lastOutputFile string
	writer := &lineWriter{
		handler: func(line string) {
			if m := finalPathRe.FindStringSubmatch(line); m != nil {
				lastOutputFile = strings.TrimSpace(m[1])
				return
			}

			// Emit log line to frontend
			wailsRuntime.EventsEmit(a.ctx, "download:log", map[string]string{
				"taskId": taskID,
				"line":   line,
			})

			if m := progressRe.FindStringSubmatch(line); m != nil {
				pct, _ := strconv.ParseFloat(m[1], 64)
				var cp *DownloadTask
				a.mu.Lock()
				if t, ok := a.downloads[taskID]; ok {
					t.Progress = pct
					t.Size = m[2]
					t.Speed = m[3]
					if len(m) > 4 {
						t.ETA = m[4]
					}
					taskCopy := *t
					cp = &taskCopy
				}
				a.mu.Unlock()
				if cp != nil {
					wailsRuntime.EventsEmit(a.ctx, "download:update", cp)
				}
			} else if m := destRe1.FindStringSubmatch(line); m != nil {
				lastOutputFile = m[1]
			} else if m := destRe2.FindStringSubmatch(line); m != nil {
				lastOutputFile = strings.Trim(m[1], `"`)
			} else if m := destRe3.FindStringSubmatch(line); m != nil {
				lastOutputFile = m[1]
			}
		},
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Run()
	wasCancelled := ctx.Err() != nil
	cancel()

	a.mu.Lock()
	delete(a.cancelFns, taskID)
	var finalTask *DownloadTask
	var removed bool
	if t, ok := a.downloads[taskID]; ok {
		switch {
		case wasCancelled:
			delete(a.downloads, taskID)
			removed = true
		case err != nil:
			t.Status = "error"
			t.Error = err.Error()
		default:
			t.Status = "completed"
			t.Progress = 100
			if lastOutputFile != "" {
				t.OutputPath = lastOutputFile
			}
		}
		cp := *t
		finalTask = &cp
	}
	a.mu.Unlock()

	if removed {
		wailsRuntime.EventsEmit(a.ctx, "download:remove", taskID)
		go a.deleteRecords([]string{taskID})
		return
	}

	if finalTask != nil {
		wailsRuntime.EventsEmit(a.ctx, "download:update", finalTask)
		go a.upsertRecord(finalTask)
	}
}

// CancelDownload cancels an active download by task ID
func (a *App) CancelDownload(taskID string) error {
	a.mu.RLock()
	cancel, ok := a.cancelFns[taskID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task not found or not active")
	}
	cancel()
	return nil
}

// GetDownloads returns all download tasks
func (a *App) GetDownloads() []*DownloadTask {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*DownloadTask, 0, len(a.downloads))
	for _, t := range a.downloads {
		cp := *t
		result = append(result, &cp)
	}
	return result
}

// ClearCompleted removes finished/failed/cancelled tasks from the list
func (a *App) ClearCompleted() {
	a.mu.Lock()
	var ids []string
	for id, t := range a.downloads {
		if t.Status == "completed" || t.Status == "error" || t.Status == "cancelled" {
			ids = append(ids, id)
			delete(a.downloads, id)
		}
	}
	a.mu.Unlock()
	go a.deleteRecords(ids)
}

// OpenFolder opens a directory in the system file manager
func (a *App) OpenFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// OpenFile opens a file with the system default application
func (a *App) OpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
