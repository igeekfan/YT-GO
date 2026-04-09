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

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// ytdlpCmd creates an exec.Cmd for yt-dlp with UTF-8 output encoding on Windows.
func (a *App) ytdlpCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, a.ytdlpPath, args...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	return cmd
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

// GetSettings returns persisted user settings
func (a *App) GetSettings() Settings {
	defaults := Settings{
		OutputDir:     a.GetDefaultDownloadDir(),
		Quality:       "best",
		Language:      "zh-CN",
		Theme:         "dark",
		Proxy:         "",
		RateLimit:     "",
		MaxConcurrent: 3,
		Notifications: true,
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
	return defaults
}

// SaveSettings persists user settings to the database
func (a *App) SaveSettings(s Settings) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	rec := SettingsRecord{
		ID:            1,
		OutputDir:     s.OutputDir,
		Quality:       s.Quality,
		Language:      s.Language,
		Theme:         s.Theme,
		Proxy:         s.Proxy,
		RateLimit:     s.RateLimit,
		MaxConcurrent: s.MaxConcurrent,
		Notifications: s.Notifications,
	}
	return a.db.Save(&rec).Error
}

// DiagnosticInfo contains debug information about the application state
type DiagnosticInfo struct {
	YtDlpPath    string `json:"ytdlpPath"`
	YtDlpVersion string `json:"ytdlpVersion"`
	YtDlpFound   bool   `json:"ytdlpFound"`
	TestOutput   string `json:"testOutput"`
	Error        string `json:"error"`
}

// GetDiagnosticInfo returns debug information to help troubleshoot issues
func (a *App) GetDiagnosticInfo() DiagnosticInfo {
	info := DiagnosticInfo{
		YtDlpPath:  a.ytdlpPath,
		YtDlpFound: a.ytdlpPath != "",
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
			// Just check if output contains expected text
			if strings.Contains(string(testOut), "youtube") || strings.Contains(string(testOut), "Usage:") {
				info.TestOutput = "yt-dlp is working correctly"
			} else {
				info.TestOutput = fmt.Sprintf("unexpected output (first 200 chars): %s", string(testOut)[:min(200, len(testOut))])
			}
		}
	} else {
		info.Error = "yt-dlp not found in PATH or common installation directories"
	}

	return info
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
	output := strings.TrimSpace(string(out))

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
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		a.emitLog("[GetVideoInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return VideoInfo{}, fmt.Errorf("%s", errMsg)
		}
		return VideoInfo{}, fmt.Errorf("failed to get video info: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		a.emitLog("[GetVideoInfo] JSON parse failed: %v, raw output: %s", err, string(out))
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
	}
	return info, nil
}

// GetPlaylistInfo fetches all video entries from a playlist URL via yt-dlp --flat-playlist
func (a *App) GetPlaylistInfo(url string) (PlaylistInfo, error) {
	if a.ytdlpPath == "" {
		return PlaylistInfo{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	args := []string{
		"--flat-playlist",
		"--dump-json",
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		a.emitLog("[GetPlaylistInfo] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return PlaylistInfo{}, fmt.Errorf("%s", errMsg)
		}
		return PlaylistInfo{}, fmt.Errorf("failed to get playlist info: %w", err)
	}

	result := PlaylistInfo{URL: url}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		// First entry may contain playlist-level metadata
		if result.Title == "" {
			if v, ok := raw["playlist_title"].(string); ok {
				result.Title = v
			} else if v, ok := raw["title"].(string); ok {
				// Only use as playlist title if it looks like a playlist entry
				if _, isPlaylist := raw["entries"]; isPlaylist {
					result.Title = v
				}
			}
			if v, ok := raw["playlist_uploader"].(string); ok {
				result.Uploader = v
			}
		}
		// Build per-video info
		info := VideoInfo{}
		if v, ok := raw["webpage_url"].(string); ok {
			info.URL = v
		} else if v, ok := raw["url"].(string); ok {
			info.URL = v
		}
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
		if info.URL != "" || info.ID != "" {
			result.Videos = append(result.Videos, info)
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		a.emitLog("[GetFormats] failed: err=%v, output=%s", err, errMsg)
		if errMsg != "" {
			return FormatInfo{}, fmt.Errorf("%s", errMsg)
		}
		return FormatInfo{}, fmt.Errorf("failed to get formats: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		a.emitLog("[GetFormats] JSON parse failed: %v, raw output: %s", err, string(out))
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
		line := strings.TrimRight(string(lw.buf[:idx]), "\r")
		lw.buf = lw.buf[idx+1:]
		if line != "" {
			lw.handler(line)
		}
	}
	return len(p), nil
}

var (
	progressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+(\S+)\s+at\s+(\S+)(?:\s+ETA\s+(\S+))?`)
	destRe1    = regexp.MustCompile(`^\[download\] Destination: (.+)$`)
	destRe2    = regexp.MustCompile(`Merging formats into "(.+)"`)
	destRe3    = regexp.MustCompile(`^\[ExtractAudio\] Destination: (.+)$`)
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
	args = append(args,
		"--newline",
		"--progress",
		"-o", filepath.Join(req.OutputDir, "%(title)s.%(ext)s"),
		"--no-playlist",
	)

	// Apply settings: rate limit, proxy and cookies
	settings := a.GetSettings()
	if settings.RateLimit != "" {
		args = append(args, "--rate-limit", settings.RateLimit)
	}
	if settings.Proxy != "" {
		args = append(args, "--proxy", settings.Proxy)
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
	if t, ok := a.downloads[taskID]; ok {
		switch {
		case wasCancelled:
			t.Status = "cancelled"
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
