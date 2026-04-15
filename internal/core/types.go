package core

type Lang string

const (
	LangZhCN Lang = "zh-CN"
	LangEnUS Lang = "en-US"
)

type YtDlpStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Path      string `json:"path"`
}

type VideoInfo struct {
	URL       string         `json:"url"`
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Thumbnail string         `json:"thumbnail"`
	Duration  float64        `json:"duration"`
	Uploader  string         `json:"uploader"`
	Platform  string         `json:"platform"`
	Subtitles []SubtitleLang `json:"subtitles"`
}

type SubtitleLang struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Auto bool   `json:"auto"`
}

type DownloadRequest struct {
	URL       string           `json:"url"`
	OutputDir string           `json:"outputDir"`
	Quality   string           `json:"quality"`
	VideoInfo *VideoInfo       `json:"videoInfo"`
	Options   *DownloadOptions `json:"options"`
}

type DownloadOptions struct {
	SaveDescription *bool  `json:"saveDescription"`
	SaveThumbnail   *bool  `json:"saveThumbnail"`
	EmbedChapters   *bool  `json:"embedChapters"`
	WriteSubtitles  *bool  `json:"writeSubtitles"`
	SubtitleLangs   string `json:"subtitleLangs"`
	EmbedSubtitles  *bool  `json:"embedSubtitles"`
	SponsorBlock    *bool  `json:"sponsorBlock"`
}

type DownloadTask struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Thumbnail  string  `json:"thumbnail"`
	Quality    string  `json:"quality"`
	Status     string  `json:"status"`
	Progress   float64 `json:"progress"`
	Speed      string  `json:"speed"`
	ETA        string  `json:"eta"`
	Size       string  `json:"size"`
	OutputPath string  `json:"outputPath"`
	OutputDir  string  `json:"outputDir"`
	Error      string  `json:"error"`
	CreatedAt  string  `json:"createdAt"`
}

type PlaylistInfo struct {
	URL      string      `json:"url"`
	Kind     string      `json:"kind"`
	Title    string      `json:"title"`
	Uploader string      `json:"uploader"`
	Count    int         `json:"count"`
	Videos   []VideoInfo `json:"videos"`
}

type Settings struct {
	OutputDir         string `json:"outputDir"`
	Quality           string `json:"quality"`
	Language          string `json:"language"`
	Theme             string `json:"theme"`
	Proxy             string `json:"proxy"`
	RateLimit         string `json:"rateLimit"`
	MaxConcurrent     int    `json:"maxConcurrent"`
	Notifications     bool   `json:"notifications"`
	SaveDescription   bool   `json:"saveDescription"`
	SaveThumbnail     bool   `json:"saveThumbnail"`
	WriteSubtitles    bool   `json:"writeSubtitles"`
	SubtitleLangs     string `json:"subtitleLangs"`
	EmbedSubtitles    bool   `json:"embedSubtitles"`
	EmbedChapters     bool   `json:"embedChapters"`
	SponsorBlock      bool   `json:"sponsorBlock"`
	FilenameTemplate  string `json:"filenameTemplate"`
	MergeOutputFormat string `json:"mergeOutputFormat"`
	AudioFormat       string `json:"audioFormat"`
	CookiesFrom       string `json:"cookiesFrom"`
	CookiesFile       string `json:"cookiesFile"`
}

type Format struct {
	FormatID   string  `json:"formatId"`
	Ext        string  `json:"ext"`
	Resolution string  `json:"resolution"`
	FPS        float64 `json:"fps"`
	VCodec     string  `json:"vcodec"`
	ACodec     string  `json:"acodec"`
	Filesize   int64   `json:"filesize"`
	TBR        float64 `json:"tbr"`
	Note       string  `json:"note"`
	HasVideo   bool    `json:"hasVideo"`
	HasAudio   bool    `json:"hasAudio"`
}

type FormatInfo struct {
	URL     string   `json:"url"`
	Title   string   `json:"title"`
	Formats []Format `json:"formats"`
}

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

type AboutInfo struct {
	AppVersion    string `json:"appVersion"`
	SystemVersion string `json:"systemVersion"`
	GithubRepo    string `json:"githubRepo"`
	GithubURL     string `json:"githubUrl"`
	AuthorEmail   string `json:"authorEmail"`
}

type UpdateInfo struct {
	HasUpdate      bool   `json:"hasUpdate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseName    string `json:"releaseName"`
	ReleaseBody    string `json:"releaseBody"`
	HTMLURL        string `json:"htmlUrl"`
	PublishedAt    string `json:"publishedAt"`
}
