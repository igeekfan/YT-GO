package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lrstanley/go-ytdlp"
)

type passthroughNetworkPolicy struct{}

func (passthroughNetworkPolicy) ConfigureTransport(*http.Transport) {}

func (passthroughNetworkPolicy) CheckRedirect(*http.Request, []*http.Request) error { return nil }

func TestExtractURLFromText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain URL unchanged",
			input: "https://www.youtube.com/watch?v=abc123",
			want:  "https://www.youtube.com/watch?v=abc123",
		},
		{
			name:  "noisy douyin share text",
			input: "3.33 08/10 rEH:/ 嘴上抵制背地里入局  https://v.douyin.com/i193-6eUp6E/ 复制此链接，打开Dou音搜索！",
			want:  "https://v.douyin.com/i193-6eUp6E/",
		},
		{
			name:  "trailing punctuation stripped",
			input: "watch this https://youtu.be/abc，",
			want:  "https://youtu.be/abc",
		},
		{
			name:  "no URL in text",
			input: "no link here",
			want:  "no link here",
		},
	}
	for _, tc := range tests {
		got := extractURLFromText(tc.input)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseWechatChannelsInput(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantShare  string
		wantToken  string
		wantExport string
	}{
		{
			name:      "weixin sph share URL",
			input:     "https://weixin.qq.com/sph/AU4cv2Iccz",
			wantShare: "https://weixin.qq.com/sph/AU4cv2Iccz",
		},
		{
			name:      "channels sph preview URL",
			input:     "https://channels.weixin.qq.com/finder-preview/pages/sph?id=AU4cv2Iccz",
			wantShare: "https://weixin.qq.com/sph/AU4cv2Iccz",
		},
		{
			name:       "playable feed URL",
			input:      "https://channels.weixin.qq.com/finder-preview/pages/feed?token=tok123&eid=export%2Fabc",
			wantToken:  "tok123",
			wantExport: "export/abc",
		},
		{
			name:      "share text",
			input:     "看看这个视频号 https://weixin.qq.com/sph/AkBAEKjbtY，",
			wantShare: "https://weixin.qq.com/sph/AkBAEKjbtY",
		},
	}

	for _, tc := range tests {
		got, err := parseWechatChannelsInput(tc.input)
		if err != nil {
			t.Fatalf("%s: parseWechatChannelsInput returned error: %v", tc.name, err)
		}
		if got.ShareURL != tc.wantShare {
			t.Fatalf("%s: ShareURL got %q, want %q", tc.name, got.ShareURL, tc.wantShare)
		}
		if got.Token != tc.wantToken {
			t.Fatalf("%s: Token got %q, want %q", tc.name, got.Token, tc.wantToken)
		}
		if got.ExportID != tc.wantExport {
			t.Fatalf("%s: ExportID got %q, want %q", tc.name, got.ExportID, tc.wantExport)
		}
	}
}

func TestBuildWechatFeedReferer(t *testing.T) {
	referer := buildWechatFeedReferer("export/a+b/c", "tok+123/=")
	if !strings.Contains(referer, "appid=0") {
		t.Fatalf("referer should force appid=0: %s", referer)
	}
	if !strings.Contains(referer, "entry_scene=0") {
		t.Fatalf("referer should force entry_scene=0: %s", referer)
	}
	if !strings.Contains(referer, "eid=export%2Fa%2Bb%2Fc") {
		t.Fatalf("referer should encode eid from playable_url: %s", referer)
	}
	if !strings.Contains(referer, "token=tok%2B123%2F%3D") {
		t.Fatalf("referer should encode token from playable_url: %s", referer)
	}
}

func TestCleanWechatOriginalVideoURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes nonessential query parameters",
			input: "https://finder.video.qq.com/251/20304/stodownload?adaptivelytrans=0&encfilekey=key%2Fpart&token=tok+value&idx=1#fragment",
			want:  "https://finder.video.qq.com/251/20304/stodownload?encfilekey=key%2Fpart&token=tok+value",
		},
		{
			name:  "decodes encoded URL before cleaning",
			input: "https%3A%2F%2Ffinder.video.qq.com%2F251%2Foriginal.mp4%3Fencfilekey%3Dkey%252Fpart%26token%3Dtok%252Bvalue%26idx%3D1",
			want:  "https://finder.video.qq.com/251/original.mp4?encfilekey=key%2Fpart&token=tok%2Bvalue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanWechatOriginalVideoURL(tc.input); got != tc.want {
				t.Fatalf("cleanWechatOriginalVideoURL got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCleanWechatOriginalVideoURLRequiresAccessParameters(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/video.mp4?encfilekey=key",
		"https://example.com/video.mp4?token=token",
		"https://example.com/video.mp4",
		"not a URL",
	} {
		if got := cleanWechatOriginalVideoURL(rawURL); got != "" {
			t.Fatalf("expected URL without encfilekey and token to be rejected, got %q", got)
		}
	}
}

func TestWechatBestPrefersCleanedOriginalURL(t *testing.T) {
	resolved := buildWechatChannelsResolved(wechatChannelsInput{ID: "sph0", ShareURL: "https://weixin.qq.com/sph/sph0"}, wechatParseData{}, wechatFeedResponse{
		Data: wechatFeedData{FeedInfo: wechatFeedInfo{
			VideoURL: "https://finder.video.qq.com/original.mp4?idx=1&encfilekey=origin%2Fkey&token=origin-token&adaptivelytrans=0",
		}},
	})
	want := "https://finder.video.qq.com/original.mp4?encfilekey=origin%2Fkey&token=origin-token"
	if resolved.VideoURLs[wechatChannelsOriginFormat] != want {
		t.Fatalf("origin format got %q, want %q", resolved.VideoURLs[wechatChannelsOriginFormat], want)
	}
	if resolved.VideoURLs[wechatChannelsDefaultFormat] != want {
		t.Fatalf("best should prefer cleaned original URL, got %q", resolved.VideoURLs[wechatChannelsDefaultFormat])
	}
	if len(resolved.Formats.Formats) != 2 {
		t.Fatalf("expected only origin and preview formats, got %d", len(resolved.Formats.Formats))
	}
	if resolved.Formats.Formats[0].FormatID != wechatChannelsOriginFormat || resolved.Formats.Formats[1].FormatID != wechatChannelsPreviewFormat {
		t.Fatalf("unexpected WeChat Channels formats: %+v", resolved.Formats.Formats)
	}
}

func TestWechatBestFallsBackToPreview(t *testing.T) {
	resolved := buildWechatChannelsResolved(wechatChannelsInput{ID: "sph1", ShareURL: "https://weixin.qq.com/sph/sph1"}, wechatParseData{}, wechatFeedResponse{
		Data: wechatFeedData{FeedInfo: wechatFeedInfo{
			VideoURL: "https://example.com/preview.mp4",
		}},
	})
	if resolved.VideoURLs[wechatChannelsDefaultFormat] != "https://example.com/preview.mp4" {
		t.Fatalf("best should fall back to preview when original is unavailable, got %q", resolved.VideoURLs[wechatChannelsDefaultFormat])
	}
	if len(resolved.Formats.Formats) != 1 {
		t.Fatalf("expected only preview format, got %d", len(resolved.Formats.Formats))
	}
	if resolved.Formats.Formats[0].FormatID != wechatChannelsPreviewFormat {
		t.Fatalf("expected preview format, got %q", resolved.Formats.Formats[0].FormatID)
	}
}

func TestWechatFormatsOnlyExposeOriginAndPreview(t *testing.T) {
	resolved := buildWechatChannelsResolved(wechatChannelsInput{ID: "sph2", ShareURL: "https://weixin.qq.com/sph/sph2"}, wechatParseData{}, wechatFeedResponse{
		Data: wechatFeedData{FeedInfo: wechatFeedInfo{
			VideoURL: "https://example.com/original.mp4?encfilekey=key&token=token&idx=1",
		}},
	})
	if len(resolved.Formats.Formats) != 2 {
		t.Fatalf("expected only origin and preview formats, got %d", len(resolved.Formats.Formats))
	}
	for _, format := range resolved.Formats.Formats {
		if format.FormatID != wechatChannelsOriginFormat && format.FormatID != wechatChannelsPreviewFormat {
			t.Fatalf("unexpected codec format exposed: %q", format.FormatID)
		}
	}
}

func TestWechatChannelsSidecarsAreWritten(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("thumbnail-data"))
	}))
	defer server.Close()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "video.mp4")
	saveThumbnail := true
	saveDescription := true
	service := NewService("test")
	service.outboundPolicy = passthroughNetworkPolicy{}
	service.saveWechatChannelsSidecars(context.Background(), "task", outputPath, VideoInfo{
		Title:     "video description",
		Thumbnail: server.URL + "/cover",
	}, Settings{}, &DownloadOptions{
		SaveThumbnail:   &saveThumbnail,
		SaveDescription: &saveDescription,
	})

	thumbnail, err := os.ReadFile(filepath.Join(dir, "video.png"))
	if err != nil {
		t.Fatalf("expected thumbnail sidecar: %v", err)
	}
	if string(thumbnail) != "thumbnail-data" {
		t.Fatalf("unexpected thumbnail content: %q", thumbnail)
	}
	description, err := os.ReadFile(filepath.Join(dir, "video.description"))
	if err != nil {
		t.Fatalf("expected description sidecar: %v", err)
	}
	if string(description) != "video description" {
		t.Fatalf("unexpected description content: %q", description)
	}
}

func TestWechatChannelsMetadataPrefersYuanbaoParseResult(t *testing.T) {
	resolved := buildWechatChannelsResolved(wechatChannelsInput{ShareURL: "https://weixin.qq.com/sph/test"}, wechatParseData{
		Desc:     "yuanbao title",
		Author:   "yuanbao channel",
		CoverURL: "https://example.com/yuanbao.jpg",
	}, wechatFeedResponse{Data: wechatFeedData{
		AuthorInfo: wechatAuthorInfo{Nickname: "preview channel"},
		FeedInfo: wechatFeedInfo{
			Description: "preview title",
			CoverURL:    "https://example.com/preview.jpg",
			VideoURL:    "https://example.com/video.mp4?encfilekey=key&token=token",
		},
	}})

	if resolved.Info.Title != "yuanbao title" {
		t.Fatalf("expected Yuanbao title, got %q", resolved.Info.Title)
	}
	if resolved.Info.Uploader != "yuanbao channel" {
		t.Fatalf("expected Yuanbao channel, got %q", resolved.Info.Uploader)
	}
	if resolved.Info.Thumbnail != "https://example.com/yuanbao.jpg" {
		t.Fatalf("expected Yuanbao thumbnail, got %q", resolved.Info.Thumbnail)
	}
}

func TestWechatChannelsResolvedCache(t *testing.T) {
	service := NewService("test")
	want := &wechatChannelsResolved{Info: VideoInfo{Title: "cached"}}
	service.cacheWechatChannelsResolved("https://weixin.qq.com/sph/test", want)
	if got := service.getCachedWechatChannelsResolved("https://weixin.qq.com/sph/test"); got != want {
		t.Fatalf("expected cached resolved result, got %#v", got)
	}
}

func TestReadCookiesFromFileMatchesLeadingDotDomain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.txt")
	content := ".yuanbao.tencent.com\tTRUE\t/\tTRUE\t1893456000\tsessionid\tabc123\n#HttpOnly_.yuanbao.tencent.com\tTRUE\t/\tTRUE\t1893456000\tlogin\tdef456\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write cookies fixture: %v", err)
	}

	cookies, err := readCookiesFromFile(path, "yuanbao.tencent.com")
	if err != nil {
		t.Fatalf("readCookiesFromFile returned error: %v", err)
	}
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}
	if cookies[0].Name != "sessionid" || cookies[0].Value != "abc123" {
		t.Fatalf("unexpected cookie: %#v", cookies[0])
	}
	if cookies[1].Name != "login" || cookies[1].Value != "def456" {
		t.Fatalf("unexpected HttpOnly cookie: %#v", cookies[1])
	}
}

func TestExtractDouyinTargetFromShareText(t *testing.T) {
	url, videoID, err := extractDouyinTarget("7.52 复制打开抖音，看看【测试账号】发布的视频！ https://v.douyin.com/iAABBccD/ 😄 ")
	if err != nil {
		t.Fatalf("extractDouyinTarget returned error: %v", err)
	}
	if url != "https://v.douyin.com/iAABBccD/" {
		t.Fatalf("unexpected url: %s", url)
	}
	if videoID != "" {
		t.Fatalf("expected empty direct video id, got %s", videoID)
	}
	if !isDouyinURL("7.52 复制打开抖音，看看【测试账号】发布的视频！ https://v.douyin.com/iAABBccD/ 😄 ") {
		t.Fatal("expected noisy share text to be recognized as douyin input")
	}
}

func TestExtractDouyinTargetFromDirectVideoURL(t *testing.T) {
	url, videoID, err := extractDouyinTarget("作者分享：https://www.douyin.com/video/7483920012345678901?previous_page=app_code_link")
	if err != nil {
		t.Fatalf("extractDouyinTarget returned error: %v", err)
	}
	if url != "https://www.douyin.com/video/7483920012345678901" {
		t.Fatalf("unexpected canonical url: %s", url)
	}
	if videoID != "7483920012345678901" {
		t.Fatalf("unexpected video id: %s", videoID)
	}
}

func TestExtractDouyinTargetFromDirectID(t *testing.T) {
	url, videoID, err := extractDouyinTarget("7483920012345678901")
	if err != nil {
		t.Fatalf("extractDouyinTarget returned error: %v", err)
	}
	if url != "https://www.douyin.com/video/7483920012345678901" {
		t.Fatalf("unexpected canonical url: %s", url)
	}
	if videoID != "7483920012345678901" {
		t.Fatalf("unexpected video id: %s", videoID)
	}
}

func TestNormalizeThumbnailURL(t *testing.T) {
	if got := normalizeThumbnailURL("//i0.hdslb.com/bfs/archive/test.jpg"); got != "https://i0.hdslb.com/bfs/archive/test.jpg" {
		t.Fatalf("unexpected normalized thumbnail url: %s", got)
	}
}

func TestExtractThumbnailFromExtractedFallbacks(t *testing.T) {
	raw := &ytdlp.ExtractedInfo{
		Thumbnails: []*ytdlp.ExtractedThumbnail{
			{URL: "https://i0.hdslb.com/bfs/archive/transparent.png"},
			{URL: "//i0.hdslb.com/high.jpg"},
		},
	}
	if got := extractThumbnailFromExtracted(raw); got != "https://i0.hdslb.com/high.jpg" {
		t.Fatalf("unexpected thumbnail fallback: %s", got)
	}
}

func TestShouldApplyMergeOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		want    bool
	}{
		{name: "preset quality", quality: "best", want: true},
		{name: "preset quality 1080p", quality: "1080p", want: true},
		{name: "audio only preset", quality: "audio", want: false},
		{name: "single custom format", quality: "f:137", want: false},
		{name: "single custom video track", quality: "fv:137", want: false},
		{name: "single custom audio track", quality: "fa:140", want: false},
		{name: "combined custom formats", quality: "f:137+140", want: false},
	}

	for _, test := range tests {
		if got := shouldApplyMergeOutputFormat(test.quality); got != test.want {
			t.Fatalf("%s: expected %v, got %v", test.name, test.want, got)
		}
	}
}
