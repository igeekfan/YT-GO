package core_test

import (
	"testing"

	_ "YT-GO/internal/core"
	_ "unsafe"
)

//go:linkname extractDouyinTarget YT-GO/internal/core.extractDouyinTarget
func extractDouyinTarget(input string) (string, string, error)

//go:linkname isDouyinURLInput YT-GO/internal/core.isDouyinURL
func isDouyinURLInput(rawURL string) bool

//go:linkname normalizeThumbnailURL YT-GO/internal/core.normalizeThumbnailURL
func normalizeThumbnailURL(raw string) string

//go:linkname extractThumbnailURL YT-GO/internal/core.extractThumbnailURL
func extractThumbnailURL(raw map[string]interface{}) string

//go:linkname shouldApplyMergeOutputFormat YT-GO/internal/core.shouldApplyMergeOutputFormat
func shouldApplyMergeOutputFormat(quality string) bool

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
	if !isDouyinURLInput("7.52 复制打开抖音，看看【测试账号】发布的视频！ https://v.douyin.com/iAABBccD/ 😄 ") {
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

func TestExtractThumbnailURLFallbacks(t *testing.T) {
	raw := map[string]interface{}{
		"thumbnails": []interface{}{
			map[string]interface{}{"url": "https://i0.hdslb.com/bfs/archive/transparent.png"},
			map[string]interface{}{"url": "//i0.hdslb.com/high.jpg"},
		},
	}
	if got := extractThumbnailURL(raw); got != "https://i0.hdslb.com/high.jpg" {
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
