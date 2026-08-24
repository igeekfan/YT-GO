package core

import (
	"net/url"
	"strings"
)

func buildWechatChannelsResolved(input wechatChannelsInput, parseData wechatParseData, feedResp wechatFeedResponse) *wechatChannelsResolved {
	feed := feedResp.Data.FeedInfo
	author := feedResp.Data.AuthorInfo
	title := strings.TrimSpace(parseData.Desc)
	if title == "" {
		title = strings.TrimSpace(feed.Description)
	}
	uploader := strings.TrimSpace(parseData.Author)
	if uploader == "" {
		uploader = strings.TrimSpace(author.Nickname)
	}
	thumbnail := strings.TrimSpace(parseData.CoverURL)
	if thumbnail == "" {
		thumbnail = strings.TrimSpace(feed.CoverURL)
	}

	infoURL := firstNonEmpty(input.ShareURL, input.PlayableURL)
	originVideoURL := getWechatOriginalVideoURL(feed)
	previewVideoURL := strings.TrimSpace(feed.VideoURL)
	videoURLs := map[string]string{}
	formats := FormatInfo{URL: infoURL, Title: title}
	appendFormat := func(formatID, videoURL, note string) {
		if videoURL == "" {
			return
		}
		videoURLs[formatID] = videoURL
		formats.Formats = append(formats.Formats, Format{
			FormatID:   formatID,
			Ext:        "mp4",
			Resolution: "unknown",
			VCodec:     "unknown",
			ACodec:     "aac",
			Note:       note,
			HasVideo:   true,
			HasAudio:   true,
		})
	}
	appendFormat(wechatChannelsOriginFormat, originVideoURL, "Origin")
	appendFormat(wechatChannelsPreviewFormat, previewVideoURL, "Preview")
	videoURLs[wechatChannelsDefaultFormat] = firstNonEmpty(originVideoURL, previewVideoURL)

	info := VideoInfo{
		URL:       infoURL,
		ID:        firstNonEmpty(input.ID, input.ExportID),
		Title:     title,
		Thumbnail: thumbnail,
		Uploader:  uploader,
		Platform:  wechatChannelsPlatform,
	}

	return &wechatChannelsResolved{
		Info:      info,
		Formats:   formats,
		VideoURLs: videoURLs,
	}
}

func getWechatOriginalVideoURL(feed wechatFeedInfo) string {
	for _, rawURL := range []string{feed.VideoURL, feed.OriginVideoURL} {
		if cleaned := cleanWechatOriginalVideoURL(rawURL); cleaned != "" {
			return cleaned
		}
	}
	return strings.TrimSpace(feed.OriginVideoURL)
}

func cleanWechatOriginalVideoURL(rawURL string) string {
	decodedURL, err := url.PathUnescape(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(decodedURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	encFileKey := parsed.Query().Get("encfilekey")
	token := parsed.Query().Get("token")
	if encFileKey == "" || token == "" {
		return ""
	}

	cleaned := &url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   parsed.Path,
	}
	query := url.Values{}
	query.Set("encfilekey", encFileKey)
	query.Set("token", token)
	cleaned.RawQuery = query.Encode()
	return cleaned.String()
}
