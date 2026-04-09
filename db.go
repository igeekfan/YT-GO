package main

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DownloadRecord is the GORM/SQLite model for persisting download history.
// Fields mirror DownloadTask; transient run-time fields (Speed, ETA) are also
// stored so the last-seen value is shown after a restart.
type DownloadRecord struct {
	ID         string `gorm:"primaryKey"`
	URL        string `gorm:"not null"`
	Title      string
	Thumbnail  string
	Quality    string
	Status     string  `gorm:"not null;default:'pending'"`
	Progress   float64 `gorm:"default:0"`
	Speed      string
	ETA        string
	Size       string
	OutputPath string
	OutputDir  string `gorm:"not null"`
	Error      string
	CreatedAt  string `gorm:"not null"`
}

// SettingsRecord stores user preferences in SQLite.
// Only one row with ID=1 is expected.
type SettingsRecord struct {
	ID            uint   `gorm:"primaryKey"`
	OutputDir     string // default download directory
	Quality       string // default quality: best, 1080p, 720p, etc.
	Language      string // zh-CN, en-US
	Theme         string // dark, light
	Proxy         string // HTTP/SOCKS5 proxy URL
	RateLimit     string // e.g. "1M", "500K"
	MaxConcurrent int    // max concurrent downloads (0 = unlimited)
	Notifications bool   // desktop notifications on completion
	SaveDescription bool  // --write-description
	SaveThumbnail bool    // --write-thumbnail
	WriteSubtitles bool   // --write-subs
	SubtitleLangs  string // --sub-langs value (e.g. en,zh-Hans)
	EmbedSubtitles bool   // --embed-subs
	EmbedChapters  bool   // --embed-chapters
	SponsorBlock   bool   // --sponsorblock-mark all
	CookiesFrom   string // --cookies-from-browser value (chrome, firefox, edge, etc.)
	CookiesFile   string // --cookies file path
}

// openDB opens (or creates) the SQLite database at %APPDATA%/YT-GO/history.db
// and auto-migrates the schema.
func openDB() (*gorm.DB, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(dir, "YT-GO")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(appDir, "history.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&DownloadRecord{}, &SettingsRecord{}); err != nil {
		return nil, err
	}
	return db, nil
}

// taskToRecord converts a DownloadTask pointer to a DownloadRecord.
func taskToRecord(t *DownloadTask) DownloadRecord {
	return DownloadRecord{
		ID:         t.ID,
		URL:        t.URL,
		Title:      t.Title,
		Thumbnail:  t.Thumbnail,
		Quality:    t.Quality,
		Status:     t.Status,
		Progress:   t.Progress,
		Speed:      t.Speed,
		ETA:        t.ETA,
		Size:       t.Size,
		OutputPath: t.OutputPath,
		OutputDir:  t.OutputDir,
		Error:      t.Error,
		CreatedAt:  t.CreatedAt,
	}
}

// recordToTask converts a DownloadRecord to a DownloadTask pointer.
func recordToTask(r DownloadRecord) *DownloadTask {
	return &DownloadTask{
		ID:         r.ID,
		URL:        r.URL,
		Title:      r.Title,
		Thumbnail:  r.Thumbnail,
		Quality:    r.Quality,
		Status:     r.Status,
		Progress:   r.Progress,
		Speed:      r.Speed,
		ETA:        r.ETA,
		Size:       r.Size,
		OutputPath: r.OutputPath,
		OutputDir:  r.OutputDir,
		Error:      r.Error,
		CreatedAt:  r.CreatedAt,
	}
}
