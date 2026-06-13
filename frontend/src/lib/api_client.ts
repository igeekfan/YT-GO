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
    _authToken = token
    if (token) sessionStorage.setItem('ytgo_auth_token', token)
    else sessionStorage.removeItem('ytgo_auth_token')
}

export function getAuthToken(): string | null {
    if (_authToken) return _authToken
    _authToken = sessionStorage.getItem('ytgo_auth_token')
    return _authToken
}

export function clearAuthToken() {
    _authToken = null
    sessionStorage.removeItem('ytgo_auth_token')
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...(init?.headers as Record<string, string> || {}),
    }
    const token = getAuthToken()
    if (token) {
        headers['Authorization'] = `Bearer ${token}`
    }
    const response = await fetch(apiURL(path), {
        ...init,
        headers,
    })

    const text = await response.text()

    if (!response.ok) {
        let message: string
        try {
            const data = text ? JSON.parse(text) : null
            if (response.status === 401) clearAuthToken()
            message = data?.error || data?.message || `${response.status} ${response.statusText}`
        } catch {
            message = `${response.status} ${response.statusText}`
        }
        throw new Error(message)
    }

    if (!text) return null as T
    try {
        return JSON.parse(text) as T
    } catch {
        throw new Error(`API returned non-JSON response for ${path}`)
    }
}

export async function fetchWebConfig(): Promise<WebConfig | null> {
    try {
        const cfg = await apiFetch<WebConfig>('/api/config')
        _webConfig = cfg
        return cfg
    } catch {
        return null
    }
}
