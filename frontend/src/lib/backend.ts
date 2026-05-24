import * as DesktopApp from '../../wailsjs/go/desktop/App'

// Lightweight interface types for API calls (avoids Wails model class requirements)
interface DownloadRequestPayload {
    url: string
    outputDir: string
    quality: string
    videoInfo?: {
        url: string
        id?: string
        title?: string
        thumbnail?: string
        duration?: number
        uploader?: string
        platform?: string
    }
    options?: {
        saveDescription?: boolean
        saveThumbnail?: boolean
        embedChapters?: boolean
        writeSubtitles?: boolean
        subtitleLangs?: string
        embedSubtitles?: boolean
        sponsorBlock?: boolean
        filenameTemplate?: string
    }
}

interface SettingsPayload {
    outputDir: string
    quality: string
    language: string
    theme: string
    proxy: string
    rateLimit: string
    maxConcurrent: number
    notifications: boolean
    saveDescription: boolean
    saveThumbnail: boolean
    writeSubtitles: boolean
    subtitleLangs: string
    embedSubtitles: boolean
    embedChapters: boolean
    sponsorBlock: boolean
    filenameTemplate: string
    mergeOutputFormat: string
    audioFormat: string
    cookiesFrom: string
    cookiesFile: string
}

const RELEASE_PAGE_URL = 'https://github.com/igeekfan/YT-GO/releases'

export const backendMode: 'desktop' | 'web' =
    typeof window !== 'undefined' && typeof (window as any).go?.desktop?.App !== 'undefined'
        ? 'desktop'
        : 'web'

function apiURL(path: string) {
    const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
    return `${base}${path}`
}

// Runtime web config (fetched from /api/config in web mode)
export interface WebConfig {
    downloadDir: string
    externalURL: string
    hasFixedDir: boolean
}

let _webConfig: WebConfig | null = null

export function getWebConfig(): WebConfig | null {
    return _webConfig
}

export async function fetchWebConfig(): Promise<WebConfig | null> {
    if (backendMode === 'desktop') return null
    try {
        _webConfig = await apiFetch<WebConfig>('/api/config')
        return _webConfig
    } catch {
        return null
    }
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(apiURL(path), {
        ...init,
        headers: {
            'Content-Type': 'application/json',
            ...(init?.headers || {}),
        },
    })

    const text = await response.text()
    const data = text ? JSON.parse(text) : null

    if (!response.ok) {
        const message = data?.error || data?.message || `${response.status} ${response.statusText}`
        throw new Error(message)
    }

    return data as T
}

function getDesktop<T>(call: () => Promise<T>, fallback: () => Promise<T>) {
    return backendMode === 'desktop' ? call() : fallback()
}

export function CheckYtDlp() {
    return getDesktop(() => DesktopApp.CheckYtDlp(), () => apiFetch('/api/ytdlp/status'))
}

export function CheckYtDlpVersion() {
    return getDesktop(
        () => DesktopApp.CheckYtDlpVersion(),
        () => apiFetch('/api/ytdlp/version-check')
    )
}

export function UpdateYtDlp() {
    return getDesktop(
        () => DesktopApp.UpdateYtDlp(),
        async () => {
            const result = await apiFetch<{output: string}>('/api/ytdlp/update', {method: 'POST'})
            return result.output || ''
        }
    )
}

export function InstallYtDlp() {
    return getDesktop(
        () => DesktopApp.InstallYtDlp(),
        async () => {
            const result = await apiFetch<{output: string}>('/api/ytdlp/install', {method: 'POST'})
            return result.output || ''
        }
    )
}

export function UpdateDeno() {
    return getDesktop(
        async () => {
            const fn = (window as any)?.go?.desktop?.App?.UpdateDeno
            if (typeof fn !== 'function') {
                throw new Error('UpdateDeno is not available')
            }
            return await fn()
        },
        async () => {
            const result = await apiFetch<{output: string}>('/api/diagnostics/deno/update', {method: 'POST'})
            return result.output || ''
        }
    )
}

export function GetVideoInfo(url: string) {
    return getDesktop(
        () => DesktopApp.GetVideoInfo(url),
        () => apiFetch('/api/video/info', {method: 'POST', body: JSON.stringify({url})})
    )
}

export function GetPlaylistInfo(url: string) {
    return getDesktop(
        () => DesktopApp.GetPlaylistInfo(url),
        () => apiFetch('/api/video/playlist', {method: 'POST', body: JSON.stringify({url})})
    )
}

export function GetFormats(url: string) {
    return getDesktop(
        () => DesktopApp.GetFormats(url),
        () => apiFetch('/api/video/formats', {method: 'POST', body: JSON.stringify({url})})
    )
}

// SelectFolder: desktop uses native dialog, web uses text input (server-side path)
export function SelectFolder() {
    if (backendMode === 'desktop') {
        return DesktopApp.SelectFolder()
    }
    // Web mode: return empty string, UI should show text input instead of dialog
    return Promise.resolve('')
}

// BrowseDir: list subdirectories on the server (web mode only)
export interface BrowseDirResult {
    path: string
    parent: string
    dirs: string[]
    homeDir: string
}

export function BrowseDir(path: string) {
    return apiFetch<BrowseDirResult>('/api/settings/browse-dir', {
        method: 'POST',
        body: JSON.stringify({path}),
    })
}

// SelectCookiesFile: desktop uses native dialog, web uploads file to server
export function SelectCookiesFile() {
    if (backendMode === 'desktop') {
        return DesktopApp.SelectCookiesFile()
    }
    // Web mode: return empty string, UI should show upload input instead
    return Promise.resolve('')
}

// UploadCookiesFile: upload a cookies file to the server (web mode)
export interface UploadCookiesResult {
    path: string
    name: string
}

export function UploadCookiesFile(file: File) {
    const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
    const formData = new FormData()
    formData.append('file', file)
    return fetch(`${base}/api/cookies/upload`, {
        method: 'POST',
        body: formData,
    }).then(async response => {
        const data = await response.json()
        if (!response.ok) {
            throw new Error(data?.error || 'Upload failed')
        }
        return data as UploadCookiesResult
    })
}

export function StartDownload(request: DownloadRequestPayload) {
    return getDesktop(
        // Wails binding expects the Wails model class; cast is safe because
        // the shape is structurally identical and Wails only reads fields.
        () => DesktopApp.StartDownload(request as any),
        async () => {
            const result = await apiFetch<{id: string}>('/api/downloads', {
                method: 'POST',
                body: JSON.stringify(request),
            })
            return result.id
        }
    )
}

export function GetDownloads() {
    return getDesktop(() => DesktopApp.GetDownloads(), () => apiFetch('/api/downloads'))
}

export function GetSettings() {
    return getDesktop(() => DesktopApp.GetSettings(), () => apiFetch('/api/settings'))
}

export function SaveSettings(settings: SettingsPayload) {
    return getDesktop(
        // Wails binding expects the Wails model class; cast is safe because
        // the shape is structurally identical and Wails only reads fields.
        () => DesktopApp.SaveSettings(settings as any),
        async () => {
            await apiFetch('/api/settings', {
                method: 'POST',
                body: JSON.stringify(settings),
            })
        }
    )
}

export function IsFirstRun() {
    return getDesktop(
        () => DesktopApp.IsFirstRun(),
        async () => {
            const result = await apiFetch<{firstRun: boolean}>('/api/settings/first-run')
            return result.firstRun
        }
    )
}

export function NeedsCookieConfig() {
    return getDesktop(
        () => DesktopApp.NeedsCookieConfig(),
        async () => {
            const result = await apiFetch<{needsCookieConfig: boolean}>('/api/settings/needs-cookie')
            return result.needsCookieConfig
        }
    )
}

export function ResetSettings() {
    return getDesktop(
        () => DesktopApp.ResetSettings(),
        async () => {
            await apiFetch('/api/settings/reset', {method: 'POST'})
        }
    )
}

export function GetCurrentVersion() {
    return getDesktop(
        () => DesktopApp.GetCurrentVersion(),
        async () => {
            const result = await apiFetch<{version: string}>('/api/version')
            return result.version
        }
    )
}

export function GetAboutInfo() {
    return getDesktop(
        async () => {
            const fn = (window as any)?.go?.desktop?.App?.GetAboutInfo
            if (typeof fn !== 'function') {
                throw new Error('GetAboutInfo is not available')
            }
            return await fn()
        },
        () => apiFetch('/api/about')
    )
}

export function CheckForUpdate() {
    return getDesktop(() => DesktopApp.CheckForUpdate(), () => apiFetch('/api/update'))
}

export function OpenReleasePage() {
    return getDesktop(
        () => DesktopApp.OpenReleasePage(),
        async () => {
            window.open(RELEASE_PAGE_URL, '_blank', 'noopener,noreferrer')
        }
    )
}

export function GetDiagnosticInfo() {
    return getDesktop(() => DesktopApp.GetDiagnosticInfo(), () => apiFetch('/api/diagnostics'))
}

export function GetDepStatus() {
    return getDesktop(
        () => DesktopApp.GetDepStatus(),
        () => apiFetch('/api/diagnostics/deps')
    )
}

export function SetLang(lang: string) {
    return getDesktop(
        () => DesktopApp.SetLang(lang),
        async () => {
            await apiFetch('/api/lang', {
                method: 'POST',
                body: JSON.stringify({lang}),
            })
        }
    )
}

export function ClearCompleted() {
    return getDesktop(
        () => DesktopApp.ClearCompleted(),
        async () => {
            await apiFetch('/api/downloads', {method: 'DELETE'})
        }
    )
}

// OpenFile: desktop opens locally, web provides download link
export function OpenFile(path: string) {
    if (backendMode === 'desktop') {
        return DesktopApp.OpenFile(path)
    }
    // Web mode: no-op, UI should use downloadFileURL() instead
    return Promise.resolve(undefined)
}

// OpenFolder: desktop opens locally, web no-op
export function OpenFolder(path: string) {
    if (backendMode === 'desktop') {
        return DesktopApp.OpenFolder(path)
    }
    // Web mode: no-op
    return Promise.resolve(undefined)
}

// getDownloadFileURL returns a URL for downloading a completed file in web mode.
// Uses YTGO_EXTERNAL_URL if configured, otherwise falls back to same-origin.
export function getDownloadFileURL(taskID: string) {
    const externalBase = _webConfig?.externalURL?.replace(/\/$/, '')
    const fallbackBase = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
    const base = externalBase || fallbackBase
    return `${base}/api/downloads/${encodeURIComponent(taskID)}/file`
}

export function CancelDownload(id: string) {
    return getDesktop(
        () => DesktopApp.CancelDownload(id),
        async () => {
            await apiFetch(`/api/downloads/${encodeURIComponent(id)}/cancel`, {method: 'POST'})
        }
    )
}

export function RemoveDownload(id: string) {
    return getDesktop(
        async () => {
            const fn = (window as any)?.go?.desktop?.App?.RemoveDownload
            if (typeof fn !== 'function') {
                throw new Error('RemoveDownload is not available')
            }
            await fn(id)
        },
        async () => {
            await apiFetch(`/api/downloads/${encodeURIComponent(id)}`, {method: 'DELETE'})
        }
    )
}
