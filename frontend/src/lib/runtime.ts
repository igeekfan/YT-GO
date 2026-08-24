import {EventsOn as DesktopEventsOn} from '../../wailsjs/runtime/runtime'
import {apiFetch, getAuthToken, onAuthTokenChange} from './api_client'

const isDesktop = typeof window !== 'undefined' && typeof (window as any).go?.desktop?.App !== 'undefined'
const eventSourceBase = `${(import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')}/api/events`
const RECONNECT_DELAY_MS = 1000

type Listener = (data: any) => void
type ConnectionListener = () => void

let webEventSource: EventSource | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
const webListeners = new Map<string, Set<Listener>>()
const webEventHandlers = new Map<string, EventListener>()
const connectionListeners = new Set<ConnectionListener>()

function hasWebListeners() {
    for (const listeners of webListeners.values()) {
        if (listeners.size > 0) return true
    }
    return false
}

function buildEventSourceURL() {
    const token = getAuthToken()
    return token ? `${eventSourceBase}?token=${encodeURIComponent(token)}` : eventSourceBase
}

function getOrCreateWebEventHandler(eventName: string) {
    const existing = webEventHandlers.get(eventName)
    if (existing) return existing

    const handler: EventListener = (event) => {
        const listeners = webListeners.get(eventName)
        if (!listeners || listeners.size === 0) return

        const message = event as MessageEvent
        let payload: any = message.data
        try {
            payload = JSON.parse(message.data)
        } catch {
        }
        for (const listener of [...listeners]) listener(payload)
    }
    webEventHandlers.set(eventName, handler)
    return handler
}

function attachAllWebEventHandlers(source: EventSource) {
    for (const eventName of webListeners.keys()) {
        source.addEventListener(eventName, getOrCreateWebEventHandler(eventName))
    }
}

function clearReconnectTimer() {
    if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
    }
}

function closeWebEventSource() {
    clearReconnectTimer()
    if (webEventSource) {
        webEventSource.close()
        webEventSource = null
    }
}

function scheduleReconnect() {
    if (isDesktop || reconnectTimer !== null || !hasWebListeners()) return
    reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        ensureWebEventSource()
    }, RECONNECT_DELAY_MS)
}

async function handleClosedConnection() {
    // EventSource does not expose the HTTP status. Probe a lightweight
    // protected endpoint so a revoked token still reaches App's 401 handler.
    if (getAuthToken()) {
        try { await apiFetch('/api/version') }
        catch {
            if (!getAuthToken()) return
        }
    }
    scheduleReconnect()
}

function ensureWebEventSource() {
    if (isDesktop || webEventSource || !hasWebListeners()) return

    const source = new EventSource(buildEventSourceURL())
    webEventSource = source
    attachAllWebEventHandlers(source)

    source.addEventListener('open', () => {
        if (source !== webEventSource) return
        clearReconnectTimer()
        for (const listener of [...connectionListeners]) listener()
    })
    source.addEventListener('error', () => {
        if (source !== webEventSource || source.readyState !== EventSource.CLOSED) return
        webEventSource = null
        void handleClosedConnection()
    })
}

function resetWebEventSource(reconnect: boolean) {
    closeWebEventSource()
    if (reconnect) ensureWebEventSource()
}

onAuthTokenChange(token => {
    if (isDesktop) return
    // A cleared token usually means a 401. Wait for App to return to login
    // instead of immediately opening another unauthorized connection.
    resetWebEventSource(!!token)
})

export function EventsOn<T = unknown>(eventName: string, callback: (data: T) => void) {
    if (isDesktop) {
        return DesktopEventsOn(eventName, callback)
    }

    const listeners = webListeners.get(eventName) || new Set<Listener>()
    const isNewEvent = !webListeners.has(eventName)
    listeners.add(callback as Listener)
    webListeners.set(eventName, listeners)

    if (webEventSource && isNewEvent) {
        webEventSource.addEventListener(eventName, getOrCreateWebEventHandler(eventName))
    }
    ensureWebEventSource()

    return () => {
        const current = webListeners.get(eventName)
        if (!current) return
        current.delete(callback as Listener)
        if (current.size === 0) {
            webListeners.delete(eventName)
            const handler = webEventHandlers.get(eventName)
            if (handler && webEventSource) webEventSource.removeEventListener(eventName, handler)
            webEventHandlers.delete(eventName)
        }
        if (!hasWebListeners()) closeWebEventSource()
    }
}

// Called after the SSE connection opens or reopens. Consumers should use this
// to reconcile transient event state with a server snapshot.
export function EventsOnConnectionOpen(callback: ConnectionListener) {
    if (isDesktop) return () => {}
    connectionListeners.add(callback)
    return () => { connectionListeners.delete(callback) }
}

// Test-only cleanup kept explicit so module-level EventSource state cannot leak
// between isolated runtime tests.
export function resetRuntimeForTests() {
    closeWebEventSource()
    webListeners.clear()
    webEventHandlers.clear()
    connectionListeners.clear()
}
