package core

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var wechatSphPathRe = regexp.MustCompile(`^/sph/([A-Za-z0-9_-]+)/?$`)

func parseWechatChannelsInput(rawInput string) (wechatChannelsInput, error) {
	rawURL := extractURLFromText(rawInput)
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return wechatChannelsInput{}, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return wechatChannelsInput{}, fmt.Errorf("unsupported URL scheme")
	}

	host := strings.ToLower(parsed.Hostname())
	path := parsed.EscapedPath()
	input := wechatChannelsInput{}

	switch host {
	case "weixin.qq.com":
		match := wechatSphPathRe.FindStringSubmatch(parsed.Path)
		if match == nil {
			return wechatChannelsInput{}, fmt.Errorf("not a WeChat Channels share URL")
		}
		input.ID = match[1]
		input.ShareURL = "https://weixin.qq.com/sph/" + input.ID
		return input, nil
	case "channels.weixin.qq.com":
		if match := wechatSphPathRe.FindStringSubmatch(parsed.Path); match != nil {
			input.ID = match[1]
			input.ShareURL = "https://weixin.qq.com/sph/" + input.ID
			return input, nil
		}
		if strings.EqualFold(parsed.Path, "/finder-preview/pages/sph") {
			id := strings.TrimSpace(parsed.Query().Get("id"))
			if id == "" {
				return wechatChannelsInput{}, fmt.Errorf("missing sph id")
			}
			input.ID = id
			input.ShareURL = "https://weixin.qq.com/sph/" + id
			return input, nil
		}
		if strings.EqualFold(parsed.Path, "/finder-preview/pages/feed") || strings.HasSuffix(path, "/finder-preview/pages/feed") {
			token := parsed.Query().Get("token")
			eid := parsed.Query().Get("eid")
			if token == "" || eid == "" {
				return wechatChannelsInput{}, fmt.Errorf("missing token or eid in WeChat Channels preview URL")
			}
			input.PlayableURL = parsed.String()
			input.Token = token
			input.ExportID = eid
			return input, nil
		}
	}

	return wechatChannelsInput{}, fmt.Errorf("not a WeChat Channels URL")
}

func fillWechatPlayableTokens(input *wechatChannelsInput) error {
	parsed, err := url.Parse(input.PlayableURL)
	if err != nil {
		return err
	}
	input.Token = parsed.Query().Get("token")
	input.ExportID = parsed.Query().Get("eid")
	if input.Token == "" || input.ExportID == "" {
		return fmt.Errorf("missing token or eid in playable_url")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
