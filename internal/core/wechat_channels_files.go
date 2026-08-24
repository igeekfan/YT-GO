package core

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	wechatInvalidFileRe  = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	wechatTemplateKeyRe  = regexp.MustCompile(`%\(([A-Za-z0-9_]+)\)s`)
	wechatWhitespaceRe   = regexp.MustCompile(`\s+`)
	wechatFormatPrefixRe = regexp.MustCompile(`^(?:f|fv|fa):`)
)

func resolveWechatChannelsFormatID(quality string) string {
	value := strings.TrimSpace(quality)
	for {
		next := wechatFormatPrefixRe.ReplaceAllString(value, "")
		if next == value {
			break
		}
		value = next
	}
	if strings.Contains(value, "+") {
		value = strings.Split(value, "+")[0]
	}
	switch value {
	case wechatChannelsPreviewFormat, wechatChannelsOriginFormat, wechatChannelsDefaultFormat:
		return value
	default:
		return wechatChannelsDefaultFormat
	}
}

func buildWechatChannelsOutputPath(req DownloadRequest, settings Settings, info VideoInfo, videoURL string) (string, error) {
	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		return "", fmt.Errorf("output directory is required")
	}
	ext := "mp4"
	if fromURL := extensionFromURL(videoURL); fromURL != "" {
		ext = fromURL
	}

	template := "%(title)s.%(ext)s"
	if strings.TrimSpace(settings.FilenameTemplate) != "" {
		template = strings.TrimSpace(settings.FilenameTemplate)
	}
	if req.Options != nil && strings.TrimSpace(req.Options.FilenameTemplate) != "" {
		template = strings.TrimSpace(req.Options.FilenameTemplate)
	}

	filename := applyWechatFilenameTemplate(template, info, ext)
	filename = sanitizeWechatFilename(filename)
	if filename == "" || filename == "." {
		filename = "WeChat Channels Video." + ext
	}
	if strings.ToLower(filepath.Ext(filename)) != "."+strings.ToLower(ext) {
		filename += "." + ext
	}
	return filepath.Join(outputDir, filename), nil
}

func applyWechatFilenameTemplate(template string, info VideoInfo, ext string) string {
	values := map[string]string{
		"id":          info.ID,
		"title":       firstNonEmpty(info.Title, "WeChat Channels Video"),
		"uploader":    info.Uploader,
		"channel":     info.Uploader,
		"extractor":   wechatChannelsPlatform,
		"platform":    wechatChannelsPlatform,
		"upload_date": "",
		"ext":         ext,
	}
	return wechatTemplateKeyRe.ReplaceAllStringFunc(template, func(match string) string {
		parts := wechatTemplateKeyRe.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return values[parts[1]]
	})
}

func sanitizeWechatFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = wechatInvalidFileRe.ReplaceAllString(name, " ")
	name = wechatWhitespaceRe.ReplaceAllString(name, " ")
	name = strings.TrimSpace(strings.Trim(name, "."))
	if len([]rune(name)) > 180 {
		runes := []rune(name)
		name = string(runes[:180])
		name = strings.TrimSpace(strings.Trim(name, "."))
	}
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if reserved[strings.ToUpper(base)] {
		name = "_" + name
	}
	return name
}

func extensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), ".")
	switch ext {
	case "mp4", "m4v", "mov":
		return ext
	default:
		return ""
	}
}

func ensureUniqueWechatFilePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
}
