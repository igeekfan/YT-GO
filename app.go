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
)

// App struct
type App struct {
	ctx         context.Context
	i18n        *I18n
	downloads   map[string]*DownloadTask
	cancelFns   map[string]context.CancelFunc
	mu          sync.RWMutex
	ytdlpPath   string
	historyPath string
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		downloads: make(map[string]*DownloadTask),
		cancelFns: make(map[string]context.CancelFunc),
		i18n:      NewI18n(),
	}
	app.ytdlpPath = app.findYtDlp()
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadHistory()
}

// historyFilePath returns the path to the history JSON file
func (a *App) historyFilePath() string {
	if a.historyPath != "" {
		return a.historyPath
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	appDir := filepath.Join(dir, "ytgo")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return ""
	}
	a.historyPath = filepath.Join(appDir, "history.json")
	return a.historyPath
}

// loadHistory reads persisted tasks from disk on startup
func (a *App) loadHistory() {
	path := a.historyFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var tasks []*DownloadTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return
	}
	a.mu.Lock()
	for _, t := range tasks {
		a.downloads[t.ID] = t
	}
	a.mu.Unlock()
}

// saveHistory writes completed/failed/cancelled tasks to disk
func (a *App) saveHistory() {
	path := a.historyFilePath()
	if path == "" {
		return
	}
	a.mu.RLock()
	var tasks []*DownloadTask
	for _, t := range a.downloads {
		if t.Status == "completed" || t.Status == "error" || t.Status == "cancelled" {
			cp := *t
			tasks = append(tasks, &cp)
		}
	}
	a.mu.RUnlock()
	data, err := json.Marshal(tasks)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
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
	execDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		for _, name := range candidates {
			p := filepath.Join(execDir, name)
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

// GetVideoInfo fetches video metadata via yt-dlp --dump-json
func (a *App) GetVideoInfo(url string) (VideoInfo, error) {
	if a.ytdlpPath == "" {
		return VideoInfo{}, fmt.Errorf("yt-dlp not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.ytdlpPath,
		"--dump-json",
		"--no-playlist",
		"--no-warnings",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return VideoInfo{}, fmt.Errorf("failed to get video info: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
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

	cmd := exec.CommandContext(ctx, a.ytdlpPath,
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
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
func qualityArgs(quality string) []string {
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
		req.URL,
	)

	cmd := exec.CommandContext(ctx, a.ytdlpPath, args...)

	var lastOutputFile string
	writer := &lineWriter{
		handler: func(line string) {
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
		go a.saveHistory()
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
	defer a.mu.Unlock()
	for id, t := range a.downloads {
		if t.Status == "completed" || t.Status == "error" || t.Status == "cancelled" {
			delete(a.downloads, id)
		}
	}
	go func() {
		path := a.historyFilePath()
		if path != "" {
			_ = os.WriteFile(path, []byte("[]"), 0600)
		}
	}()
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
