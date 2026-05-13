package core

import (
	"fmt"
	"strings"
)

// describeCookieSource returns a human-readable description of the current cookie configuration.
func describeCookieSource(settings Settings) string {
	if settings.CookiesFrom != "" {
		return fmt.Sprintf("当前使用浏览器 Cookies: %s。", settings.CookiesFrom)
	}
	if settings.CookiesFile != "" {
		return fmt.Sprintf("当前使用 cookies 文件: %s。", settings.CookiesFile)
	}
	return ""
}

// buildChallengeFailureHint returns a detailed hint when YouTube JS challenge solving fails.
func buildChallengeFailureHint(settings Settings) string {
	cookieHint := describeCookieSource(settings)
	selection := detectPreferredJSRuntime()
	if selection.Found {
		runtimeLabel := selection.Name
		if selection.Version != "" {
			runtimeLabel = fmt.Sprintf("%s %s", selection.Name, selection.Version)
		}
		return fmt.Sprintf("yt-dlp 已读取到 YouTube 页面，但当前环境仍未完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s当前已检测到可用的 %s。请先更新 yt-dlp 到最新版本；如果仍失败，再按 yt-dlp 的 EJS 指南检查 challenge solver 组件来源。", cookieHint, runtimeLabel)
	}
	return fmt.Sprintf("yt-dlp 已读取到 YouTube 页面，但当前环境无法完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s%s请升级到 Deno >= %d.0.0（推荐）或 Node.js >= %d.0.0，并在安装后重启应用。", cookieHint, selection.Reason, minimumDenoMajor, minimumNodeMajor)
}

// normalizeYtDlpError enhances raw yt-dlp error messages with actionable hints.
func normalizeYtDlpError(errMsg string, settings Settings) string {
	errMsg = strings.TrimSpace(errMsg)
	if errMsg == "" {
		return errMsg
	}
	if strings.Contains(errMsg, "Fresh cookies") && strings.Contains(errMsg, "Douyin") {
		cookieHint := "当前未配置抖音 Cookies。"
		if hint := describeCookieSource(settings); hint != "" {
			cookieHint = hint
		}
		return fmt.Sprintf("抖音需要有效的登录 Cookies 才能访问该视频。%s请登录 www.douyin.com 后，使用浏览器扩展导出 cookies.txt 并在设置中配置，或改用\u300c从浏览器导入 Cookies\u300d。原始错误: %s", cookieHint, errMsg)
	}
	if strings.Contains(errMsg, "Sign in to confirm") || strings.Contains(errMsg, "not a bot") {
		cookieHint := "当前未配置 Cookies。"
		if hint := describeCookieSource(settings); hint != "" {
			cookieHint = hint
		}
		return fmt.Sprintf("YouTube 拒绝了当前访问，请求被判定为需要登录验证。%s这通常表示 Cookies 已过期、导出不完整、账号未登录 YouTube，或当前代理/IP 风险较高。请重新导出最新的 YouTube cookies.txt，或改用\u300c从浏览器导入 Cookies\u300d。原始错误: %s", cookieHint, errMsg)
	}
	if strings.Contains(errMsg, "Failed to decrypt with DPAPI") {
		if settings.CookiesFrom != "" {
			return fmt.Sprintf("无法读取浏览器 Cookies（DPAPI 解密失败）。当前浏览器来源: %s。请先关闭该浏览器，并确保 YT-GO 不是以管理员身份运行；如果仍失败，请改用导出的 cookies.txt 文件。原始错误: %s", settings.CookiesFrom, errMsg)
		}
		return fmt.Sprintf("Cookies 解密失败（DPAPI）。请检查是否以相同 Windows 用户运行，并避免管理员身份运行；必要时改用导出的 cookies.txt 文件。原始错误: %s", errMsg)
	}
	if strings.Contains(errMsg, "Signature solving failed") || strings.Contains(errMsg, "n challenge solving failed") || strings.Contains(errMsg, "Only images are available for download") || strings.Contains(errMsg, "Requested format is not available") {
		return fmt.Sprintf("%s 原始错误: %s", buildChallengeFailureHint(settings), errMsg)
	}
	return errMsg
}
