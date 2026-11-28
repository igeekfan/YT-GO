package core

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/lrstanley/go-ytdlp"
)

const youtubeOrigin = "https://www.youtube.com"

var (
	youtubeVideoIDPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	youtubePlaylistIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	youtubeChannelPath       = regexp.MustCompile(`^/(?:@[^/]+|(?:channel|c|user)/[^/]+)(?:/(?:videos|shorts|streams))?$`)
)

// YouTubePlaylistVideo is a downloadable video found in a YouTube collection.
type YouTubePlaylistVideo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
}

// IncrementalPlaylistScan contains the items to review before queueing.
type IncrementalPlaylistScan struct {
	NewVideos   []YouTubePlaylistVideo `json:"newVideos"`
	NewCount    int                    `json:"newCount"`
	Skipped     int                    `json:"skipped"`
	Unavailable int                    `json:"unavailable"`
}

type youtubeCollectionEntry struct {
	ytdlp.ExtractedInfo
	IEKey   string                    `json:"ie_key"`
	Entries []*youtubeCollectionEntry `json:"entries"`
}

func parseYouTubeURL(raw string) *url.URL {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil {
		return nil
	}
	switch parsed.Scheme {
	case "http":
		if parsed.Port() != "" && parsed.Port() != "80" {
			return nil
		}
	case "https":
		if parsed.Port() != "" && parsed.Port() != "443" {
			return nil
		}
	default:
		return nil
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com",
		"youtu.be", "www.youtu.be", "youtube-nocookie.com", "www.youtube-nocookie.com":
		return parsed
	}
	return nil
}

func youtubeQueryValue(parsed *url.URL, key string) string {
	values := parsed.Query()[key]
	if len(values) == 1 {
		return values[0]
	}
	return ""
}

func youtubeVideoID(raw string) string {
	parsed := parseYouTubeURL(raw)
	if parsed == nil {
		return ""
	}
	return youtubeIDFromURL(parsed)
}

func youtubeIDFromURL(parsed *url.URL) string {
	path := strings.TrimSuffix(parsed.Path, "/")
	var id string
	if strings.EqualFold(parsed.Hostname(), "youtu.be") || strings.EqualFold(parsed.Hostname(), "www.youtu.be") {
		id = strings.TrimPrefix(path, "/")
	} else if path == "/watch" {
		id = youtubeQueryValue(parsed, "v")
	} else {
		section, value, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
		switch section {
		case "shorts", "live", "embed", "v", "e":
			id = value
		}
	}
	if youtubeVideoIDPattern.MatchString(id) {
		return id
	}
	return ""
}

func youtubeCollectionURL(raw string) string {
	parsed := parseYouTubeURL(extractURLFromText(raw))
	if parsed == nil {
		return ""
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	shortLink := strings.HasSuffix(strings.ToLower(parsed.Hostname()), "youtu.be")
	if listID := youtubeQueryValue(parsed, "list"); youtubePlaylistIDPattern.MatchString(listID) {
		if youtubeIDFromURL(parsed) != "" || !shortLink && (path == "/playlist" || path == "/watch" || path == "/embed/videoseries") {
			return youtubeOrigin + "/playlist?list=" + listID
		}
	}
	if youtubeChannelPath.MatchString(path) && !shortLink &&
		!strings.HasSuffix(strings.ToLower(parsed.Hostname()), "youtube-nocookie.com") {
		return youtubeOrigin + strings.TrimSuffix(parsed.EscapedPath(), "/")
	}
	return ""
}

// ScanYouTubeCollection fetches the current playlist or channel and previews only new videos.
func (s *Service) ScanYouTubeCollection(rawURL string) (IncrementalPlaylistScan, error) {
	collectionURL := youtubeCollectionURL(rawURL)
	if collectionURL == "" {
		return IncrementalPlaylistScan{}, errors.New(s.i18n.T("incremental.unsupportedCollection"))
	}
	result, err := s.probePlaylist(collectionURL)
	if err != nil {
		return IncrementalPlaylistScan{}, err
	}
	var collection youtubeCollectionEntry
	if err := json.Unmarshal([]byte(result.Stdout), &collection); err != nil || collection.Type != ytdlp.ExtractedTypePlaylist {
		return IncrementalPlaylistScan{}, errors.New(s.i18n.T("incremental.invalidCollection"))
	}
	videos, unavailable := youtubeCollectionVideos(collection.Entries)
	s.mu.RLock()
	newVideos, err := s.newYouTubeVideosLocked(videos)
	s.mu.RUnlock()
	if err != nil {
		return IncrementalPlaylistScan{}, err
	}
	return IncrementalPlaylistScan{
		NewVideos:   newVideos[:min(len(newVideos), maxQueuedDownloads)],
		NewCount:    len(newVideos),
		Skipped:     len(videos) - len(newVideos),
		Unavailable: unavailable,
	}, nil
}

func youtubeCollectionVideos(entries []*youtubeCollectionEntry) ([]YouTubePlaylistVideo, int) {
	videos := make([]YouTubePlaylistVideo, 0, len(entries))
	unavailable := 0
	var visit func([]*youtubeCollectionEntry)
	visit = func(entries []*youtubeCollectionEntry) {
		for _, entry := range entries {
			if entry != nil && entry.Entries != nil {
				visit(entry.Entries)
				continue
			}
			if entry == nil || entry.isUnavailable() {
				unavailable++
				continue
			}
			id := entry.videoID()
			if id == "" {
				unavailable++
				continue
			}
			video := YouTubePlaylistVideo{ID: id, Thumbnail: extractThumbnailFromExtracted(&entry.ExtractedInfo)}
			if entry.Title != nil {
				video.Title = *entry.Title
			}
			videos = append(videos, video)
		}
	}
	visit(entries)
	return videos, unavailable
}

func (entry *youtubeCollectionEntry) isUnavailable() bool {
	if entry.Availability != nil {
		if *entry.Availability == "unavailable" {
			return true
		}
		if *entry.Availability == ytdlp.ExtractedAvailabilityPrivate && entry.Duration == nil {
			return true
		}
	}
	if entry.LiveStatus != nil && (*entry.LiveStatus == ytdlp.ExtractedLiveStatusIsUpcoming ||
		*entry.LiveStatus == ytdlp.ExtractedLiveStatusPostLive) {
		return true
	}
	if entry.Title != nil {
		title := strings.Trim(strings.ToLower(strings.TrimSpace(*entry.Title)), "[]")
		switch title {
		case "private video", "deleted video", "unavailable video", "video unavailable":
			return entry.Duration == nil
		}
	}
	return false
}

func (entry *youtubeCollectionEntry) videoID() string {
	if entry.Type == ytdlp.ExtractedTypePlaylist || entry.Type == ytdlp.ExtractedTypeMultiVideo {
		return ""
	}
	isYouTube := strings.EqualFold(entry.IEKey, "Youtube") ||
		(entry.ExtractorKey != nil && strings.EqualFold(*entry.ExtractorKey, "Youtube")) ||
		(entry.Extractor != nil && strings.EqualFold(*entry.Extractor, "youtube"))
	var id string
	hasURL := false
	for _, raw := range []*string{entry.WebpageURL, entry.URL} {
		if raw == nil || *raw == "" {
			continue
		}
		hasURL = true
		urlID := youtubeVideoID(*raw)
		if isYouTube && youtubeVideoIDPattern.MatchString(*raw) {
			urlID = *raw
		}
		if urlID == "" {
			continue
		}
		if id != "" && urlID != id || youtubeVideoIDPattern.MatchString(entry.ID) && urlID != entry.ID {
			return ""
		}
		id = urlID
	}
	if !hasURL && isYouTube && youtubeVideoIDPattern.MatchString(entry.ID) {
		return entry.ID
	}
	return id
}
