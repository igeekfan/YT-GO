package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const youtubeLedgerBatchSize = 500

type youtubeDownload struct {
	VideoID string `gorm:"primaryKey"`
}

func migrateYouTubeDownloads(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&youtubeDownload{}) {
			return nil
		}
		if err := tx.AutoMigrate(&youtubeDownload{}); err != nil {
			return err
		}
		var records []DownloadRecord
		return tx.Select("id", "url").Where("status = ?", "completed").
			FindInBatches(&records, youtubeLedgerBatchSize, func(batch *gorm.DB, _ int) error {
				downloads := make([]youtubeDownload, 0, len(records))
				for _, record := range records {
					if id := youtubeVideoID(record.URL); id != "" {
						downloads = append(downloads, youtubeDownload{VideoID: id})
					}
				}
				if len(downloads) == 0 {
					return nil
				}
				return batch.Clauses(clause.OnConflict{DoNothing: true}).Create(&downloads).Error
			}).Error
	})
}

// IncrementalDownloadRequest carries the videos reviewed in a scan and the download settings.
type IncrementalDownloadRequest struct {
	Videos    []YouTubePlaylistVideo `json:"videos"`
	OutputDir string                 `json:"outputDir"`
	Quality   string                 `json:"quality"`
	Options   *DownloadOptions       `json:"options"`
}

// IncrementalDownloadResult reports the queue-time outcome for the reviewed videos.
type IncrementalDownloadResult struct {
	Queued    int `json:"queued"`
	Skipped   int `json:"skipped"`
	Remaining int `json:"remaining"`
}

func (s *Service) newYouTubeVideosLocked(videos []YouTubePlaylistVideo) ([]YouTubePlaylistVideo, error) {
	newVideos := make([]YouTubePlaylistVideo, 0, len(videos))
	if len(videos) == 0 {
		return newVideos, nil
	}
	db := s.database()
	if db == nil {
		return nil, errors.New(s.i18n.T("incremental.ledgerUnavailable"))
	}
	skip := make(map[string]bool)
	for batch := range slices.Chunk(videos, youtubeLedgerBatchSize) {
		ids := make([]string, 0, len(batch))
		for _, video := range batch {
			ids = append(ids, video.ID)
		}
		var completed []string
		if err := db.Model(&youtubeDownload{}).Where("video_id IN ?", ids).Pluck("video_id", &completed).Error; err != nil {
			return nil, fmt.Errorf("%s: %w", s.i18n.T("incremental.ledgerUnavailable"), err)
		}
		for _, id := range completed {
			skip[id] = true
		}
	}
	for taskID := range s.activeDownloads {
		if task := s.downloads[taskID]; task != nil {
			if id := youtubeVideoID(task.URL); id != "" {
				skip[id] = true
			}
		}
	}
	for _, video := range videos {
		if !skip[video.ID] {
			skip[video.ID] = true
			newVideos = append(newVideos, video)
		}
	}
	return newVideos, nil
}

// QueueNewYouTubeVideos reserves the reviewed items that are still eligible.
func (s *Service) QueueNewYouTubeVideos(req IncrementalDownloadRequest) (IncrementalDownloadResult, error) {
	if s.downloadDir != "" {
		req.OutputDir = s.downloadDir
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		return IncrementalDownloadResult{}, errors.New(s.i18n.T("incremental.outputDirRequired"))
	}
	for _, video := range req.Videos {
		if !youtubeVideoIDPattern.MatchString(video.ID) {
			return IncrementalDownloadResult{}, errors.New(s.i18n.T("incremental.invalidVideo"))
		}
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.accepting {
		return IncrementalDownloadResult{}, ErrServiceClosed
	}
	s.ensureRuntimeLocked()
	s.mu.Lock()
	newVideos, err := s.newYouTubeVideosLocked(req.Videos)
	if err != nil {
		s.mu.Unlock()
		return IncrementalDownloadResult{}, err
	}
	queued := make([]queuedDownload, 0, min(len(newVideos), maxQueuedDownloads))
	for _, video := range newVideos {
		url := youtubeOrigin + "/watch?v=" + video.ID
		download := DownloadRequest{
			URL:       url,
			OutputDir: req.OutputDir,
			Quality:   req.Quality,
			Options:   req.Options,
			VideoInfo: &VideoInfo{URL: url, ID: video.ID, Title: video.Title, Thumbnail: video.Thumbnail, Platform: "Youtube"},
		}
		entry, err := s.enqueueDownloadLocked(download, "", true)
		if err != nil {
			break
		}
		queued = append(queued, entry)
	}
	s.mu.Unlock()
	for _, entry := range queued {
		s.publishQueuedDownload(entry)
	}
	return IncrementalDownloadResult{
		Queued:    len(queued),
		Skipped:   len(req.Videos) - len(newVideos),
		Remaining: len(newVideos) - len(queued),
	}, nil
}
