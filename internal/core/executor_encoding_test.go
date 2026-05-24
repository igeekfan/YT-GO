package core

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func mustGBK(t *testing.T, s string) []byte {
	t.Helper()
	out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatalf("encode %q to GBK: %v", s, err)
	}
	return out
}

func TestToUTF8PureUTF8(t *testing.T) {
	in := []byte("下载完成: 测试视频.mp4")
	got := toUTF8(in)
	if got != string(in) {
		t.Fatalf("expected pure UTF-8 to pass through unchanged, got %q", got)
	}
}

func TestToUTF8PureGBK(t *testing.T) {
	want := "下载失败: 文件不存在"
	in := mustGBK(t, want)
	got := toUTF8(in)
	if got != want {
		t.Fatalf("expected GBK decoded to %q, got %q", want, got)
	}
}

func TestToUTF8MixedLines(t *testing.T) {
	utf8Line := "[download] Destination: 视频.mp4"
	gbkLine := string(mustGBK(t, "ffmpeg 合并失败: 找不到输入文件"))
	var raw strings.Builder
	raw.WriteString(utf8Line)
	raw.WriteString("\n")
	raw.WriteString(gbkLine)
	raw.WriteString("\n")
	raw.WriteString(utf8Line)

	got := toUTF8([]byte(raw.String()))
	wantLines := []string{
		utf8Line,
		"ffmpeg 合并失败: 找不到输入文件",
		utf8Line,
	}
	want := strings.Join(wantLines, "\n")
	if got != want {
		t.Fatalf("mixed encoding decode mismatch:\nwant: %q\n got: %q", want, got)
	}
}

func TestToUTF8CarriageReturnSeparator(t *testing.T) {
	utf8Line := "[download]  50.0% of 10.00MiB"
	gbkLine := string(mustGBK(t, "下载中"))
	raw := utf8Line + "\r" + gbkLine
	got := toUTF8([]byte(raw))
	want := utf8Line + "\r" + "下载中"
	if got != want {
		t.Fatalf("CR separator decode mismatch:\nwant: %q\n got: %q", want, got)
	}
}
