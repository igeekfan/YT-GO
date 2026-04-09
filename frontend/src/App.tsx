import {useState, useEffect, useCallback, useRef} from 'react'
import {CheckYtDlp, UpdateYtDlp, GetVideoInfo, GetPlaylistInfo, GetFormats, SelectFolder, StartDownload, GetDownloads, GetSettings} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {YtDlpStatus, VideoInfo, PlaylistInfo, FormatInfo, DownloadTask, Settings} from './types'
import {useI18n} from './i18n/context'
import DownloadList from './components/DownloadList'
import SettingsDialog from './components/SettingsDialog'
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

function formatFileSize(bytes: number): string {
    if (!bytes) return ''
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
    return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function formatOptionLabel(format: FormatInfo['formats'][number]): string {
    const primary = format.resolution || (format.hasAudio && !format.hasVideo ? 'audio' : format.formatId)
    const codec = [format.ext, format.note ? `(${format.note})` : '', format.filesize ? formatFileSize(format.filesize) : '']
        .filter(Boolean)
        .join(' ')
    return `${primary} | ${codec}`
}

function getConsoleLogType(line: string): 'error' | 'warning' | 'command' | 'info' {
    const normalized = line.toLowerCase()
    if (normalized.includes(' failed:') || normalized.includes('error:')) return 'error'
    if (normalized.includes('warning:')) return 'warning'
    if (normalized.includes(' exec: ')) return 'command'
    return 'info'
}

function App() {
    const {t, lang, setLang} = useI18n()
    const [theme, setTheme] = useState<'dark' | 'light'>(() =>
        (localStorage.getItem('YT-GOto-theme') as 'dark' | 'light') || 'dark'
    )
    const [ytdlp, setYtdlp] = useState<YtDlpStatus | null>(null)
    const [url, setUrl] = useState('')
    const [videoInfo, setVideoInfo] = useState<VideoInfo | null>(null)
    const [playlistInfo, setPlaylistInfo] = useState<PlaylistInfo | null>(null)
    const [formatInfo, setFormatInfo] = useState<FormatInfo | null>(null)
    const [formatMode, setFormatMode] = useState<'single' | 'combine'>('single')
    const [selectedFormat, setSelectedFormat] = useState('')
    const [selectedVideoFormat, setSelectedVideoFormat] = useState('')
    const [selectedAudioFormat, setSelectedAudioFormat] = useState('')
    const [isGettingFormats, setIsGettingFormats] = useState(false)
    const [quality, setQuality] = useState('best')
    const [outputDir, setOutputDir] = useState('')
    const [isGettingInfo, setIsGettingInfo] = useState(false)
    const [isStarting, setIsStarting] = useState(false)
    const [isUpdatingYtDlp, setIsUpdatingYtDlp] = useState(false)
    const [downloads, setDownloads] = useState<DownloadTask[]>([])
    const [toast, setToast] = useState<string | null>(null)
    const [showSettings, setShowSettings] = useState(false)
    const [notificationsEnabled, setNotificationsEnabled] = useState(false)
    const [consoleLogs, setConsoleLogs] = useState<string[]>([])
    const [showConsole, setShowConsole] = useState(false)
    const consoleEndRef = useRef<HTMLDivElement>(null)

    const showToast = useCallback((msg: string) => {
        setToast(msg)
        setTimeout(() => setToast(null), 3000)
    }, [])

    // Request notification permission and send completion notification
    const sendNotification = useCallback((title: string, body: string) => {
        if (!notificationsEnabled) return
        if ('Notification' in window) {
            if (Notification.permission === 'granted') {
                new Notification(title, { body })
            } else if (Notification.permission !== 'denied') {
                Notification.requestPermission().then(permission => {
                    if (permission === 'granted') {
                        new Notification(title, { body })
                    }
                })
            }
        }
    }, [notificationsEnabled])

    const handleUpdateYtDlp = async () => {
        setIsUpdatingYtDlp(true)
        try {
            const result = await UpdateYtDlp()
            showToast(t('ytdlp.updateSuccess'))
            // Re-check version after update
            const status = await CheckYtDlp()
            setYtdlp(status)
        } catch (e: any) {
            showToast(t('ytdlp.updateFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsUpdatingYtDlp(false)
        }
    }

    useEffect(() => {
        document.documentElement.setAttribute('data-theme', theme)
        localStorage.setItem('YT-GOto-theme', theme)
    }, [theme])

    useEffect(() => {
        CheckYtDlp().then(setYtdlp).catch(() => setYtdlp({available: false, version: '', path: ''}))
        GetSettings().then(s => {
            if (s.outputDir) setOutputDir(s.outputDir)
            if (s.quality) setQuality(s.quality)
            if (s.theme) setTheme(s.theme as 'dark' | 'light')
            if (s.language) setLang(s.language as any)
            setNotificationsEnabled(s.notifications || false)
            // Request notification permission if enabled
            if (s.notifications && 'Notification' in window && Notification.permission === 'default') {
                Notification.requestPermission()
            }
        }).catch(() => {})
        GetDownloads().then(tasks => { if (tasks) setDownloads(tasks) }).catch(() => {})
    }, [])

    // Listen for download updates and send notifications
    useEffect(() => {
        const off = EventsOn('download:update', (task: DownloadTask) => {
            setDownloads(prev => {
                const idx = prev.findIndex(d => d.id === task.id)
                const wasCompleted = idx >= 0 && prev[idx].status === 'completed'
                const nowCompleted = task.status === 'completed'

                // Send notification if just completed
                if (!wasCompleted && nowCompleted) {
                    sendNotification(t('notification.downloadComplete'), task.title || task.url)
                }

                if (idx >= 0) {
                    const next = [...prev]
                    next[idx] = task
                    return next
                }
                return [task, ...prev]
            })
        })
        return () => { if (typeof off === 'function') off() }
    }, [sendNotification, t])

    useEffect(() => {
        const off = EventsOn('download:remove', (taskId: string) => {
            setDownloads(prev => prev.filter(d => d.id !== taskId))
        })
        return () => { if (typeof off === 'function') off() }
    }, [])

    // Listen for app:log events (console output from backend)
    useEffect(() => {
        const off = EventsOn('app:log', (msg: string) => {
            const timestamp = new Date().toLocaleTimeString()
            setConsoleLogs(prev => {
                const next = [...prev, `[${timestamp}] ${msg}`]
                return next.length > 200 ? next.slice(-200) : next
            })
            setShowConsole(true)
        })
        return () => { if (typeof off === 'function') off() }
    }, [])

    useEffect(() => {
        if (showConsole && consoleEndRef.current) {
            consoleEndRef.current.scrollIntoView({behavior: 'smooth'})
        }
    }, [consoleLogs, showConsole])

    const hasCustomFormatSelection = !!selectedFormat || !!selectedVideoFormat || !!selectedAudioFormat
    const videoOnlyFormats = formatInfo?.formats.filter(f => f.hasVideo && !f.hasAudio) || []
    const audioOnlyFormats = formatInfo?.formats.filter(f => f.hasAudio && !f.hasVideo) || []
    const combineVideoFormats = (videoOnlyFormats.length > 0 ? videoOnlyFormats : formatInfo?.formats.filter(f => f.hasVideo) || [])
        .sort((a, b) => (b.filesize || 0) - (a.filesize || 0))
    const combineAudioFormats = (audioOnlyFormats.length > 0 ? audioOnlyFormats : formatInfo?.formats.filter(f => f.hasAudio) || [])
        .sort((a, b) => (b.filesize || 0) - (a.filesize || 0))

    const resolveDownloadQuality = () => {
        if (formatMode === 'single' && selectedFormat) return `f:${selectedFormat}`
        if (formatMode === 'combine') {
            if (selectedVideoFormat && selectedAudioFormat) return `f:${selectedVideoFormat}+${selectedAudioFormat}`
            if (selectedVideoFormat) return `f:${selectedVideoFormat}`
            if (selectedAudioFormat) return `f:${selectedAudioFormat}`
        }
        return quality
    }

    const handleGetInfo = async () => {
        if (!url.trim()) return
        setIsGettingInfo(true)
        setVideoInfo(null)
        setPlaylistInfo(null)
        setFormatInfo(null)
        setFormatMode('single')
        setSelectedFormat('')
        setSelectedVideoFormat('')
        setSelectedAudioFormat('')
        try {
            const info = await GetVideoInfo(url.trim())
            setVideoInfo(info)
            // Auto-fetch available formats after getting video info
            try {
                const formats = await GetFormats(url.trim())
                setFormatInfo(formats)
            } catch {
                // Ignore format fetch errors - user can still use presets
            }
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

    const handleGetFormats = async () => {
        if (!url.trim()) return
        setIsGettingFormats(true)
        try {
            const info = await GetFormats(url.trim())
            setFormatInfo(info)
            setSelectedFormat('')
            setSelectedVideoFormat('')
            setSelectedAudioFormat('')
        } catch (e: any) {
            showToast(t('toast.getFormatsFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsGettingFormats(false)
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
            const downloadQuality = resolveDownloadQuality()
            await StartDownload({
                url: url.trim(),
                outputDir,
                quality: downloadQuality,
                videoInfo: videoInfo || undefined,
            } as any)
            setUrl('')
            setVideoInfo(null)
            setPlaylistInfo(null)
            setFormatInfo(null)
            setSelectedFormat('')
            setSelectedVideoFormat('')
            setSelectedAudioFormat('')
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
            const downloadQuality = resolveDownloadQuality()
            for (const video of playlistInfo.videos) {
                if (video.url) {
                    await StartDownload({
                        url: video.url,
                        outputDir,
                        quality: downloadQuality,
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
                    {ytdlp?.available && (
                        <button
                            className="btn-ghost btn-xs ytdlp-update-btn"
                            onClick={handleUpdateYtDlp}
                            disabled={isUpdatingYtDlp}
                            title={t('ytdlp.update')}
                        >
                            {isUpdatingYtDlp ? '⏳' : '↻'}
                        </button>
                    )}
                </div>
                <div className="header-right">
                    <button
                        className="btn-ghost btn-sm"
                        onClick={() => setShowSettings(true)}
                        title={t('settings.title')}
                    >
                        ⚙
                    </button>
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
                {/* yt-dlp installation guide when not found */}
                {ytdlp && !ytdlp.available && (
                    <div className="ytdlp-install-guide">
                        <div className="install-guide-icon">⚠️</div>
                        <h3>{t('ytdlp.notFound')}</h3>
                        <p>{t('ytdlp.installGuide')}</p>
                        <div className="install-commands">
                            <div className="install-method">
                                <strong>Windows (winget):</strong>
                                <code>winget install yt-dlp</code>
                            </div>
                            <div className="install-method">
                                <strong>Windows (scoop):</strong>
                                <code>scoop install yt-dlp</code>
                            </div>
                            <div className="install-method">
                                <strong>macOS (Homebrew):</strong>
                                <code>brew install yt-dlp</code>
                            </div>
                            <div className="install-method">
                                <strong>Linux (pip):</strong>
                                <code>pip install yt-dlp</code>
                            </div>
                        </div>
                        <p className="install-note">{t('ytdlp.installNote')}</p>
                        <button className="btn-primary" onClick={() => CheckYtDlp().then(setYtdlp)}>
                            {t('ytdlp.recheck')}
                        </button>
                    </div>
                )}

                {/* URL input - only show when yt-dlp is available */}
                {ytdlp?.available && (
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
                                onChange={e => {
                                    setQuality(e.target.value)
                                    setSelectedFormat('')
                                    setSelectedVideoFormat('')
                                    setSelectedAudioFormat('')
                                }}
                                disabled={hasCustomFormatSelection}
                            >
                                {QUALITY_OPTIONS.map(q => (
                                    <option key={q} value={q}>
                                        {t(`quality.${q}` as any)}
                                    </option>
                                ))}
                            </select>
                        </div>
                        {videoInfo && (
                            <div className="option-group">
                                <label className="option-label">{t('format.label')}</label>
                                <div className="format-row">
                                    {isGettingInfo ? (
                                        <span className="format-loading">{t('format.loading')}</span>
                                    ) : !formatInfo ? (
                                        <button
                                            className="btn-secondary"
                                            onClick={handleGetFormats}
                                            disabled={isGettingFormats}
                                        >
                                            {isGettingFormats ? t('format.loading') : t('format.detect')}
                                        </button>
                                    ) : (
                                        <div className="format-stack">
                                            <select
                                                className="select-input format-mode-select"
                                                value={formatMode}
                                                onChange={e => {
                                                    const nextMode = e.target.value as 'single' | 'combine'
                                                    setFormatMode(nextMode)
                                                    setSelectedFormat('')
                                                    setSelectedVideoFormat('')
                                                    setSelectedAudioFormat('')
                                                }}
                                            >
                                                <option value="single">{t('format.mode.single')}</option>
                                                <option value="combine">{t('format.mode.combine')}</option>
                                            </select>
                                            {formatMode === 'single' ? (
                                                <select
                                                    className="select-input format-select"
                                                    value={selectedFormat}
                                                    onChange={e => {
                                                        setSelectedFormat(e.target.value)
                                                        setSelectedVideoFormat('')
                                                        setSelectedAudioFormat('')
                                                    }}
                                                >
                                                    <option value="">{t('format.usePreset')}</option>
                                                    {formatInfo.formats
                                                        .filter(f => f.hasVideo || f.hasAudio)
                                                        .sort((a, b) => {
                                                            const aScore = (a.hasVideo ? 2 : 0) + (a.hasAudio ? 1 : 0)
                                                            const bScore = (b.hasVideo ? 2 : 0) + (b.hasAudio ? 1 : 0)
                                                            if (aScore !== bScore) return bScore - aScore
                                                            return (b.filesize || 0) - (a.filesize || 0)
                                                        })
                                                        .map(f => (
                                                            <option key={f.formatId} value={f.formatId}>
                                                                {formatOptionLabel(f)}
                                                            </option>
                                                        ))}
                                                </select>
                                            ) : (
                                                <div className="format-combine-grid">
                                                    <div className="format-combine-group">
                                                        <span className="format-sub-label">{t('format.video')}</span>
                                                        <select
                                                            className="select-input format-select"
                                                            value={selectedVideoFormat}
                                                            onChange={e => setSelectedVideoFormat(e.target.value)}
                                                        >
                                                            <option value="">{t('format.selectVideo')}</option>
                                                            {combineVideoFormats.map(f => (
                                                                <option key={f.formatId} value={f.formatId}>
                                                                    {formatOptionLabel(f)}
                                                                </option>
                                                            ))}
                                                        </select>
                                                    </div>
                                                    <div className="format-combine-group">
                                                        <span className="format-sub-label">{t('format.audio')}</span>
                                                        <select
                                                            className="select-input format-select"
                                                            value={selectedAudioFormat}
                                                            onChange={e => setSelectedAudioFormat(e.target.value)}
                                                        >
                                                            <option value="">{t('format.selectAudio')}</option>
                                                            {combineAudioFormats.map(f => (
                                                                <option key={f.formatId} value={f.formatId}>
                                                                    {formatOptionLabel(f)}
                                                                </option>
                                                            ))}
                                                        </select>
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    )}
                                </div>
                            </div>
                        )}
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
                )}

                {/* Console logs */}
                {consoleLogs.length > 0 && (
                    <div className="console-panel">
                        <div className="console-header">
                            <button
                                className="btn-ghost btn-sm"
                                onClick={() => setShowConsole(!showConsole)}
                            >
                                {showConsole ? '▼' : '▶'} {t('console.title')} ({consoleLogs.length})
                            </button>
                            <button
                                className="btn-ghost btn-sm"
                                onClick={() => { setConsoleLogs([]); setShowConsole(false) }}
                            >
                                {t('console.clear')}
                            </button>
                        </div>
                        {showConsole && (
                            <pre className="console-content">
                                {consoleLogs.map((line, i) => (
                                    <div key={i} className={`log-line log-line-${getConsoleLogType(line)}`}>{line}</div>
                                ))}
                                <div ref={consoleEndRef} />
                            </pre>
                        )}
                    </div>
                )}

                {/* Downloads list */}
                <DownloadList downloads={downloads} onUpdate={setDownloads} />
            </main>

            {/* Toast */}
            {toast && <div className="toast">{toast}</div>}

            {/* Settings Dialog */}
            <SettingsDialog
                open={showSettings}
                onClose={() => setShowSettings(false)}
                onSaved={(s) => {
                    setOutputDir(s.outputDir)
                    setQuality(s.quality)
                    setTheme(s.theme as 'dark' | 'light')
                    setLang(s.language as any)
                    setNotificationsEnabled(s.notifications || false)
                    // Request notification permission if newly enabled
                    if (s.notifications && 'Notification' in window && Notification.permission === 'default') {
                        Notification.requestPermission()
                    }
                }}
            />
        </div>
    )
}

export default App