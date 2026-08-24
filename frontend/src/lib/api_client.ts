/**
 * Shared HTTP API client for web mode.
 * Extracted from backend.ts to keep it lean and reusable.
 */

export interface WebConfig {
    downloadDir: string
    externalURL: string
    hasFixedDir: boolean
    authRequired: boolean
}

let _webConfig: WebConfig | null = null
let _authToken: string | null = null

type AuthTokenListener = (token: string | null) => void
type UnauthorizedListener = () => void

const authTokenListeners = new Set<AuthTokenListener>()
const unauthorizedListeners = new Set<UnauthorizedListener>()

export class ApiError extends Error {
    constructor(message: string, readonly status: number) {
        super(message)
        this.name = 'ApiError'
    }
}

export function apiURL(path: string) {
    const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
    return `${base}${path}`
}

export function getWebConfig(): WebConfig | null {
    return _webConfig
}

export function setWebConfig(cfg: WebConfig | null) {
    _webConfig = cfg
}

export function setAuthToken(token: string | null) {
    const previous = getAuthToken()
    _authToken = token
    if (token) sessionStorage.setItem('ytgo_auth_token', token)
    else sessionStorage.removeItem('ytgo_auth_token')
    if (previous !== token) {
        for (const listener of authTokenListeners) listener(token)
    }
}

export function getAuthToken(): string | null {
    if (_authToken) return _authToken
    _authToken = sessionStorage.getItem('ytgo_auth_token')
    return _authToken
}

export function clearAuthToken() {
    setAuthToken(null)
}

export function onAuthTokenChange(listener: AuthTokenListener) {
    authTokenListeners.add(listener)
    return () => { authTokenListeners.delete(listener) }
}

export function onUnauthorized(listener: UnauthorizedListener) {
    unauthorizedListeners.add(listener)
    return () => { unauthorizedListeners.delete(listener) }
}

export function notifyUnauthorized() {
    clearAuthToken()
    for (const listener of unauthorizedListeners) listener()
}

export function notifyUnauthorizedIfCurrent(requestToken: string | null) {
    if (getAuthToken() === requestToken) notifyUnauthorized()
}

export function getAuthorizationHeaders(): Record<string, string> {
    const token = getAuthToken()
    return token ? {Authorization: `Bearer ${token}`} : {}
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
    const requestToken = getAuthToken()
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...(init?.headers as Record<string, string> || {}),
    }
    if (requestToken) headers.Authorization = `Bearer ${requestToken}`
    const response = await fetch(apiURL(path), {
        ...init,
        headers,
    })

    const text = await response.text()

    if (response.status === 401) notifyUnauthorizedIfCurrent(requestToken)

    if (!response.ok) {
        let message: string
        try {
            const data = text ? JSON.parse(text) : null
            message = data?.error || data?.message || `${response.status} ${response.statusText}`
        } catch {
            message = `${response.status} ${response.statusText}`
        }
        throw new ApiError(message, response.status)
    }

    if (!text) return null as T
    try {
        return JSON.parse(text) as T
    } catch {
        throw new Error(`API returned non-JSON response for ${path}`)
    }
}

export async function apiFetchBlob(path: string): Promise<{blob: Blob, filename: string}> {
    const url = /^https?:\/\//i.test(path) ? path : apiURL(path)
    const requestToken = getAuthToken()
    const response = await fetch(url, {
        headers: requestToken ? {Authorization: `Bearer ${requestToken}`} : {},
    })

    if (response.status === 401) notifyUnauthorizedIfCurrent(requestToken)
    if (!response.ok) {
        let message = `${response.status} ${response.statusText}`
        try {
            const data = await response.json()
            message = data?.error || data?.message || message
        } catch {
        }
        throw new ApiError(message, response.status)
    }

    const disposition = response.headers.get('Content-Disposition') || ''
    const encodedMatch = disposition.match(/filename\*=UTF-8''([^;]+)/i)
    const plainMatch = disposition.match(/filename="?([^";]+)"?/i)
    let filename = ''
    if (encodedMatch) {
        try { filename = decodeURIComponent(encodedMatch[1]) } catch { filename = encodedMatch[1] }
    } else if (plainMatch) {
        filename = plainMatch[1]
    }
    return {blob: await response.blob(), filename}
}

export async function fetchWebConfig(): Promise<WebConfig> {
    const cfg = await apiFetch<WebConfig>('/api/config')
    _webConfig = cfg
    return cfg
}
