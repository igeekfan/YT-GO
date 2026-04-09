import {useState, useEffect, useCallback} from 'react'
import {CheckYtDlp, GetVideoInfo, GetPlaylistInfo, GetDefaultDownloadDir, SelectFolder, StartDownload, GetDownloads} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {YtDlpStatus, VideoInfo, PlaylistInfo, DownloadTask} from './types'
import {useI18n} from './i18n/context'
import DownloadList from './components/DownloadList'
import './App.css'

const QUALITY_OPTIONS = ['best', '1080p', '720p', '480p', '360p', 'audio']

function formatDuration(seconds: number): string {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = Math.floor(seconds % 60)
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    return `${m}:${String(s).padStart(2, '0')}`
}

function App() {
    const {t, lang, setLang} = useI18n()
    const [theme, setTheme] = useState<'dark' | 'light'>(() =>
        (localStorage.getItem('ytgoto-theme') as 'dark' | 'light') || 'dark'
    )
    const [ytdlp, setYtdlp] = useState<YtDlpStatus | null>(null)
    const [url, setUrl] = useState('')
    const [videoInfo, setVideoInfo] = useState<VideoInfo | null>(null)
    const [playlistInfo, setPlaylistInfo] = useState<PlaylistInfo | null>(null)
    const [quality, setQuality] = useState('best')
    const [outputDir, setOutputDir] = useState('')
    const [isGettingInfo, setIsGettingInfo] = useState(false)
    const [isStarting, setIsStarting] = useState(false)
    const [downloads, setDownloads] = useState<DownloadTask[]>([])
    const [toast, setToast] = useState<string | null>(null)

    const showToast = useCallback((msg: string) => {
        setToast(msg)
        setTimeout(() => setToast(null), 3000)
    }, [])

    useEffect(() => {
        document.documentElement.setAttribute('data-theme', theme)
        localStorage.setItem('ytgoto-theme', theme)
    }, [theme])

    useEffect(() => {
        CheckYtDlp().then(setYtdlp).catch(() => setYtdlp({available: false, version: '', path: ''}))
        GetDefaultDownloadDir().then(dir => { if (dir) setOutputDir(dir) }).catch(() => {})
        GetDownloads().then(tasks => { if (tasks) setDownloads(tasks) }).catch(() => {})

        const off = EventsOn('download:update', (task: DownloadTask) => {
            setDownloads(prev => {
                const idx = prev.findIndex(d => d.id === task.id)
                if (idx >= 0) {
                    const next = [...prev]
                    next[idx] = task
                    return next
                }
                return [task, ...prev]
            })
        })
        return () => { if (typeof off === 'function') off() }
    }, [])

    const handleGetInfo = async () => {
        if (!url.trim()) return
        setIsGettingInfo(true)
        setVideoInfo(null)
        setPlaylistInfo(null)
        try {
            const info = await GetVideoInfo(url.trim())
            setVideoInfo(info)
        } catch (e: any) {
            // Try as playlist if single video fetch fails or URL looks like a playlist
            try {
                const plist = await GetPlaylistInfo(url.trim())
                if (plist && plist.count > 0) {
                    setPlaylistInfo(plist)
                } else {
                    showToast(t('toast.getInfoFail') + (e?.message ? `: ${e.message}` : ''))
                }
            } catch {
                showToast(t('toast.getInfoFail') + (e?.message ? `: ${e.message}` : ''))
            }
        } finally {
            setIsGettingInfo(false)
        }
    }

    const handleDownload = async () => {
        if (!url.trim()) return
        if (!outputDir) {
            showToast(t('download.noDir'))
            return
        }
        setIsStarting(true)
        try {
            await StartDownload({
                url: url.trim(),
                outputDir,
                quality,
                videoInfo: videoInfo || undefined,
            } as any)
            setUrl('')
            setVideoInfo(null)
            setPlaylistInfo(null)
        } catch (e: any) {
            showToast(t('toast.downloadStartFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsStarting(false)
        }
    }

    const handleDownloadAll = async () => {
        if (!playlistInfo || !outputDir) {
            if (!outputDir) showToast(t('download.noDir'))
            return
        }
        setIsStarting(true)
        try {
            for (const video of playlistInfo.videos) {
                if (video.url) {
                    await StartDownload({
                        url: video.url,
                        outputDir,
                        quality,
                        videoInfo: video,
                    } as any)
                }
            }
            setUrl('')
            setVideoInfo(null)
            setPlaylistInfo(null)
        } catch (e: any) {
            showToast(t('toast.downloadStartFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsStarting(false)
        }
    }

    const handleSelectFolder = async () => {
        const dir = await SelectFolder()
        if (dir) setOutputDir(dir)
    }

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter') handleGetInfo()
    }

    return (
        <div className="app-root">
            {/* Header */}
            <header className="app-header">
                <div className="header-left">
                    <span className="app-logo">
                        <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M23.495 6.205a3.007 3.007 0 0 0-2.088-2.088c-1.87-.501-9.396-.501-9.396-.501s-7.507-.01-9.396.501A3.007 3.007 0 0 0 .527 6.205a31.247 31.247 0 0 0-.522 5.805 31.247 31.247 0 0 0 .522 5.783 3.007 3.007 0 0 0 2.088 2.088c1.868.502 9.396.502 9.396.502s7.506 0 9.396-.502a3.007 3.007 0 0 0 2.088-2.088 31.247 31.247 0 0 0 .5-5.783 31.247 31.247 0 0 0-.5-5.805zM9.609 15.601V8.408l6.264 3.602z"/>
                        </svg>
                    </span>
                    <h1 className="app-title">{t('app.title')}</h1>
                    {ytdlp && (
                        <span className={`ytdlp-badge ${ytdlp.available ? 'ytdlp-ok' : 'ytdlp-missing'}`}>
                            {ytdlp.available
                                ? t('ytdlp.version', {version: ytdlp.version})
                                : t('ytdlp.notFound')}
                        </span>
                    )}
                </div>
                <div className="header-right">
                    <button
                        className="btn-ghost btn-sm"
                        onClick={() => setLang(lang === 'zh-CN' ? 'en-US' : 'zh-CN')}
                    >
                        {lang === 'zh-CN' ? t('lang.en') : t('lang.zh')}
                    </button>
                    <button
                        className="btn-ghost btn-sm"
                        onClick={() => setTheme(t => t === 'dark' ? 'light' : 'dark')}
                        title={theme === 'dark' ? t('app.theme.light') : t('app.theme.dark')}
                    >
                        {theme === 'dark' ? '☀' : '☽'}
                    </button>
                </div>
            </header>

            {/* Main content */}
            <main className="app-main">
                {/* URL input */}
                <div className="url-section">
                    <div className="url-row">
                        <input
                            className="url-input"
                            type="text"
                            value={url}
                            onChange={e => setUrl(e.target.value)}
                            onKeyDown={handleKeyDown}
                            placeholder={t('url.placeholder')}
                            disabled={isGettingInfo}
                        />
                        <button
                            className="btn-primary"
                            onClick={handleGetInfo}
                            disabled={isGettingInfo || !url.trim()}
                        >
                            {isGettingInfo ? t('url.gettingInfo') : t('url.getInfo')}
                        </button>
                    </div>

                    {/* Video info preview */}
                    {videoInfo && (
                        <div className="video-card">
                            {videoInfo.thumbnail && (
                                <img
                                    src={videoInfo.thumbnail}
                                    alt={videoInfo.title}
                                    className="video-thumb"
                                    onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
                                />
                            )}
                            <div className="video-meta">
                                <div className="video-title">{videoInfo.title}</div>
                                <div className="video-details">
                                    {videoInfo.duration > 0 && (
                                        <span>{t('video.duration')}: {formatDuration(videoInfo.duration)}</span>
                                    )}
                                    {videoInfo.uploader && (
                                        <span>{t('video.uploader')}: {videoInfo.uploader}</span>
                                    )}
                                    {videoInfo.platform && (
                                        <span>{t('video.platform')}: {videoInfo.platform}</span>
                                    )}
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Playlist info preview */}
                    {playlistInfo && (
                        <div className="video-card playlist-card">
                            <div className="playlist-icon">
                                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                                    <line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/>
                                    <line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/>
                                    <line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>
                                </svg>
                            </div>
                            <div className="video-meta">
                                <div className="video-title">
                                    {t('playlist.detected')}{playlistInfo.title ? `: ${playlistInfo.title}` : ''}
                                </div>
                                <div className="video-details">
                                    <span>{t('playlist.videoCount', {count: String(playlistInfo.count)})}</span>
                                    {playlistInfo.uploader && (
                                        <span>{t('playlist.uploader')}: {playlistInfo.uploader}</span>
                                    )}
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Download options */}
                    <div className="options-row">
                        <div className="option-group">
                            <label className="option-label">{t('quality.label')}</label>
                            <select
                                className="select-input"
                                value={quality}
                                onChange={e => setQuality(e.target.value)}
                            >
                                {QUALITY_OPTIONS.map(q => (
                                    <option key={q} value={q}>
                                        {t(`quality.${q}` as any)}
                                    </option>
                                ))}
                            </select>
                        </div>
                        <div className="option-group flex-1">
                            <label className="option-label">{t('outputDir.label')}</label>
                            <div className="dir-row">
                                <input
                                    className="dir-input"
                                    type="text"
                                    value={outputDir}
                                    onChange={e => setOutputDir(e.target.value)}
                                    placeholder={t('outputDir.placeholder')}
                                />
                                <button className="btn-secondary" onClick={handleSelectFolder}>
                                    {t('outputDir.browse')}
                                </button>
                            </div>
                        </div>
                        <button
                            className="btn-primary download-btn"
                            onClick={handleDownload}
                            disabled={isStarting || !url.trim() || !outputDir}
                        >
                            {isStarting ? t('download.downloading') : t('download.start')}
                        </button>
                        {playlistInfo && playlistInfo.count > 0 && (
                            <button
                                className="btn-primary download-btn"
                                onClick={handleDownloadAll}
                                disabled={isStarting || !outputDir}
                            >
                                {isStarting ? t('playlist.startingAll') : t('playlist.downloadAll')}
                            </button>
                        )}
                    </div>
                </div>

                {/* Downloads list */}
                <DownloadList downloads={downloads} onUpdate={setDownloads} />
            </main>

            {/* Toast */}
            {toast && <div className="toast">{toast}</div>}
        </div>
    )
}

export default App