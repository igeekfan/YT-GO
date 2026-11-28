const youtubeHosts = new Set([
    'youtube.com', 'www.youtube.com', 'm.youtube.com', 'music.youtube.com',
    'youtu.be', 'www.youtu.be', 'youtube-nocookie.com', 'www.youtube-nocookie.com',
])
const channelPath = /^\/(?:@[^/]+|(?:channel|c|user)\/[^/]+)(?:\/(?:videos|shorts|streams))?$/
const videoPath = /^\/(?:shorts|live|embed|v|e)\/[A-Za-z0-9_-]{11}$/
const shortVideoPath = /^\/[A-Za-z0-9_-]{11}$/

export function getYouTubeCollectionKind(input: string): 'playlist' | 'channel' | 'video-with-playlist' | null {
    const trimmed = input.trim()
    const urlText = /\s/.test(trimmed)
        ? trimmed.match(/https?:\/\/\S+/)?.[0].replace(/[.,;:!?)\]}，。；：！？、）】》」』]+$/, '')
        : trimmed
    if (!urlText) return null

    try {
        const url = new URL(urlText)
        if (!['http:', 'https:'].includes(url.protocol) || url.port || url.username || url.password) return null
        if (!youtubeHosts.has(url.hostname)) return null

        const path = decodeURIComponent(url.pathname).replace(/\/$/, '')
        const isShortLink = url.hostname.endsWith('youtu.be')
        const lists = url.searchParams.getAll('list')
        if (lists.length === 1 && /^[A-Za-z0-9_-]+$/.test(lists[0])) {
            if (isShortLink) return shortVideoPath.test(path) ? 'video-with-playlist' : null
            if (path === '/playlist' || path === '/embed/videoseries') return 'playlist'
            if (path === '/watch' || videoPath.test(path)) return 'video-with-playlist'
        }
        if (!isShortLink && !url.hostname.endsWith('youtube-nocookie.com') && channelPath.test(path)) return 'channel'
        return null
    } catch {
        return null
    }
}
