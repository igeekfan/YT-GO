package core

import "fmt"

func isDouyinURL(rawURL string) bool {
	_, _, err := extractDouyinTarget(rawURL)
	return err == nil
}

func (s *Service) GetDouyinVideoInfo(rawURL string) (VideoInfo, error) {
	itemInfo, videoID, finalURL, err := s.resolveDouyinItemInfo(rawURL)
	if err != nil {
		return VideoInfo{}, s.translateDouyinError(err)
	}
	video := VideoInfo{
		ID:       videoID,
		URL:      finalURL,
		Title:    douyinString(itemInfo, "desc"),
		Duration: douyinVideoDurationSeconds(itemInfo),
		Platform: s.i18n.T("douyin.platform"),
	}
	if video.Title == "" {
		video.Title = fmt.Sprintf(s.i18n.T("douyin.video_title"), videoID)
	}
	if author, ok := itemInfo["author"].(map[string]any); ok {
		video.Uploader = douyinString(author, "nickname")
	}
	if thumb := douyinCoverURL(itemInfo); thumb != "" {
		video.Thumbnail = thumb
	}
	return video, nil
}

func (s *Service) GetDouyinFormats(rawURL string) (FormatInfo, error) {
	itemInfo, _, finalURL, err := s.resolveDouyinItemInfo(rawURL)
	if err != nil {
		return FormatInfo{}, s.translateDouyinError(err)
	}
	if _, err := douyinVideoURL(itemInfo); err != nil {
		return FormatInfo{}, s.translateDouyinError(err)
	}
	width, height := douyinVideoDimensions(itemInfo)
	resolution := s.i18n.T("douyin.resolution_original")
	if width > 0 && height > 0 {
		resolution = fmt.Sprintf("%dx%d", width, height)
	}
	note := s.i18n.T("douyin.no_watermark")
	if height > 0 {
		note = fmt.Sprintf(s.i18n.T("douyin.no_watermark_height"), height)
	}
	title := douyinString(itemInfo, "desc")
	if title == "" {
		title = s.i18n.T("douyin.video_label")
	}
	return FormatInfo{
		URL:   finalURL,
		Title: title,
		Formats: []Format{{
			FormatID:   "douyin_nowm",
			Ext:        "mp4",
			Resolution: resolution,
			VCodec:     "h264",
			ACodec:     "aac",
			HasVideo:   true,
			HasAudio:   true,
			Note:       note,
			TBR:        1,
		}},
	}, nil
}

func (s *Service) resolveDouyinItemInfo(rawInput string) (map[string]any, string, string, error) {
	shareURL, directVideoID, err := extractDouyinTarget(rawInput)
	if err != nil {
		return nil, "", "", err
	}
	settings := s.GetSettings()
	resolvedURL := shareURL
	videoID := directVideoID
	if videoID == "" {
		resolvedURL, err = s.resolveDouyinRedirect(shareURL, settings)
		if err != nil {
			return nil, "", "", err
		}
		videoID, err = extractDouyinVideoID(resolvedURL)
		if err != nil {
			return nil, "", "", err
		}
	}
	itemInfo, err := s.fetchDouyinItemInfo(videoID, resolvedURL, settings)
	if err != nil {
		return nil, "", "", err
	}
	return itemInfo, videoID, resolvedURL, nil
}
