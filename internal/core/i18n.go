package core

import "sync"

type Lang string

const (
	LangZhCN Lang = "zh-CN"
	LangEnUS Lang = "en-US"
)

// translations holds all backend user-facing strings keyed by language.
var translations = map[Lang]map[string]string{
	LangZhCN: {
		// errhint.go — cookie source description
		"hint.cookies.browser":      "当前使用浏览器 Cookies: %s。",
		"hint.cookies.file":         "当前使用 cookies 文件: %s。",
		"hint.cookies.none":         "当前未配置 Cookies。",
		"hint.cookies.none.douyin":  "当前未配置抖音 Cookies。",
		// errhint.go — YouTube challenge hints
		"hint.challenge.found":     "yt-dlp 已读取到 YouTube 页面，但当前环境仍未完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s当前已检测到可用的 %s。请先更新 yt-dlp 到最新版本；如果仍失败，再按 yt-dlp 的 EJS 指南检查 challenge solver 组件来源。",
		"hint.challenge.missing":   "yt-dlp 已读取到 YouTube 页面，但当前环境无法完成签名/JS challenge 求解，所以只拿到了图片 storyboard，没有拿到真实视频格式。%s%s请升级到 Deno >= %d.0.0（推荐）或 Node.js >= %d.0.0，并在安装后重启应用。",
		"hint.challenge.raw":       "%s 原始错误: %s",
		// errhint.go — Douyin cookies
		"hint.douyin.cookies":      "抖音需要有效的登录 Cookies 才能访问该视频。%s请登录 www.douyin.com 后，使用浏览器扩展导出 cookies.txt 并在设置中配置，或改用「从浏览器导入 Cookies」。原始错误: %s",
		// errhint.go — YouTube sign-in
		"hint.youtube.signin":      "YouTube 拒绝了当前访问，请求被判定为需要登录验证。%s这通常表示 Cookies 已过期、导出不完整、账号未登录 YouTube，或当前代理/IP 风险较高。请重新导出最新的 YouTube cookies.txt，或改用「从浏览器导入 Cookies」。原始错误: %s",
		// errhint.go — DPAPI
		"hint.dpapi.with_browser":  "无法读取浏览器 Cookies（DPAPI 解密失败）。当前浏览器来源: %s。请先关闭该浏览器，并确保 YT-GO 不是以管理员身份运行；如果仍失败，请改用导出的 cookies.txt 文件。原始错误: %s",
		"hint.dpapi.generic":       "Cookies 解密失败（DPAPI）。请检查是否以相同 Windows 用户运行，并避免管理员身份运行；必要时改用导出的 cookies.txt 文件。原始错误: %s",
		// jsruntime.go — probe reasons
		"runtime.deno.found_but_failed": "当前检测到 Deno 路径 %s，但无法正常执行。",
		"runtime.deno.too_old":          "当前检测到 Deno %s，但版本过低，yt-dlp EJS 至少需要 Deno %d.0.0。",
		"runtime.node.found_but_failed": "当前检测到 Node.js 路径 %s，但无法正常执行。",
		"runtime.node.too_old":          "当前检测到 Node.js %s，但版本过低，yt-dlp EJS 至少需要 Node.js %d.0.0。",
		"runtime.none_found":            "当前未检测到可用的 Deno 或 Node.js。",
		// jsruntime.go — ensureYouTubeJSRuntime
		"runtime.youtube.need_js":       "当前链接属于 YouTube，yt-dlp 需要可用的 JS runtime 才能完成签名/JS challenge 求解。%s%s请升级到 Deno >= %d.0.0（推荐）或 Node.js >= %d.0.0，并在安装后完全重启应用再重试。",
		"runtime.missing_generic":       "当前缺少可用的 JS runtime。",
	},
	LangEnUS: {
		// errhint.go — cookie source description
		"hint.cookies.browser":      "Currently using browser cookies: %s.",
		"hint.cookies.file":         "Currently using cookies file: %s.",
		"hint.cookies.none":         "No cookies configured.",
		"hint.cookies.none.douyin":  "No Douyin cookies configured.",
		// errhint.go — YouTube challenge hints
		"hint.challenge.found":     "yt-dlp loaded the YouTube page, but the current environment could not complete signature/JS challenge solving, so only image storyboards were retrieved instead of real video formats. %sA working %s runtime was detected. Please update yt-dlp to the latest version; if the issue persists, check the EJS challenge solver component source per yt-dlp's guide.",
		"hint.challenge.missing":   "yt-dlp loaded the YouTube page, but the current environment cannot complete signature/JS challenge solving, so only image storyboards were retrieved instead of real video formats. %s%sPlease upgrade to Deno >= %d.0.0 (recommended) or Node.js >= %d.0.0, and restart the app after installation.",
		"hint.challenge.raw":       "%s Original error: %s",
		// errhint.go — Douyin cookies
		"hint.douyin.cookies":      "Douyin requires valid login cookies to access this video. %sPlease log in to www.douyin.com, export cookies.txt using a browser extension and configure it in settings, or use \"Import cookies from browser\". Original error: %s",
		// errhint.go — YouTube sign-in
		"hint.youtube.signin":      "YouTube rejected the current access; the request was flagged as requiring sign-in verification. %sThis usually means cookies have expired, the export was incomplete, the account is not logged in to YouTube, or the proxy/IP risk level is high. Please re-export the latest YouTube cookies.txt, or use \"Import cookies from browser\". Original error: %s",
		// errhint.go — DPAPI
		"hint.dpapi.with_browser":  "Failed to read browser cookies (DPAPI decryption failed). Current browser source: %s. Please close that browser first and make sure YT-GO is not running as administrator; if it still fails, use an exported cookies.txt file instead. Original error: %s",
		"hint.dpapi.generic":       "Cookie decryption failed (DPAPI). Please make sure you are running as the same Windows user and avoid running as administrator; use an exported cookies.txt file if necessary. Original error: %s",
		// jsruntime.go — probe reasons
		"runtime.deno.found_but_failed": "Deno detected at %s, but failed to execute.",
		"runtime.deno.too_old":          "Deno %s detected, but the version is too old. yt-dlp EJS requires Deno >= %d.0.0.",
		"runtime.node.found_but_failed": "Node.js detected at %s, but failed to execute.",
		"runtime.node.too_old":          "Node.js %s detected, but the version is too old. yt-dlp EJS requires Node.js >= %d.0.0.",
		"runtime.none_found":            "No usable Deno or Node.js runtime detected.",
		// jsruntime.go — ensureYouTubeJSRuntime
		"runtime.youtube.need_js":       "The URL belongs to YouTube, which requires a usable JS runtime for signature/JS challenge solving. %s%sPlease upgrade to Deno >= %d.0.0 (recommended) or Node.js >= %d.0.0, and fully restart the app after installation before retrying.",
		"runtime.missing_generic":       "No usable JS runtime available.",
	},
}

// I18n provides language-aware string lookups.
type I18n struct {
	mu   sync.RWMutex
	lang Lang
}

// NewI18n creates an I18n instance with the default language (zh-CN).
func NewI18n() *I18n {
	return &I18n{lang: LangZhCN}
}

// SetLang switches the active language.
func (i *I18n) SetLang(lang Lang) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if lang == LangZhCN || lang == LangEnUS {
		i.lang = lang
	}
}

// GetLang returns the active language.
func (i *I18n) GetLang() Lang {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.lang
}

// T returns the translation for the given key in the active language.
// If the key is missing in the active language, it falls back to zh-CN.
// If the key is not found at all, the key itself is returned.
func (i *I18n) T(key string) string {
	i.mu.RLock()
	lang := i.lang
	i.mu.RUnlock()
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if m, ok := translations[LangZhCN]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}
