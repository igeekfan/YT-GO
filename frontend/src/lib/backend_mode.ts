type WebViewWindow = Window & {chrome?: {webview?: {postMessage?: unknown}}}

export const backendMode: 'desktop' | 'web' =
    typeof window !== 'undefined' && (
        (window.location.protocol === 'wails:' && window.location.hostname === 'localhost')
        || (window.location.protocol === 'http:' && window.location.hostname === 'wails.localhost'
            && typeof (window as WebViewWindow).chrome?.webview?.postMessage === 'function')
    )
        ? 'desktop'
        : 'web'
