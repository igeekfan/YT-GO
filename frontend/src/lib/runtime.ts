import {EventsOn as DesktopEventsOn} from '../../wailsjs/runtime/runtime'
import {getAuthToken} from './backend'

const isDesktop = typeof window !== 'undefined' && typeof (window as any).go?.desktop?.App !== 'undefined'
const eventSourceBase = `${(import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')}/api/events`

type Listener = (data: any) => void

let webEventSource: EventSource | null = null
const webListeners = new Map<string, Set<Listener>>()

function ensureWebEventSource() {
    if (isDesktop || webEventSource) return

    // Append auth token as query param since EventSource can't set headers.
    let url = eventSourceBase
    const token = getAuthToken()
    if (token) url += `?token=${encodeURIComponent(token)}`

    webEventSource = new EventSource(url)
    for (const [eventName] of webListeners) {
        attachWebListener(eventName)
    }
    webEventSource.addEventListener('error', () => {
        if (webEventSource?.readyState === EventSource.CLOSED) {
            webEventSource = null
        }
    })
}

function attachWebListener(eventName: string) {
    if (!webEventSource) return
    webEventSource.addEventListener(eventName, (event: MessageEvent) => {
        const listeners = webListeners.get(eventName)
        if (!listeners || listeners.size === 0) return

        let payload: any = event.data
        try {
            payload = JSON.parse(event.data)
        } catch {
        }

        for (const listener of listeners) {
            listener(payload)
        }
    })
}

function closeWebEventSourceIfIdle() {
    for (const listeners of webListeners.values()) {
        if (listeners.size > 0) return
    }
    if (webEventSource) {
        webEventSource.close()
        webEventSource = null
    }
}

export function EventsOn(eventName: string, callback: (data: any) => void) {
    if (isDesktop) {
        return DesktopEventsOn(eventName, callback)
    }

    const listeners = webListeners.get(eventName) || new Set<Listener>()
    const isNewEvent = !webListeners.has(eventName)
    listeners.add(callback)
    webListeners.set(eventName, listeners)
    ensureWebEventSource()
    if (isNewEvent) {
        attachWebListener(eventName)
    }

    return () => {
        const current = webListeners.get(eventName)
        if (!current) return
        current.delete(callback)
        if (current.size === 0) {
            webListeners.delete(eventName)
        }
        closeWebEventSourceIfIdle()
    }
}