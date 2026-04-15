import {useState, useEffect, useCallback, useRef} from 'react'
import {CheckYtDlp, UpdateYtDlp, GetVideoInfo, GetPlaylistInfo, GetFormats, SelectFolder, StartDownload, GetDownloads, GetSettings, IsFirstRun, NeedsCookieConfig, SaveSettings, ResetSettings, CheckForUpdate, OpenReleasePage} from './lib/backend'
import {EventsOn} from './lib/runtime'
import {YtDlpStatus, VideoInfo, PlaylistInfo, FormatInfo, DownloadTask, Settings, DownloadOptions, SubtitleLang} from './types'
import {useI18n} from './i18n/context'
import DownloadList from './components/DownloadList'
import SettingsDialog from './components/SettingsDialog'
import SetupWizard from './components/SetupWizard'
import UpdateDialog from './components/UpdateDialog'
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

function parseResolutionHeight(res: string): number {
    const m = res.match(/(\d+)p/)
    if (m) return parseInt(m[1], 10)
    const m2 = res.match(/(\d+)x(\d+)/)
    if (m2) return parseInt(m2[2], 10)
    return 0
}

function formatOptionLabel(format: FormatInfo['formats'][number]): string {
    const parts: string[] = []
    // Type badge
    if (format.hasVideo && format.hasAudio) parts.push('[V+A]')
    else if (format.hasVideo) parts.push('[V]')
    else if (format.hasAudio) parts.push('[A]')
    // Resolution or audio indicator
    if (format.resolution && format.resolution !== 'audio only') {
        parts.push(format.resolution)
    } else if (format.hasAudio && !format.hasVideo) {
        parts.push('audio')
    }
    // FPS
    if (format.fps && format.fps > 0 && format.hasVideo) {
        parts.push(`${format.fps}fps`)
    }
    // Container
    if (format.ext) parts.push(format.ext)
    // Codec info
    const codecs: string[] = []
    if (format.vcodec && format.vcodec !== 'none') codecs.push(format.vcodec.split('.')[0])
    if (format.acodec && format.acodec !== 'none') codecs.push(format.acodec.split('.')[0])
    if (codecs.length > 0) parts.push(codecs.join('+'))
    // File size
    if (format.filesize) parts.push(formatFileSize(format.filesize))
    // Note
    if (format.note) parts.push(`(${format.note})`)
    return parts.join(' | ')
}

function sortFormats(formats: FormatInfo['formats'][number][]): FormatInfo['formats'][number][] {
    return [...formats].sort((a, b) => {
        // First: video+audio > video > audio
        const aScore = (a.hasVideo ? 2 : 0) + (a.hasAudio ? 1 : 0)
        const bScore = (b.hasVideo ? 2 : 0) + (b.hasAudio ? 1 : 0)
        if (aScore !== bScore) return bScore - aScore
        // Then: higher resolution first
        const aHeight = parseResolutionHeight(a.resolution || '')
        const bHeight = parseResolutionHeight(b.resolution || '')
        if (aHeight !== bHeight) return bHeight - aHeight
        // Then: higher bitrate/filesize first
        if ((b.filesize || 0) !== (a.filesize || 0)) return (b.filesize || 0) - (a.filesize || 0)
        return (b.tbr || 0) - (a.tbr || 0)
    })
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
    const [selectedPlaylistItems, setSelectedPlaylistItems] = useState<Set<number>>(new Set())
    const [quality, setQuality] = useState('best')
    const [outputDir, setOutputDir] = useState('')
    const [isGettingInfo, setIsGettingInfo] = useState(false)
    const [isStarting, setIsStarting] = useState(false)
    const [isUpdatingYtDlp, setIsUpdatingYtDlp] = useState(false)
    const [downloads, setDownloads] = useState<DownloadTask[]>([])
    const [currentSettings, setCurrentSettings] = useState<Settings | null>(null)
    const [toast, setToast] = useState<string | null>(null)
    const [showSettings, setShowSettings] = useState(false)
    const [showSetupWizard, setShowSetupWizard] = useState(false)
    const [notificationsEnabled, setNotificationsEnabled] = useState(false)
    const [consoleLogs, setConsoleLogs] = useState<string[]>([])
    const [showConsole, setShowConsole] = useState(false)
    const consoleEndRef = useRef<HTMLDivElement>(null)

    // Update dialog state
    const [showUpdateDialog, setShowUpdateDialog] = useState(false)
    const [updateInfo, setUpdateInfo] = useState<{
        hasUpdate: boolean
        currentVersion: string
        latestVersion: string
        releaseName: string
        releaseBody: string
        htmlUrl: string
        publishedAt: string
    } | null>(null)
    const [updateLoading, setUpdateLoading] = useState(false)
    const [updateError, setUpdateError] = useState<string | null>(null)

    const handleCheckUpdate = useCallback(async () => {
        setUpdateLoading(true)
        setUpdateError(null)
        setShowUpdateDialog(true)
        try {
            const info = await CheckForUpdate()
            setUpdateInfo(info)
        } catch (e: any) {
            console.error('Failed to check for updates:', e)
            setUpdateError(e?.message || 'Failed to connect to update server. Please try again.')
        } finally {
            setUpdateLoading(false)
        }
    }, [])

    const handleOpenReleasePage = useCallback(async () => {
        try {
            await OpenReleasePage()
        } catch (e) {
            console.error('Failed to open release page:', e)
        }
    }, [])

    // Per-download options (override global settings)
    const [dlOptSaveThumbnail, setDlOptSaveThumbnail] = useState(false)
    const [dlOptSaveDescription, setDlOptSaveDescription] = useState(false)
    const [dlOptEmbedChapters, setDlOptEmbedChapters] = useState(false)
    const [dlOptWriteSubtitles, setDlOptWriteSubtitles] = useState(false)
    const [dlOptEmbedSubtitles, setDlOptEmbedSubtitles] = useState(false)
    const [dlOptSponsorBlock, setDlOptSponsorBlock] = useState(false)
    const [selectedSubtitleLangs, setSelectedSubtitleLangs] = useState<Set<string>>(new Set())

    const showToast = useCallback((msg: string) => {
        setToast(msg)
        setTimeout(() => setToast(null), 3000)
    }, [])

    const applySettingsToUI = useCallback((settings: Settings) => {
        setCurrentSettings(settings)
        if (settings.outputDir) setOutputDir(settings.outputDir)
        if (settings.quality) setQuality(settings.quality)
        if (settings.theme) setTheme(settings.theme as 'dark' | 'light')
        if (settings.language) setLang(settings.language as any)
        setNotificationsEnabled(settings.notifications || false)
    }, [setLang])

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
        
        // Check if first run or needs cookie configuration
        IsFirstRun().then((firstRun: boolean) => {
            if (firstRun) {
                setShowSetupWizard(true)
                return
            }
            // Check if cookies need to be configured
            NeedsCookieConfig().then((needsCookie: boolean) => {
                if (needsCookie) {
                    setShowSetupWizard(true)
                }
            }).catch(() => {})
        }).catch(() => {
            // If check fails, show wizard anyway
            setShowSetupWizard(true)
        })
        
        GetSettings().then(s => {
            applySettingsToUI(s)
            // Initialize per-download options from global settings
            setDlOptSaveThumbnail(s.saveThumbnail || false)
            setDlOptSaveDescription(s.saveDescription || false)
            setDlOptEmbedChapters(s.embedChapters || false)
            setDlOptWriteSubtitles(s.writeSubtitles || false)
            setDlOptEmbedSubtitles(s.embedSubtitles || false)
            setDlOptSponsorBlock(s.sponsorBlock || false)
            // Request notification permission if enabled
            if (s.notifications && 'Notification' in window && Notification.permission === 'default') {
                Notification.requestPermission()
            }
        }).catch(() => {})
        GetDownloads().then(tasks => { if (tasks) setDownloads(tasks) }).catch(() => {})
    }, [applySettingsToUI, setLang])

    const handleQuickLanguageToggle = async () => {
        const nextLanguage = lang === 'zh-CN' ? 'en-US' : 'zh-CN'
        setLang(nextLanguage)
        setCurrentSettings(prev => prev ? {...prev, language: nextLanguage} : prev)
        if (currentSettings) {
            try {
                await SaveSettings({...currentSettings, language: nextLanguage})
            } catch (e) {
                console.error('Failed to persist language:', e)
            }
        }
    }

    const handleQuickThemeToggle = async () => {
        const nextTheme = theme === 'dark' ? 'light' : 'dark'
        setTheme(nextTheme)
        setCurrentSettings(prev => prev ? {...prev, theme: nextTheme} : prev)
        if (currentSettings) {
            try {
                await SaveSettings({...currentSettings, theme: nextTheme})
            } catch (e) {
                console.error('Failed to persist theme:', e)
            }
        }
    }

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

    // Auto check for updates on startup (with delay)
    useEffect(() => {
        const timer = setTimeout(() => {
            handleCheckUpdate()
        }, 3000) // Delay 3 seconds to let app fully load
        return () => clearTimeout(timer)
    }, [handleCheckUpdate])

    const dismissUpdateBanner = () => {
        setShowUpdateDialog(false)
    }



    const hasCustomFormatSelection = !!selectedFormat || !!selectedVideoFormat || !!selectedAudioFormat
    const videoOnlyFormats = formatInfo?.formats.filter(f => f.hasVideo && !f.hasAudio) || []
    const audioOnlyFormats = formatInfo?.formats.filter(f => f.hasAudio && !f.hasVideo) || []
    const collectionKind = playlistInfo?.kind === 'channel' ? 'channel' : 'playlist'
    const combineVideoFormats = (videoOnlyFormats.length > 0 ? videoOnlyFormats : formatInfo?.formats.filter(f => f.hasVideo) || [])
        .sort((a, b) => {
            const aH = parseResolutionHeight(a.resolution || '')
            const bH = parseResolutionHeight(b.resolution || '')
            if (aH !== bH) return bH - aH
            return (b.filesize || 0) - (a.filesize || 0)
        })
    const combineAudioFormats = (audioOnlyFormats.length > 0 ? audioOnlyFormats : formatInfo?.formats.filter(f => f.hasAudio) || [])
        .sort((a, b) => (b.tbr || b.filesize || 0) - (a.tbr || a.filesize || 0))

    const resolveDownloadQuality = () => {
        if (formatMode === 'single' && selectedFormat) return `f:${selectedFormat}`
        if (formatMode === 'combine') {
            if (selectedVideoFormat && selectedAudioFormat) return `f:${selectedVideoFormat}+${selectedAudioFormat}`
            if (selectedVideoFormat) return `f:${selectedVideoFormat}`
            if (selectedAudioFormat) return `f:${selectedAudioFormat}`
        }
        return quality
    }

    const buildDownloadOptions = (): DownloadOptions | undefined => {
        const langs = Array.from(selectedSubtitleLangs).join(',')
        return {
            saveThumbnail: dlOptSaveThumbnail,
            saveDescription: dlOptSaveDescription,
            embedChapters: dlOptEmbedChapters,
            writeSubtitles: dlOptWriteSubtitles,
            embedSubtitles: dlOptEmbedSubtitles,
            sponsorBlock: dlOptSponsorBlock,
            subtitleLangs: langs || '',
        } as DownloadOptions
    }

    const handleSelectBestQuality = () => {
        if (!formatInfo) return
        const sorted = sortFormats(formatInfo.formats.filter(f => f.hasVideo && f.hasAudio))
        if (sorted.length > 0) {
            setFormatMode('single')
            setSelectedFormat(sorted[0].formatId)
            setSelectedVideoFormat('')
            setSelectedAudioFormat('')
        }
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
        setSelectedPlaylistItems(new Set())
        setSelectedSubtitleLangs(new Set())
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
                    setSelectedPlaylistItems(new Set(plist.videos.map((_: any, i: number) => i)))
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
                options: buildDownloadOptions(),
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
            for (let i = 0; i < playlistInfo.videos.length; i++) {
                if (!selectedPlaylistItems.has(i)) continue
                const video = playlistInfo.videos[i]
                if (video.url) {
                    await StartDownload({
                        url: video.url,
                        outputDir,
                        quality: downloadQuality,
                        videoInfo: video,
                        options: buildDownloadOptions(),
                    } as any)
                }
            }
            setUrl('')
            setVideoInfo(null)
            setPlaylistInfo(null)
            setSelectedPlaylistItems(new Set())
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
                        onClick={handleQuickLanguageToggle}
                    >
                        {lang === 'zh-CN' ? 'EN' : '中'}
                    </button>
                    <button
                        className="btn-ghost btn-sm"
                        onClick={handleQuickThemeToggle}
                        title={theme === 'dark' ? t('app.theme.light') : t('app.theme.dark')}
                    >
                        {theme === 'dark' ? '☀' : '☽'}
                    </button>
                    <button
                        className="btn-ghost btn-sm"
                        onClick={() => setShowSettings(true)}
                        title={t('settings.title')}
                    >
                        ⚙
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
                        <>
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
                                    {t(`collection.${collectionKind}.detected` as any)}{playlistInfo.title ? `: ${playlistInfo.title}` : ''}
                                </div>
                                <div className="video-details">
                                    <span>{t(`collection.${collectionKind}.count` as any, {count: String(playlistInfo.count)})}</span>
                                    {playlistInfo.uploader && (
                                        <span>{t('playlist.uploader')}: {playlistInfo.uploader}</span>
                                    )}
                                    <span>{t('collection.selected', {count: String(selectedPlaylistItems.size)})}</span>
                                </div>
                            </div>
                        </div>
                        {/* Playlist item selector */}
                        <div className="playlist-selector">
                            <div className="playlist-selector-header">
                                <button
                                    className="btn-ghost btn-sm"
                                    onClick={() => setSelectedPlaylistItems(new Set(playlistInfo.videos.map((_: any, i: number) => i)))}
                                >
                                    {t('collection.selectAll')}
                                </button>
                                <button
                                    className="btn-ghost btn-sm"
                                    onClick={() => setSelectedPlaylistItems(new Set())}
                                >
                                    {t('collection.selectNone')}
                                </button>
                            </div>
                            <div className="playlist-selector-list">
                                {playlistInfo.videos.map((video, idx) => (
                                    <label key={idx} className="playlist-selector-item">
                                        <input
                                            type="checkbox"
                                            className="setting-checkbox"
                                            checked={selectedPlaylistItems.has(idx)}
                                            onChange={e => {
                                                const next = new Set(selectedPlaylistItems)
                                                if (e.target.checked) next.add(idx)
                                                else next.delete(idx)
                                                setSelectedPlaylistItems(next)
                                            }}
                                        />
                                        <span className="playlist-selector-idx">{idx + 1}</span>
                                        <span className="playlist-selector-title">{video.title || video.url || video.id}</span>
                                        {video.duration > 0 && (
                                            <span className="playlist-selector-duration">{formatDuration(video.duration)}</span>
                                        )}
                                    </label>
                                ))}
                            </div>
                        </div>
                        </>
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
                        {videoInfo && formatInfo && (
                            <div className="option-group">
                                <label className="option-label">{t('format.label')}</label>
                                <div className="format-list-container">
                                    <div className="format-list-header">
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
                                        {formatMode === 'single' && (
                                            <button className="btn-best-quality" onClick={handleSelectBestQuality}>
                                                {t('format.bestQuality')}
                                            </button>
                                        )}
                                    </div>
                                    {formatMode === 'single' ? (
                                        <div className="format-list">
                                            <label className={`format-list-item${!selectedFormat ? ' selected' : ''}`}>
                                                <input
                                                    type="radio"
                                                    name="format-single"
                                                    checked={!selectedFormat}
                                                    onChange={() => {
                                                        setSelectedFormat('')
                                                        setSelectedVideoFormat('')
                                                        setSelectedAudioFormat('')
                                                    }}
                                                />
                                                {t('format.usePreset')}
                                            </label>
                                            {sortFormats(formatInfo.formats.filter(f => f.hasVideo || f.hasAudio))
                                                .map(f => (
                                                    <label key={f.formatId} className={`format-list-item${selectedFormat === f.formatId ? ' selected' : ''}`}>
                                                        <input
                                                            type="radio"
                                                            name="format-single"
                                                            checked={selectedFormat === f.formatId}
                                                            onChange={() => {
                                                                setSelectedFormat(f.formatId)
                                                                setSelectedVideoFormat('')
                                                                setSelectedAudioFormat('')
                                                            }}
                                                        />
                                                        {formatOptionLabel(f)}
                                                    </label>
                                                ))}
                                        </div>
                                    ) : (
                                        <div className="format-combine-grid">
                                            <div className="format-combine-group">
                                                <span className="format-sub-label">{t('format.video')}</span>
                                                <div className="format-list">
                                                    <label className={`format-list-item${!selectedVideoFormat ? ' selected' : ''}`}>
                                                        <input type="radio" name="format-video" checked={!selectedVideoFormat} onChange={() => setSelectedVideoFormat('')} />
                                                        {t('format.selectVideo')}
                                                    </label>
                                                    {combineVideoFormats.map(f => (
                                                        <label key={f.formatId} className={`format-list-item${selectedVideoFormat === f.formatId ? ' selected' : ''}`}>
                                                            <input type="radio" name="format-video" checked={selectedVideoFormat === f.formatId} onChange={() => setSelectedVideoFormat(f.formatId)} />
                                                            {formatOptionLabel(f)}
                                                        </label>
                                                    ))}
                                                </div>
                                            </div>
                                            <div className="format-combine-group">
                                                <span className="format-sub-label">{t('format.audio')}</span>
                                                <div className="format-list">
                                                    <label className={`format-list-item${!selectedAudioFormat ? ' selected' : ''}`}>
                                                        <input type="radio" name="format-audio" checked={!selectedAudioFormat} onChange={() => setSelectedAudioFormat('')} />
                                                        {t('format.selectAudio')}
                                                    </label>
                                                    {combineAudioFormats.map(f => (
                                                        <label key={f.formatId} className={`format-list-item${selectedAudioFormat === f.formatId ? ' selected' : ''}`}>
                                                            <input type="radio" name="format-audio" checked={selectedAudioFormat === f.formatId} onChange={() => setSelectedAudioFormat(f.formatId)} />
                                                            {formatOptionLabel(f)}
                                                        </label>
                                                    ))}
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                </div>
                            </div>
                        )}
                        {videoInfo && !formatInfo && (
                            <div className="option-group">
                                <label className="option-label">{t('format.label')}</label>
                                <div className="format-row">
                                    {isGettingFormats ? (
                                        <span className="format-loading">{t('format.loading')}</span>
                                    ) : (
                                        <button
                                            className="btn-secondary"
                                            onClick={handleGetFormats}
                                            disabled={isGettingFormats}
                                        >
                                            {isGettingFormats ? t('format.loading') : t('format.detect')}
                                        </button>
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
                    </div>

                    {/* Per-download options panel (shown after video info is fetched) */}
                    {videoInfo && (
                        <div className="download-options-panel">
                            <div className="download-options-title">⚙ {t('downloadOpt.title')}</div>
                            <div className="download-options-grid">
                                <label className="download-opt-item">
                                    <input type="checkbox" checked={dlOptSaveThumbnail} onChange={e => setDlOptSaveThumbnail(e.target.checked)} />
                                    {t('downloadOpt.saveThumbnail')}
                                </label>
                                <label className="download-opt-item">
                                    <input type="checkbox" checked={dlOptSaveDescription} onChange={e => setDlOptSaveDescription(e.target.checked)} />
                                    {t('downloadOpt.saveDescription')}
                                </label>
                                <label className="download-opt-item">
                                    <input type="checkbox" checked={dlOptEmbedChapters} onChange={e => setDlOptEmbedChapters(e.target.checked)} />
                                    {t('downloadOpt.embedChapters')}
                                </label>
                                <label className="download-opt-item">
                                    <input type="checkbox" checked={dlOptWriteSubtitles} onChange={e => setDlOptWriteSubtitles(e.target.checked)} />
                                    {t('downloadOpt.writeSubtitles')}
                                </label>
                                {dlOptWriteSubtitles && (
                                    <label className="download-opt-item">
                                        <input type="checkbox" checked={dlOptEmbedSubtitles} onChange={e => setDlOptEmbedSubtitles(e.target.checked)} />
                                        {t('downloadOpt.embedSubtitles')}
                                    </label>
                                )}
                                <label className="download-opt-item">
                                    <input type="checkbox" checked={dlOptSponsorBlock} onChange={e => setDlOptSponsorBlock(e.target.checked)} />
                                    {t('downloadOpt.sponsorBlock')}
                                    <span className="sponsorblock-tooltip">
                                        <span className="tooltip-icon">?</span>
                                        <span className="tooltip-text">{t('downloadOpt.sponsorBlockDesc')}</span>
                                    </span>
                                </label>
                            </div>
                            {/* Subtitle language picker */}
                            {dlOptWriteSubtitles && (
                                <div className="subtitle-picker">
                                    <div className="subtitle-picker-label">{t('downloadOpt.subtitleLangs')}</div>
                                    {videoInfo.subtitles && videoInfo.subtitles.length > 0 ? (
                                        <div className="subtitle-picker-list">
                                            {videoInfo.subtitles.map(sub => (
                                                <label key={sub.code} className="subtitle-picker-item">
                                                    <input
                                                        type="checkbox"
                                                        checked={selectedSubtitleLangs.has(sub.code)}
                                                        onChange={e => {
                                                            const next = new Set(selectedSubtitleLangs)
                                                            if (e.target.checked) next.add(sub.code)
                                                            else next.delete(sub.code)
                                                            setSelectedSubtitleLangs(next)
                                                        }}
                                                    />
                                                    {sub.name || sub.code}
                                                    <span className="subtitle-picker-badge">
                                                        {sub.auto ? t('downloadOpt.subtitleAuto') : t('downloadOpt.subtitleManual')}
                                                    </span>
                                                </label>
                                            ))}
                                        </div>
                                    ) : (
                                        <div className="subtitle-picker-empty">{t('downloadOpt.noSubtitles')}</div>
                                    )}
                                </div>
                            )}
                        </div>
                    )}

                    <div className="options-row">
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
                                disabled={isStarting || !outputDir || selectedPlaylistItems.size === 0}
                            >
                                {isStarting ? t('playlist.startingAll') : `${t(`collection.${collectionKind}.downloadAll` as any)} (${selectedPlaylistItems.size})`}
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
                initialSettings={currentSettings}
                onClose={() => setShowSettings(false)}
                onSaved={(s) => {
                    applySettingsToUI(s)
                    // Request notification permission if newly enabled
                    if (s.notifications && 'Notification' in window && Notification.permission === 'default') {
                        Notification.requestPermission()
                    }
                }}
                onThemePreview={setTheme}
                onLanguagePreview={setLang}
            />

            {/* Setup Wizard */}
            {showSetupWizard && (
                <SetupWizard
                    onComplete={async (outputDir: string, cookiesFrom: string, cookiesFile: string, proxy: string, language: 'zh-CN' | 'en-US', theme: 'dark' | 'light') => {
                        const settings: Settings = {
                            outputDir,
                            quality: 'best',
                            language,
                            theme,
                            proxy,
                            rateLimit: '',
                            maxConcurrent: 3,
                            notifications: true,
                            saveDescription: false,
                            saveThumbnail: false,
                            writeSubtitles: false,
                            subtitleLangs: '',
                            embedSubtitles: false,
                            embedChapters: false,
                            sponsorBlock: false,
                            filenameTemplate: '',
                            mergeOutputFormat: '',
                            audioFormat: '',
                            cookiesFrom,
                            cookiesFile,
                        }
                        await SaveSettings(settings)
                        setCurrentSettings(settings)
                        setOutputDir(outputDir)
                        setTheme(theme)
                        setLang(language)
                        setShowSetupWizard(false)
                    }}
                />
            )}

            {/* Update Dialog */}
            <UpdateDialog
                open={showUpdateDialog}
                updateInfo={updateInfo}
                loading={updateLoading}
                error={updateError}
                onClose={() => setShowUpdateDialog(false)}
                onOpenReleasePage={handleOpenReleasePage}
                onCheckUpdate={handleCheckUpdate}
            />
        </div>
    )
}

export default App