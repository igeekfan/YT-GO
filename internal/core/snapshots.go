package core

func cloneDownloadTask(task *DownloadTask) *DownloadTask {
	if task == nil {
		return nil
	}
	snapshot := *task
	return &snapshot
}

func cloneDownloadRequest(req DownloadRequest) DownloadRequest {
	snapshot := req
	if req.VideoInfo != nil {
		info := *req.VideoInfo
		info.Subtitles = append([]SubtitleLang(nil), req.VideoInfo.Subtitles...)
		snapshot.VideoInfo = &info
	}
	if req.Options != nil {
		options := *req.Options
		options.SaveDescription = cloneBool(req.Options.SaveDescription)
		options.SaveThumbnail = cloneBool(req.Options.SaveThumbnail)
		options.EmbedChapters = cloneBool(req.Options.EmbedChapters)
		options.WriteSubtitles = cloneBool(req.Options.WriteSubtitles)
		options.WriteManualSubs = cloneBool(req.Options.WriteManualSubs)
		options.WriteAutoSubs = cloneBool(req.Options.WriteAutoSubs)
		options.EmbedSubtitles = cloneBool(req.Options.EmbedSubtitles)
		options.SponsorBlock = cloneBool(req.Options.SponsorBlock)
		snapshot.Options = &options
	}
	return snapshot
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
