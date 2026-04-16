import * as DesktopApp from '../../wailsjs/go/desktop/App'

const RELEASE_PAGE_URL = 'https://github.com/igeekfan/YT-GO/releases'

export const backendMode: 'desktop' | 'web' =
    typeof window !== 'undefined' && typeof (window as any).go?.desktop?.App !== 'undefined'
        ? 'desktop'
        : 'web'

function apiURL(path: string) {
    const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
    return `${base}${path}`
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

export function SelectFolder() {
    if (backendMode === 'desktop') {
        return DesktopApp.SelectFolder()
    }

    return new Promise<string>((resolve) => {
        const input = document.createElement('input')
        input.type = 'file'
        input.webkitdirectory = true
        input.style.display = 'none'
        document.body.appendChild(input)

        input.onchange = () => {
            const files = input.files
            let dir = ''
            if (files && files.length > 0) {
                const lastSlash = files[0].webkitRelativePath.lastIndexOf('/')
                dir = lastSlash > 0 ? files[0].webkitRelativePath.substring(0, lastSlash) : ''
            }
            document.body.removeChild(input)
            resolve(dir)
        }

        input.click()
    })
}

export function SelectCookiesFile() {
    if (backendMode === 'desktop') {
        return DesktopApp.SelectCookiesFile()
    }

    return new Promise<string>((resolve) => {
        const input = document.createElement('input')
        input.type = 'file'
        input.accept = '.txt,.cookies'
        input.style.display = 'none'
        document.body.appendChild(input)

        input.onchange = () => {
            const file = input.files?.[0]
            document.body.removeChild(input)
            resolve(file ? file.name : '')
        }

        input.click()
    })
}

export function StartDownload(request: any) {
    return getDesktop(
        () => DesktopApp.StartDownload(request),
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

export function SaveSettings(settings: any) {
    return getDesktop(
        () => DesktopApp.SaveSettings(settings),
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

export function OpenFile(path: string) {
    return getDesktop(() => DesktopApp.OpenFile(path), async () => undefined)
}

export function OpenFolder(path: string) {
    return getDesktop(() => DesktopApp.OpenFolder(path), async () => undefined)
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