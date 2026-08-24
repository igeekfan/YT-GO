package core

import "time"

const (
	wechatChannelsPlatform      = "WeChat Channels"
	wechatChannelsDefaultFormat = "wechat:best"
	wechatChannelsPreviewFormat = "wechat:preview"
	wechatChannelsOriginFormat  = "wechat:origin"
	wechatChannelsCacheTTL      = 5 * time.Minute
)

type wechatChannelsInput struct {
	ShareURL    string
	PlayableURL string
	ID          string
	Token       string
	ExportID    string
}

type wechatChannelsResolved struct {
	Info      VideoInfo
	Formats   FormatInfo
	VideoURLs map[string]string
}

type wechatChannelsCacheEntry struct {
	Resolved  *wechatChannelsResolved
	ExpiresAt time.Time
}

type wechatParseResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data wechatParseData `json:"data"`
}

type wechatParseData struct {
	CoverURL string `json:"cover_url"`
	Author   string `json:"author"`
	Desc     string `json:"desc"`
	Playable string `json:"playable_url"`
}

type wechatFeedResponse struct {
	Data    wechatFeedData `json:"data"`
	ErrCode int            `json:"errCode"`
	ErrMsg  string         `json:"errMsg"`
}

type wechatFeedData struct {
	AuthorInfo wechatAuthorInfo `json:"authorInfo"`
	FeedInfo   wechatFeedInfo   `json:"feedInfo"`
}

type wechatAuthorInfo struct {
	Nickname string `json:"nickname"`
}

type wechatFeedInfo struct {
	VideoURL       string `json:"videoUrl"`
	OriginVideoURL string `json:"originVideoUrl"`
	Description    string `json:"description"`
	CoverURL       string `json:"coverUrl"`
}
