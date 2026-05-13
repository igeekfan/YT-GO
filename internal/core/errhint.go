package core

import (
	"fmt"
	"strings"
)

// describeCookieSource returns a human-readable description of the current cookie configuration.
func describeCookieSource(i *I18n, settings Settings) string {
	if settings.CookiesFrom != "" {
		return fmt.Sprintf(i.T("hint.cookies.browser"), settings.CookiesFrom)
	}
	if settings.CookiesFile != "" {
		return fmt.Sprintf(i.T("hint.cookies.file"), settings.CookiesFile)
	}
	return ""
}

// buildChallengeFailureHint returns a detailed hint when YouTube JS challenge solving fails.
func buildChallengeFailureHint(i *I18n, settings Settings) string {
	cookieHint := describeCookieSource(i, settings)
	selection := detectPreferredJSRuntime(i)
	if selection.Found {
		runtimeLabel := selection.Name
		if selection.Version != "" {
			runtimeLabel = fmt.Sprintf("%s %s", selection.Name, selection.Version)
		}
		return fmt.Sprintf(i.T("hint.challenge.found"), cookieHint, runtimeLabel)
	}
	return fmt.Sprintf(i.T("hint.challenge.missing"), cookieHint, selection.Reason, minimumDenoMajor, minimumNodeMajor)
}

// normalizeYtDlpError enhances raw yt-dlp error messages with actionable hints.
func normalizeYtDlpError(i *I18n, errMsg string, settings Settings) string {
	errMsg = strings.TrimSpace(errMsg)
	if errMsg == "" {
		return errMsg
	}
	if strings.Contains(errMsg, "Fresh cookies") && strings.Contains(errMsg, "Douyin") {
		cookieHint := i.T("hint.cookies.none.douyin")
		if hint := describeCookieSource(i, settings); hint != "" {
			cookieHint = hint
		}
		return fmt.Sprintf(i.T("hint.douyin.cookies"), cookieHint, errMsg)
	}
	if strings.Contains(errMsg, "Sign in to confirm") || strings.Contains(errMsg, "not a bot") {
		cookieHint := i.T("hint.cookies.none")
		if hint := describeCookieSource(i, settings); hint != "" {
			cookieHint = hint
		}
		return fmt.Sprintf(i.T("hint.youtube.signin"), cookieHint, errMsg)
	}
	if strings.Contains(errMsg, "Failed to decrypt with DPAPI") {
		if settings.CookiesFrom != "" {
			return fmt.Sprintf(i.T("hint.dpapi.with_browser"), settings.CookiesFrom, errMsg)
		}
		return fmt.Sprintf(i.T("hint.dpapi.generic"), errMsg)
	}
	if strings.Contains(errMsg, "Signature solving failed") || strings.Contains(errMsg, "n challenge solving failed") || strings.Contains(errMsg, "Only images are available for download") || strings.Contains(errMsg, "Requested format is not available") {
		return fmt.Sprintf(i.T("hint.challenge.raw"), buildChallengeFailureHint(i, settings), errMsg)
	}
	return errMsg
}
