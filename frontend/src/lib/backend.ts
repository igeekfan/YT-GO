import {apiFetch, apiFetchBlob, apiURL, getWebConfig, setWebConfig, fetchWebConfig, setAuthToken, getAuthToken, clearAuthToken, onUnauthorized, notifyUnauthorizedIfCurrent} from './api_client'
import type {WebConfig} from './api_client'
import {backendMode} from './backend_mode'
import type {
    DownloadOptions, DownloadRequest, DownloadTask, FormatInfo, IncrementalDownloadRequest,
    IncrementalDownloadResult, IncrementalPlaylistScan, PlaylistInfo, Settings, VideoInfo,
} from '../types'
import type * as DesktopModels from '../../bindings/YT-GO/desktop/models'

// Re-export for consumers that import from backend.ts
export {backendMode, getWebConfig, setWebConfig, fetchWebConfig, setAuthToken, getAuthToken, clearAuthToken, onUnauthorized}
export type {WebConfig}

type DownloadRequestPayload = Omit<DownloadRequest, 'videoInfo' | 'options'> & {
    videoInfo?: Pick<VideoInfo, 'url'> & Partial<Omit<VideoInfo, 'url'>>
    options?: DownloadOptions
}

export type SettingsPayload = Settings

export type SettingsPatch = Partial<SettingsPayload>

const RELEASE_PAGE_URL = 'https://github.com/igeekfan/YT-GO/releases'

type DesktopApp = typeof import('../../bindings/YT-GO/desktop/app')

function getDesktop<T>(call: (app: DesktopApp) => Promise<T>, fallback: () => Promise<T>) {
    return backendMode === 'desktop' ? import('../../bindings/YT-GO/desktop/app').then(call) : fallback()
}

export function CheckYtDlp() {
    return getDesktop(app => app.CheckYtDlp(), () => apiFetch<DesktopModels.YtDlpStatus>('/api/ytdlp/status'))
}

// Uses a protected endpoint in web mode, so a successful response proves that
// the configured bearer token is valid. Desktop mode does not require auth.
export async function VerifyAuthToken() {
    if (backendMode === 'desktop') return
    await apiFetch('/api/version')
}

export function CheckYtDlpVersion() {
    return getDesktop(
        app => app.CheckYtDlpVersion(),
        () => apiFetch<DesktopModels.YtDlpVersionCheck>('/api/ytdlp/version-check')
    )
}

export function UpdateYtDlp() {
    return getDesktop(
        app => app.UpdateYtDlp(),
        async () => {
            const result = await apiFetch<{output: string}>('/api/ytdlp/update', {method: 'POST'})
            return result.output || ''
        }
    )
}

export function InstallYtDlp() {
    return getDesktop(
        app => app.InstallYtDlp(),
        async () => {
            const result = await apiFetch<{output: string}>('/api/ytdlp/install', {method: 'POST'})
            return result.output || ''
        }
    )
}

export function UpdateDeno() {
    return getDesktop(
        app => app.UpdateDeno(),
        async () => {
            const result = await apiFetch<{output: string}>('/api/diagnostics/deno/update', {method: 'POST'})
            return result.output || ''
        }
    )
}

export function GetVideoInfo(url: string) {
    return getDesktop(
        app => app.GetVideoInfo(url),
        () => apiFetch<DesktopModels.VideoInfo>('/api/video/info', {method: 'POST', body: JSON.stringify({url})})
    ).then(normalizeVideoInfo)
}

function normalizeVideoInfo(info: DesktopModels.VideoInfo): VideoInfo {
    return {...info, subtitles: info.subtitles ?? []}
}

export function GetPlaylistInfo(url: string): Promise<PlaylistInfo> {
    return getDesktop(
        app => app.GetPlaylistInfo(url),
        () => apiFetch<DesktopModels.PlaylistInfo>('/api/video/playlist', {method: 'POST', body: JSON.stringify({url})})
    ).then(info => ({...info, videos: (info.videos ?? []).map(normalizeVideoInfo)}))
}

export function ScanYouTubeCollection(url: string): Promise<IncrementalPlaylistScan> {
    return getDesktop(
        app => app.ScanYouTubeCollection(url),
        () => apiFetch<DesktopModels.IncrementalPlaylistScan>('/api/video/playlist/scan', {
            method: 'POST',
            body: JSON.stringify({url}),
        }),
    ).then(scan => ({...scan, newVideos: scan.newVideos ?? []}))
}

export function QueueNewYouTubeVideos(request: IncrementalDownloadRequest): Promise<IncrementalDownloadResult> {
    return getDesktop(
        app => app.QueueNewYouTubeVideos(request),
        () => apiFetch<IncrementalDownloadResult>('/api/video/playlist/queue', {
            method: 'POST',
            body: JSON.stringify(request),
        }),
    )
}

export function GetFormats(url: string): Promise<FormatInfo> {
    return getDesktop(
        app => app.GetFormats(url),
        () => apiFetch<DesktopModels.FormatInfo>('/api/video/formats', {method: 'POST', body: JSON.stringify({url})})
    ).then(info => ({...info, formats: info.formats ?? []}))
}

// SelectFolder: desktop uses native dialog, web uses text input (server-side path)
export function SelectFolder() {
    return getDesktop(app => app.SelectFolder(), () => Promise.resolve(''))
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
    return getDesktop(app => app.SelectCookiesFile(), () => Promise.resolve(''))
}

// UploadCookiesFile: upload a cookies file to the server (web mode)
export interface UploadCookiesResult {
    path: string
    name: string
}

export function UploadCookiesFile(file: File) {
    const requestToken = getAuthToken()
    const formData = new FormData()
    formData.append('file', file)
    return fetch(apiURL('/api/cookies/upload'), {
        method: 'POST',
        headers: requestToken ? {Authorization: `Bearer ${requestToken}`} : {},
        body: formData,
    }).then(async response => {
        if (response.status === 401) notifyUnauthorizedIfCurrent(requestToken)
        const data = await response.json().catch(() => null)
        if (!response.ok) {
            throw new Error(data?.error || 'Upload failed')
        }
        return data as UploadCookiesResult
    })
}

export function StartDownload(request: DownloadRequestPayload) {
    return getDesktop(
        app => app.StartDownload({
            ...request,
            videoInfo: request.videoInfo ? {
                id: '',
                title: '',
                thumbnail: '',
                duration: 0,
                uploader: '',
                platform: '',
                subtitles: null,
                ...request.videoInfo,
            } : null,
            options: request.options ?? null,
        }),
        async () => {
            const result = await apiFetch<{id: string}>('/api/downloads', {
                method: 'POST',
                body: JSON.stringify(request),
            })
            return result.id
        }
    )
}

export function ReviewDownloadURLs(input: string) {
    return getDesktop(
        async app => (await app.ReviewDownloadURLs(input)) ?? [],
        () => apiFetch<DesktopModels.DownloadURLReview[]>('/api/downloads/review', {
            method: 'POST',
            body: JSON.stringify({input}),
        })
    )
}

export function RetryDownload(id: string) {
    return getDesktop(
        app => app.RetryDownload(id),
        async () => {
            const result = await apiFetch<{id: string}>(`/api/downloads/${encodeURIComponent(id)}/retry`, {method: 'POST'})
            return result.id
        }
    )
}

export function GetDownloads() {
    return getDesktop(
        async app => (await app.GetDownloads() ?? []).filter(task => task !== null),
        () => apiFetch<DownloadTask[]>('/api/downloads'),
    )
}

export function GetSettings() {
    return getDesktop<Settings>(app => app.GetSettings(), () => apiFetch<Settings>('/api/settings'))
}

export function SaveSettings(settings: SettingsPayload) {
    return getDesktop(
        app => app.SaveSettings({...settings, updateProxy: settings.updateProxy ?? ''}),
        async () => {
            await apiFetch('/api/settings', {
                method: 'POST',
                body: JSON.stringify(settings),
            })
        }
    )
}

export function ApplySettingsPatch(patch: SettingsPatch): Promise<SettingsPayload> {
    return getDesktop<SettingsPayload>(
        app => app.ApplySettingsPatch(patch),
        () => apiFetch<SettingsPayload>('/api/settings', {
            method: 'PATCH',
            body: JSON.stringify(patch),
        })
    )
}

export function IsFirstRun() {
    return getDesktop(
        app => app.IsFirstRun(),
        async () => {
            const result = await apiFetch<{firstRun: boolean}>('/api/settings/first-run')
            return result.firstRun
        }
    )
}

export function NeedsCookieConfig() {
    return getDesktop(
        app => app.NeedsCookieConfig(),
        async () => {
            const result = await apiFetch<{needsCookieConfig: boolean}>('/api/settings/needs-cookie')
            return result.needsCookieConfig
        }
    )
}

export function ResetSettings() {
    return getDesktop(
        app => app.ResetSettings(),
        async () => {
            await apiFetch('/api/settings/reset', {method: 'POST'})
        }
    )
}

export function GetCurrentVersion() {
    return getDesktop(
        app => app.GetCurrentVersion(),
        async () => {
            const result = await apiFetch<{version: string}>('/api/version')
            return result.version
        }
    )
}

export function GetAboutInfo() {
    return getDesktop(
        app => app.GetAboutInfo(),
        () => apiFetch<DesktopModels.AboutInfo>('/api/about')
    )
}

export function CheckForUpdate() {
    return getDesktop(app => app.CheckForUpdate(), () => apiFetch<DesktopModels.UpdateInfo>('/api/update'))
}

export function OpenReleasePage() {
    return getDesktop(
        app => app.OpenReleasePage(),
        async () => {
            window.open(RELEASE_PAGE_URL, '_blank', 'noopener,noreferrer')
        }
    )
}

export function DownloadAndInstallUpdate() {
    // A web server cannot replace its own running process. Open the release
    // page so the user can download the package and restart the service.
    return getDesktop(app => app.DownloadAndInstallUpdate(), OpenReleasePage)
}

export function GetDiagnosticInfo() {
    return getDesktop(app => app.GetDiagnosticInfo(), () => apiFetch<DesktopModels.DiagnosticInfo>('/api/diagnostics'))
}

export function GetDepStatus() {
    return getDesktop(
        app => app.GetDepStatus(),
        () => apiFetch<DesktopModels.DepStatus>('/api/diagnostics/deps')
    )
}

export function SetLang(lang: string) {
    return getDesktop(
        app => app.SetLang(lang),
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
        app => app.ClearCompleted(),
        async () => {
            await apiFetch('/api/downloads', {method: 'DELETE'})
        }
    )
}

// OpenFile: desktop opens locally, web provides download link
export function OpenFile(path: string) {
    return getDesktop(app => app.OpenFile(path), () => Promise.resolve())
}

// OpenFolder: desktop opens locally, web no-op
export function OpenFolder(path: string) {
    return getDesktop(app => app.OpenFolder(path), () => Promise.resolve())
}

// getDownloadFileURL returns a URL for downloading a completed file in web mode.
// Uses YTGO_EXTERNAL_URL if configured, otherwise falls back to same-origin.
export function getDownloadFileURL(taskID: string) {
    const cfg = getWebConfig()
    const externalBase = cfg?.externalURL?.replace(/\/$/, '')
    const fallbackBase = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
    const base = externalBase || fallbackBase
    return `${base}/api/downloads/${encodeURIComponent(taskID)}/file`
}

// Download through fetch so authenticated web mode can send the bearer token
// without placing a long-lived credential in the URL or browser history.
export async function DownloadFile(taskID: string) {
    const fileURL = getDownloadFileURL(taskID)
    if (getWebConfig()?.authRequired === false) {
        const anchor = document.createElement('a')
        anchor.href = fileURL
        anchor.style.display = 'none'
        document.body.appendChild(anchor)
        anchor.click()
        anchor.remove()
        return
    }

    const {blob, filename} = await apiFetchBlob(fileURL)
    const objectURL = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = objectURL
    anchor.download = filename || 'download'
    anchor.style.display = 'none'
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000)
}

export function CancelDownload(id: string) {
    return getDesktop(
        app => app.CancelDownload(id),
        async () => {
            await apiFetch(`/api/downloads/${encodeURIComponent(id)}/cancel`, {method: 'POST'})
        }
    )
}

export function RemoveDownload(id: string) {
    return getDesktop(
        app => app.RemoveDownload(id),
        async () => {
            await apiFetch(`/api/downloads/${encodeURIComponent(id)}`, {method: 'DELETE'})
        }
    )
}
