package core

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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

type SettingsRecord struct {
	ID                uint `gorm:"primaryKey"`
	OutputDir         string
	Quality           string
	Language          string
	Theme             string
	Proxy             string
	RateLimit         string
	MaxConcurrent     int
	Notifications     bool
	SaveDescription   bool
	SaveThumbnail     bool
	WriteSubtitles    bool
	SubtitleLangs     string
	EmbedSubtitles    bool
	EmbedChapters     bool
	SponsorBlock      bool
	FilenameTemplate  string
	MergeOutputFormat string
	AudioFormat       string
	CookiesFrom       string
	CookiesFile       string
}

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
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&DownloadRecord{}, &SettingsRecord{}); err != nil {
		return nil, err
	}
	return db, nil
}

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
