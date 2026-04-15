package desktop

import "YT-GO/internal/core"

func toCoreSettings(in Settings) core.Settings {
	return core.Settings(in)
}

func fromCoreSettings(in core.Settings) Settings {
	return Settings(in)
}

func fromCoreYtDlpStatus(in core.YtDlpStatus) YtDlpStatus {
	return YtDlpStatus(in)
}

func toCoreSubtitleLangs(in []SubtitleLang) []core.SubtitleLang {
	out := make([]core.SubtitleLang, 0, len(in))
	for _, item := range in {
		out = append(out, core.SubtitleLang(item))
	}
	return out
}

func fromCoreSubtitleLangs(in []core.SubtitleLang) []SubtitleLang {
	out := make([]SubtitleLang, 0, len(in))
	for _, item := range in {
		out = append(out, SubtitleLang(item))
	}
	return out
}

func toCoreVideoInfo(in *VideoInfo) *core.VideoInfo {
	if in == nil {
		return nil
	}
	return &core.VideoInfo{URL: in.URL, ID: in.ID, Title: in.Title, Thumbnail: in.Thumbnail, Duration: in.Duration, Uploader: in.Uploader, Platform: in.Platform, Subtitles: toCoreSubtitleLangs(in.Subtitles)}
}

func fromCoreVideoInfo(in core.VideoInfo) VideoInfo {
	return VideoInfo{URL: in.URL, ID: in.ID, Title: in.Title, Thumbnail: in.Thumbnail, Duration: in.Duration, Uploader: in.Uploader, Platform: in.Platform, Subtitles: fromCoreSubtitleLangs(in.Subtitles)}
}

func fromCorePlaylistInfo(in core.PlaylistInfo) PlaylistInfo {
	videos := make([]VideoInfo, 0, len(in.Videos))
	for _, item := range in.Videos {
		videos = append(videos, fromCoreVideoInfo(item))
	}
	return PlaylistInfo{URL: in.URL, Kind: in.Kind, Title: in.Title, Uploader: in.Uploader, Count: in.Count, Videos: videos}
}

func fromCoreFormatInfo(in core.FormatInfo) FormatInfo {
	formats := make([]Format, 0, len(in.Formats))
	for _, item := range in.Formats {
		formats = append(formats, Format(item))
	}
	return FormatInfo{URL: in.URL, Title: in.Title, Formats: formats}
}

func toCoreDownloadOptions(in *DownloadOptions) *core.DownloadOptions {
	if in == nil {
		return nil
	}
	return &core.DownloadOptions{SaveDescription: in.SaveDescription, SaveThumbnail: in.SaveThumbnail, EmbedChapters: in.EmbedChapters, WriteSubtitles: in.WriteSubtitles, SubtitleLangs: in.SubtitleLangs, EmbedSubtitles: in.EmbedSubtitles, SponsorBlock: in.SponsorBlock}
}

func toCoreDownloadRequest(in DownloadRequest) core.DownloadRequest {
	return core.DownloadRequest{URL: in.URL, OutputDir: in.OutputDir, Quality: in.Quality, VideoInfo: toCoreVideoInfo(in.VideoInfo), Options: toCoreDownloadOptions(in.Options)}
}

func fromCoreDownloadTask(in core.DownloadTask) DownloadTask { return DownloadTask(in) }

func fromCoreDownloadTasks(in []*core.DownloadTask) []*DownloadTask {
	out := make([]*DownloadTask, 0, len(in))
	for _, item := range in {
		if item == nil {
			out = append(out, nil)
			continue
		}
		copy := fromCoreDownloadTask(*item)
		out = append(out, &copy)
	}
	return out
}

func fromCoreDiagnosticInfo(in core.DiagnosticInfo) DiagnosticInfo { return DiagnosticInfo(in) }

func fromCoreAboutInfo(in core.AboutInfo) AboutInfo { return AboutInfo(in) }

func fromCoreUpdateInfo(in core.UpdateInfo) UpdateInfo { return UpdateInfo(in) }

func fromCoreDepItem(in core.DepItem) DepItem { return DepItem(in) }

func fromCoreDepStatus(in core.DepStatus) DepStatus {
	return DepStatus{
		YtDlp:         fromCoreDepItem(in.YtDlp),
		FFmpeg:        fromCoreDepItem(in.FFmpeg),
		JSRuntime:     fromCoreDepItem(in.JSRuntime),
		JSRuntimeName: in.JSRuntimeName,
	}
}

func (a *App) GetSettings() Settings         { return fromCoreSettings(a.service.GetSettings()) }
func (a *App) IsFirstRun() bool              { return a.service.IsFirstRun() }
func (a *App) NeedsCookieConfig() bool       { return a.service.NeedsCookieConfig() }
func (a *App) SaveSettings(s Settings) error { return a.service.SaveSettings(toCoreSettings(s)) }
func (a *App) ResetSettings() error          { return a.service.ResetSettings() }
func (a *App) GetDefaultDownloadDir() string { return a.service.GetDefaultDownloadDir() }
func (a *App) SetLang(lang string)           { a.service.SetLang(lang) }
func (a *App) GetLang() string               { return a.service.GetLang() }
func (a *App) CheckYtDlp() YtDlpStatus       { return fromCoreYtDlpStatus(a.service.CheckYtDlp()) }
func (a *App) UpdateYtDlp() (string, error)  { return a.service.UpdateYtDlp() }
func (a *App) UpdateDeno() (string, error)   { return a.service.UpdateDeno() }
func (a *App) GetVideoInfo(url string) (VideoInfo, error) {
	info, err := a.service.GetVideoInfo(url)
	return fromCoreVideoInfo(info), err
}
func (a *App) GetPlaylistInfo(url string) (PlaylistInfo, error) {
	info, err := a.service.GetPlaylistInfo(url)
	return fromCorePlaylistInfo(info), err
}
func (a *App) GetFormats(url string) (FormatInfo, error) {
	info, err := a.service.GetFormats(url)
	return fromCoreFormatInfo(info), err
}
func (a *App) StartDownload(req DownloadRequest) (string, error) {
	return a.service.StartDownload(toCoreDownloadRequest(req))
}
func (a *App) CancelDownload(taskID string) error { return a.service.CancelDownload(taskID) }
func (a *App) RemoveDownload(taskID string) error { return a.service.RemoveDownload(taskID) }
func (a *App) GetDownloads() []*DownloadTask      { return fromCoreDownloadTasks(a.service.GetDownloads()) }
func (a *App) ClearCompleted()                    { a.service.ClearCompleted() }
func (a *App) GetDiagnosticInfo() DiagnosticInfo {
	return fromCoreDiagnosticInfo(a.service.GetDiagnosticInfo())
}
func (a *App) GetAboutInfo() AboutInfo   { return fromCoreAboutInfo(a.service.GetAboutInfo()) }
func (a *App) GetCurrentVersion() string { return a.service.GetCurrentVersion() }
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	info, err := a.service.CheckForUpdate()
	return fromCoreUpdateInfo(info), err
}
func (a *App) GetDepStatus() DepStatus { return fromCoreDepStatus(a.service.GetDepStatus()) }
