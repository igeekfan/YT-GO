package core

import (
	"errors"
	"net/url"
	"strings"
)

const defaultDownloadFilenameTemplate = "%(title)s.%(ext)s"

type downloadMediaOptions struct {
	FilenameTemplate       string                 `json:"filenameTemplate"`
	MergeOutputFormat      string                 `json:"mergeOutputFormat"`
	AudioFormat            string                 `json:"audioFormat"`
	RateLimit              string                 `json:"rateLimit"`
	SaveDescription        bool                   `json:"saveDescription"`
	SaveThumbnail          bool                   `json:"saveThumbnail"`
	EmbedChapters          bool                   `json:"embedChapters"`
	SponsorBlock           bool                   `json:"sponsorBlock"`
	Subtitles              subtitleDownloadConfig `json:"subtitles"`
	NeedsSourceCredentials bool                   `json:"needsSourceCredentials,omitempty"`
}

func downloadFilenameTemplate(settings Settings, options *DownloadOptions) string {
	if options != nil && strings.TrimSpace(options.FilenameTemplate) != "" {
		return options.FilenameTemplate
	}
	if settings.FilenameTemplate != "" {
		return settings.FilenameTemplate
	}
	return defaultDownloadFilenameTemplate
}

func resolveDownloadMediaOptions(settings Settings, req DownloadRequest) downloadMediaOptions {
	template := downloadFilenameTemplate(settings, req.Options)
	if isWechatChannelsURL(req.URL) || (req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "") {
		template = strings.TrimSpace(template)
		if template == "" {
			template = defaultDownloadFilenameTemplate
		}
	}
	resolved := downloadMediaOptions{
		FilenameTemplate:       template,
		RateLimit:              settings.RateLimit,
		SaveDescription:        settings.SaveDescription,
		SaveThumbnail:          settings.SaveThumbnail,
		EmbedChapters:          settings.EmbedChapters,
		SponsorBlock:           settings.SponsorBlock,
		Subtitles:              resolveSubtitleDownloadConfig(settings, req.Options),
		NeedsSourceCredentials: downloadHistoryURL(req.URL) != req.URL,
	}
	if shouldApplyMergeOutputFormat(req.Quality) {
		resolved.MergeOutputFormat = settings.MergeOutputFormat
	}
	if requiresAudioExtraction(req.Quality) {
		resolved.AudioFormat = settings.AudioFormat
		if resolved.AudioFormat == "" && req.Quality == "audio" {
			resolved.AudioFormat = "mp3"
		}
	}
	if req.Options != nil {
		if req.Options.SaveDescription != nil {
			resolved.SaveDescription = *req.Options.SaveDescription
		}
		if req.Options.SaveThumbnail != nil {
			resolved.SaveThumbnail = *req.Options.SaveThumbnail
		}
		if req.Options.EmbedChapters != nil {
			resolved.EmbedChapters = *req.Options.EmbedChapters
		}
		if req.Options.SponsorBlock != nil {
			resolved.SponsorBlock = *req.Options.SponsorBlock
		}
	}
	return resolved
}

func parseDownloadURL(rawURL string) (*url.URL, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, false
	}
	return parsed, true
}

// DownloadValidator applies runtime-specific URL and destination rules to a download.
type DownloadValidator func(rawURL, outputDir, filenameTemplate string) (validatedURL, validatedDir string, err error)

// StartDownload queues a download with the effective media options resolved at submission.
func (s *Service) StartDownload(req DownloadRequest) (string, error) {
	return s.StartDownloadValidated(req, nil)
}

// StartDownloadValidated applies the same queueing rules with runtime-specific validation.
func (s *Service) StartDownloadValidated(req DownloadRequest, validate DownloadValidator) (string, error) {
	req.URL = extractURLFromText(req.URL)
	if req.URL == "" {
		return "", errors.New(s.i18n.T("download.url_required"))
	}
	if _, ok := parseDownloadURL(req.URL); !ok {
		return "", errors.New(s.i18n.T("download.url_invalid"))
	}
	if s.downloadDir != "" {
		req.OutputDir = s.downloadDir
	}
	if req.Quality == "" {
		req.Quality = "best"
	}

	settings := s.GetSettings()
	template := downloadFilenameTemplate(settings, req.Options)
	if req.mediaOptions != nil {
		template = req.mediaOptions.FilenameTemplate
	}
	if validate != nil {
		validatedURL, validatedDir, err := validate(req.URL, req.OutputDir, template)
		if err != nil {
			return "", err
		}
		req.URL = validatedURL
		req.OutputDir = validatedDir
	}
	if req.mediaOptions == nil {
		opts := resolveDownloadMediaOptions(settings, req)
		req.mediaOptions = &opts
	}
	return s.enqueueDownload(req, "")
}

// RetryDownload queues a fresh attempt using the original task's saved media options.
func (s *Service) RetryDownload(taskID string, validate DownloadValidator) (string, error) {
	task, err := s.GetDownload(taskID)
	if err != nil {
		return "", err
	}
	if task.Status == "pending" || task.Status == "downloading" {
		return "", errors.New(s.i18n.T("download.retry_active"))
	}
	rawURL := task.URL
	if task.sourceURL != "" {
		rawURL = task.sourceURL
	} else if task.mediaOptions != nil && task.mediaOptions.NeedsSourceCredentials {
		return "", errors.New(s.i18n.T("download.retry_credentials"))
	}

	req := DownloadRequest{
		URL: rawURL, OutputDir: task.OutputDir, Quality: task.Quality,
		VideoInfo:    &VideoInfo{Title: task.Title, Thumbnail: task.Thumbnail},
		mediaOptions: task.mediaOptions,
	}
	return s.StartDownloadValidated(req, validate)
}
