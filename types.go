package main

// AppVersion is the current application version
const AppVersion = "1.0.0"

// YtDlpStatus holds yt-dlp availability info
type YtDlpStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Path      string `json:"path"`
}

// VideoInfo holds metadata about a video URL
type VideoInfo struct {
	URL       string  `json:"url"`
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Thumbnail string  `json:"thumbnail"`
	Duration  float64 `json:"duration"`
	Uploader  string  `json:"uploader"`
	Platform  string  `json:"platform"`
}

// DownloadRequest specifies what to download
type DownloadRequest struct {
	URL       string     `json:"url"`
	OutputDir string     `json:"outputDir"`
	Quality   string     `json:"quality"` // best, 1080p, 720p, 480p, 360p, audio
	VideoInfo *VideoInfo `json:"videoInfo"`
}

// DownloadTask tracks a single download job
type DownloadTask struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Thumbnail  string  `json:"thumbnail"`
	Quality    string  `json:"quality"`
	Status     string  `json:"status"` // pending, downloading, completed, error, cancelled
	Progress   float64 `json:"progress"`
	Speed      string  `json:"speed"`
	ETA        string  `json:"eta"`
	Size       string  `json:"size"`
	OutputPath string  `json:"outputPath"`
	OutputDir  string  `json:"outputDir"`
	Error      string  `json:"error"`
	CreatedAt  string  `json:"createdAt"`
}

// PlaylistInfo holds metadata about a playlist URL
type PlaylistInfo struct {
	URL      string      `json:"url"`
	Kind     string      `json:"kind"`
	Title    string      `json:"title"`
	Uploader string      `json:"uploader"`
	Count    int         `json:"count"`
	Videos   []VideoInfo `json:"videos"`
}

// Settings holds user preferences
type Settings struct {
	OutputDir     string `json:"outputDir"`
	Quality       string `json:"quality"`
	Language      string `json:"language"`
	Theme         string `json:"theme"`
	Proxy         string `json:"proxy"`
	RateLimit     string `json:"rateLimit"`
	MaxConcurrent int    `json:"maxConcurrent"`
	Notifications bool   `json:"notifications"`
	SaveDescription bool `json:"saveDescription"`
	SaveThumbnail bool   `json:"saveThumbnail"`
	WriteSubtitles bool  `json:"writeSubtitles"`
	SubtitleLangs  string `json:"subtitleLangs"`
	EmbedSubtitles bool  `json:"embedSubtitles"`
	CookiesFrom   string `json:"cookiesFrom"`   // --cookies-from-browser value (e.g. chrome, firefox, edge)
	CookiesFile   string `json:"cookiesFile"`   // --cookies file path
}

// Format represents a single video/audio format option
type Format struct {
	FormatID   string  `json:"formatId"`
	Ext        string  `json:"ext"`
	Resolution string  `json:"resolution"`
	FPS        float64 `json:"fps"`
	VCodec     string  `json:"vcodec"`
	ACodec     string  `json:"acodec"`
	Filesize   int64   `json:"filesize"`
	TBR        float64 `json:"tbr"` // total bitrate
	Note       string  `json:"note"`
	HasVideo   bool    `json:"hasVideo"`
	HasAudio   bool    `json:"hasAudio"`
}

// FormatInfo holds all available formats for a video
type FormatInfo struct {
	URL     string   `json:"url"`
	Title   string   `json:"title"`
	Formats []Format `json:"formats"`
}
