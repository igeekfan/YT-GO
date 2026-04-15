package core

import "testing"

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
