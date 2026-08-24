package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) GetWechatChannelsVideoInfo(rawInput string) (VideoInfo, error) {
	resolved, err := s.resolveWechatChannels(context.Background(), rawInput)
	if err != nil {
		return VideoInfo{}, err
	}
	return resolved.Info, nil
}

func (s *Service) GetWechatChannelsFormats(rawInput string) (FormatInfo, error) {
	resolved, err := s.resolveWechatChannels(context.Background(), rawInput)
	if err != nil {
		return FormatInfo{}, err
	}
	return resolved.Formats, nil
}

func (s *Service) resolveWechatChannels(parent context.Context, rawInput string) (*wechatChannelsResolved, error) {
	input, err := parseWechatChannelsInput(rawInput)
	if err != nil {
		return nil, err
	}
	cacheKey := firstNonEmpty(input.ShareURL, input.PlayableURL)
	if cached := s.getCachedWechatChannelsResolved(cacheKey); cached != nil {
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()

	settings := s.GetSettings()
	parseData := wechatParseData{}
	if input.PlayableURL == "" {
		cookieHeader, err := yuanbaoCookieHeader(settings)
		if err != nil {
			return nil, err
		}
		parseData, err = s.parseWechatChannelsShareURL(ctx, input.ShareURL, cookieHeader, settings)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(parseData.Playable) == "" {
			return nil, fmt.Errorf("yuanbao parse result did not include playable_url")
		}
		input.PlayableURL = parseData.Playable
		if err := fillWechatPlayableTokens(&input); err != nil {
			return nil, err
		}
		s.emitLog("[WechatChannels] Yuanbao parsed share URL: id=%s eid=%s", input.ID, input.ExportID)
	}

	feedResp, err := s.fetchWechatChannelsFeedInfo(ctx, input, settings)
	if err != nil {
		return nil, err
	}
	if feedResp.ErrCode != 0 {
		if feedResp.ErrMsg != "" {
			return nil, fmt.Errorf("WeChat Channels feed API error %d: %s", feedResp.ErrCode, feedResp.ErrMsg)
		}
		return nil, fmt.Errorf("WeChat Channels feed API error %d", feedResp.ErrCode)
	}

	resolved := buildWechatChannelsResolved(input, parseData, feedResp)
	if resolved.Info.Title == "" {
		resolved.Info.Title = "WeChat Channels Video"
	}
	if resolved.VideoURLs[wechatChannelsDefaultFormat] == "" {
		return nil, fmt.Errorf("WeChat Channels feed did not include a downloadable video URL; Yuanbao parsing succeeded, but WeChat preview API did not return videoUrl")
	}
	s.cacheWechatChannelsResolved(cacheKey, resolved)
	return resolved, nil
}

func (s *Service) getCachedWechatChannelsResolved(key string) *wechatChannelsResolved {
	s.wechatCacheMu.Lock()
	defer s.wechatCacheMu.Unlock()
	entry, ok := s.wechatCache[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.wechatCache, key)
		return nil
	}
	return entry.Resolved
}

func (s *Service) cacheWechatChannelsResolved(key string, resolved *wechatChannelsResolved) {
	if key == "" || resolved == nil {
		return
	}
	s.wechatCacheMu.Lock()
	defer s.wechatCacheMu.Unlock()
	s.wechatCache[key] = wechatChannelsCacheEntry{
		Resolved:  resolved,
		ExpiresAt: time.Now().Add(wechatChannelsCacheTTL),
	}
}
