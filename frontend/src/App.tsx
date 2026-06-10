import {useState, useEffect, useCallback, useRef} from 'react'
import {CheckYtDlp, UpdateYtDlp, InstallYtDlp, GetVideoInfo, GetPlaylistInfo, GetFormats, SelectFolder, StartDownload, GetDownloads, GetSettings, IsFirstRun, NeedsCookieConfig, SaveSettings, ResetSettings, CheckForUpdate, OpenReleasePage, backendMode, fetchWebConfig, getWebConfig, getAuthToken} from './lib/backend'
import {EventsOn} from './lib/runtime'
import {YtDlpStatus, VideoInfo, PlaylistInfo, FormatInfo, DownloadTask, Settings, DownloadOptions} from './types'
import {useI18n} from './i18n/context'
import {formatDuration, formatFileSize} from './lib/formatUtils'
import DownloadList from './components/DownloadList'
import SettingsDialog from './components/SettingsDialog'
import SetupWizard from './components/SetupWizard'
import UpdateDialog from './components/UpdateDialog'
import DirBrowser from './components/DirBrowser'
import LoginPage from './components/LoginPage'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Badge} from '@/components/ui/badge'
import {Checkbox} from '@/components/ui/checkbox'
import {Label} from '@/components/ui/label'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip'
import {Separator} from '@/components/ui/separator'
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible'
import {toast} from 'sonner'
import {
    Search, X, Sun, Moon, Settings as SettingsIcon, RefreshCw,
    Play, Download, ChevronDown, ChevronRight, Info, FolderOpen
} from 'lucide-react'

function getSubtitleSelectionKey(sub: {code: string, auto?: boolean, selector?: string}): string {
    return sub.selector || `${sub.auto ? 'auto' : 'manual'}:${sub.code}`
}

function splitSelectedSubtitleLangs(subtitles: VideoInfo['subtitles'] | undefined, selectedKeys: Set<string>) {
    const manualLangs = new Set<string>()
    const autoLangs = new Set<string>()

    for (const sub of subtitles || []) {
        if (!selectedKeys.has(getSubtitleSelectionKey(sub))) continue
        if (sub.auto) autoLangs.add(sub.code)
        else manualLangs.add(sub.code)
    }

    return {
        manualLangs: Array.from(manualLangs),
        autoLangs: Array.from(autoLangs),
    }
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
    if (format.hasVideo && format.hasAudio) parts.push('[V+A]')
    else if (format.hasVideo) parts.push('[V]')
    else if (format.hasAudio) parts.push('[A]')
    if (format.resolution && format.resolution !== 'audio only') {
        parts.push(format.resolution)
    } else if (format.hasAudio && !format.hasVideo) {
        parts.push('audio')
    }
    if (format.fps && format.fps > 0 && format.hasVideo) {
        parts.push(`${format.fps}fps`)
    }
    if (format.ext) parts.push(format.ext)
    const codecs: string[] = []
    if (format.vcodec && format.vcodec !== 'none') codecs.push(format.vcodec.split('.')[0])
    if (format.acodec && format.acodec !== 'none') codecs.push(format.acodec.split('.')[0])
    if (codecs.length > 0) parts.push(codecs.join('+'))
    if (format.filesize) parts.push(formatFileSize(format.filesize))
    if (format.note) parts.push(`(${format.note})`)
    return parts.join(' | ')
}

function sortFormats(formats: FormatInfo['formats'][number][]): FormatInfo['formats'][number][] {
    return [...formats].sort((a, b) => {
        const aScore = (a.hasVideo ? 2 : 0) + (a.hasAudio ? 1 : 0)
        const bScore = (b.hasVideo ? 2 : 0) + (b.hasAudio ? 1 : 0)
        if (aScore !== bScore) return bScore - aScore
        const aHeight = parseResolutionHeight(a.resolution || '')
        const bHeight = parseResolutionHeight(b.resolution || '')
        if (aHeight !== bHeight) return bHeight - aHeight
        if ((b.filesize || 0) !== (a.filesize || 0)) return (b.filesize || 0) - (a.filesize || 0)
        return (b.tbr || 0) - (a.tbr || 0)
    })
}

function findFormatByID(formats: FormatInfo['formats'], formatId: string): FormatInfo['formats'][number] | undefined {
    return formats.find(format => format.formatId === formatId)
}

function getConsoleLogType(line: string): 'error' | 'warning' | 'command' | 'info' {
    const normalized = line.toLowerCase()
    if (normalized.includes(' failed:') || normalized.includes('error:')) return 'error'
    if (normalized.includes('warning:')) return 'warning'
    if (normalized.includes(' exec: ')) return 'command'
    return 'info'
}

function shouldTryPlaylistFallback(error: unknown): boolean {
    const message = String((error as any)?.message || error || '').toLowerCase()
    if (!message) return true
    const nonFallbackSignals = [
        'js runtime', 'deno', 'node.js', 'sign in to confirm', 'not a bot',
        'dpapi', 'cookies', 'storyboard', 'rejected the current access',
        'requires valid login cookies',
        '请安装 deno', 'node.js lts', 'youtube 拒绝了当前访问', '抖音需要有效的登录 cookies',
    ]
    return !nonFallbackSignals.some(signal => message.includes(signal))
}

function App() {
    const {t, lang, setLang} = useI18n()
    const [theme, setTheme] = useState<'dark' | 'light'>(() => {
        const saved = localStorage.getItem('YT-GOto-theme') as 'dark' | 'light' | null
        if (saved) return saved
        return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
    })
    const [ytdlp, setYtdlp] = useState<YtDlpStatus | null>(null)
    const [url, setUrl] = useState('')
    const [videoInfo, setVideoInfo] = useState<VideoInfo | null>(null)
    const [playlistInfo, setPlaylistInfo] = useState<PlaylistInfo | null>(null)
    const [formatInfo, setFormatInfo] = useState<FormatInfo | null>(null)
    const [formatMode, setFormatMode] = useState<'single' | 'combine' | 'audio-only' | 'video-only'>('single')
    const [selectedFormat, setSelectedFormat] = useState('')
    const [selectedVideoFormat, setSelectedVideoFormat] = useState('')
    const [selectedAudioFormat, setSelectedAudioFormat] = useState('')
    const [isGettingFormats, setIsGettingFormats] = useState(false)
    const [formatExpanded, setFormatExpanded] = useState(true)
    const [selectedPlaylistItems, setSelectedPlaylistItems] = useState<Set<number>>(new Set())
    const [quality, setQuality] = useState('best')
    const [outputDir, setOutputDir] = useState('')
    const [isGettingInfo, setIsGettingInfo] = useState(false)
    const [isStarting, setIsStarting] = useState(false)
    const [isUpdatingYtDlp, setIsUpdatingYtDlp] = useState(false)
    const [isInstallingYtDlp, setIsInstallingYtDlp] = useState(false)
    const [downloads, setDownloads] = useState<DownloadTask[]>([])
    const [currentSettings, setCurrentSettings] = useState<Settings | null>(null)
    const [showSettings, setShowSettings] = useState(false)
    const [showSetupWizard, setShowSetupWizard] = useState(false)
    const [notificationsEnabled, setNotificationsEnabled] = useState(false)
    const [consoleLogs, setConsoleLogs] = useState<string[]>([])
    const [showConsole, setShowConsole] = useState(false)
    const consoleEndRef = useRef<HTMLDivElement>(null)
    const [showUpdateDialog, setShowUpdateDialog] = useState(false)
    const [showDirBrowser, setShowDirBrowser] = useState(false)
    const [needsAuth, setNeedsAuth] = useState(false)
    const [isAuthenticated, setIsAuthenticated] = useState(() => !!getAuthToken())
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

    // Per-download options
    const [dlOptSaveThumbnail, setDlOptSaveThumbnail] = useState(false)
    const [dlOptSaveDescription, setDlOptSaveDescription] = useState(false)
    const [dlOptEmbedChapters, setDlOptEmbedChapters] = useState(false)
    const [dlOptWriteSubtitles, setDlOptWriteSubtitles] = useState(false)
    const [dlOptEmbedSubtitles, setDlOptEmbedSubtitles] = useState(false)
    const [dlOptSponsorBlock, setDlOptSponsorBlock] = useState(false)
    const [dlOptFilenameTemplate, setDlOptFilenameTemplate] = useState('')
    const [selectedSubtitleLangs, setSelectedSubtitleLangs] = useState<Set<string>>(new Set())
    const [subtitleSearch, setSubtitleSearch] = useState('')

    const handleCheckUpdate = useCallback(async () => {
        setUpdateLoading(true)
        setUpdateError(null)
        setShowUpdateDialog(true)
        try {
            const info = await CheckForUpdate()
            setUpdateInfo(info)
        } catch (e: any) {
            setUpdateError(e?.message || t('update.error'))
        } finally {
            setUpdateLoading(false)
        }
    }, [])

    const handleOpenReleasePage = useCallback(async () => {
        try { await OpenReleasePage() } catch (e) { console.error('Failed to open release page:', e) }
    }, [])

    const persistSettingsPatch = useCallback((patch: Partial<Settings>) => {
        setCurrentSettings(prev => {
            if (!prev) return prev
            const next = {...prev, ...patch}
            SaveSettings(next).catch(error => console.error('Failed to persist settings patch:', error))
            return next
        })
    }, [])

    const applySettingsToUI = useCallback((settings: Settings) => {
        setCurrentSettings(settings)
        if (settings.outputDir) setOutputDir(settings.outputDir)
        if (settings.quality) setQuality(settings.quality)
        if (settings.theme) setTheme(settings.theme as 'dark' | 'light')
        if (settings.language) setLang(settings.language as any)
        setNotificationsEnabled(settings.notifications || false)
    }, [setLang])

    const sendNotification = useCallback((title: string, body: string) => {
        if (!notificationsEnabled) return
        if ('Notification' in window) {
            if (Notification.permission === 'granted') new Notification(title, { body })
            else if (Notification.permission !== 'denied') {
                Notification.requestPermission().then(permission => {
                    if (permission === 'granted') new Notification(title, { body })
                })
            }
        }
    }, [notificationsEnabled])

    const handleUpdateYtDlp = async () => {
        setIsUpdatingYtDlp(true)
        try {
            await UpdateYtDlp()
            toast.success(t('ytdlp.updateSuccess'))
            const status = await CheckYtDlp()
            setYtdlp(status)
        } catch (e: any) {
            toast.error(t('ytdlp.updateFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsUpdatingYtDlp(false)
        }
    }

    const handleInstallYtDlp = async () => {
        setIsInstallingYtDlp(true)
        try {
            await InstallYtDlp()
            toast.success(t('ytdlp.installSuccess'))
            const status = await CheckYtDlp()
            setYtdlp(status)
        } catch (e: any) {
            toast.error(t('ytdlp.installFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsInstallingYtDlp(false)
        }
    }

    useEffect(() => {
        const root = document.documentElement
        if (theme === 'dark') root.classList.add('dark')
        else root.classList.remove('dark')
        localStorage.setItem('YT-GOto-theme', theme)
    }, [theme])

    useEffect(() => {
        const init = async () => {
            if (backendMode === 'web') {
                await fetchWebConfig().catch(console.error)
                const cfg = getWebConfig()
                if (cfg?.authRequired) {
                    if (!getAuthToken()) {
                        setNeedsAuth(true)
                        return
                    }
                    // Verify existing token by calling a protected endpoint.
                    try {
                        await CheckYtDlp()
                    } catch {
                        setNeedsAuth(true)
                        return
                    }
                }
            }
            setIsAuthenticated(true)
            CheckYtDlp().then(setYtdlp).catch(() => setYtdlp({available: false, version: '', path: ''}))
            if (backendMode === 'web') return
            IsFirstRun().then((firstRun: boolean) => {
                if (firstRun) { setShowSetupWizard(true); return }
                NeedsCookieConfig().then((needsCookie: boolean) => {
                    if (needsCookie) setShowSetupWizard(true)
                }).catch(() => {})
            }).catch(() => { setShowSetupWizard(true) })
        }
        init()
        GetSettings().then(s => {
            applySettingsToUI(s)
            setDlOptSaveThumbnail(s.saveThumbnail || false)
            setDlOptSaveDescription(s.saveDescription || false)
            setDlOptEmbedChapters(s.embedChapters || false)
            setDlOptWriteSubtitles(s.writeSubtitles || false)
            setDlOptEmbedSubtitles(s.embedSubtitles || false)
            setDlOptSponsorBlock(s.sponsorBlock || false)
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
            try { await SaveSettings({...currentSettings, language: nextLanguage}) }
            catch (e) { console.error('Failed to persist language:', e) }
        }
    }

    const handleQuickThemeToggle = async () => {
        const nextTheme = theme === 'dark' ? 'light' : 'dark'
        setTheme(nextTheme)
        setCurrentSettings(prev => prev ? {...prev, theme: nextTheme} : prev)
        if (currentSettings) {
            try { await SaveSettings({...currentSettings, theme: nextTheme}) }
            catch (e) { console.error('Failed to persist theme:', e) }
        }
    }

    const clearCurrentInput = useCallback(() => {
        setUrl('')
        setVideoInfo(null)
        setPlaylistInfo(null)
        setFormatInfo(null)
        setSelectedFormat('')
        setSelectedVideoFormat('')
        setSelectedAudioFormat('')
        setSelectedPlaylistItems(new Set())
        setSelectedSubtitleLangs(new Set())
        setSubtitleSearch('')
        setFormatExpanded(false)
    }, [])

    useEffect(() => {
        const off = EventsOn('download:update', (task: DownloadTask) => {
            setDownloads(prev => {
                const idx = prev.findIndex(d => d.id === task.id)
                const wasCompleted = idx >= 0 && prev[idx].status === 'completed'
                if (!wasCompleted && task.status === 'completed') {
                    sendNotification(t('notification.downloadComplete'), task.title || task.url)
                }
                if (idx >= 0) { const next = [...prev]; next[idx] = task; return next }
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
        const off = EventsOn('download:log', (data: {taskId: string; line: string}) => {
            const timestamp = new Date().toLocaleTimeString()
            setConsoleLogs(prev => {
                const next = [...prev, `[${timestamp}] [${data.taskId}] ${data.line}`]
                return next.length > 200 ? next.slice(-200) : next
            })
            setShowConsole(true)
        })
        return () => { if (typeof off === 'function') off() }
    }, [])

    useEffect(() => {
        const timer = setTimeout(() => { handleCheckUpdate() }, 3000)
        return () => clearTimeout(timer)
    }, [handleCheckUpdate])

    const videoOnlyFormats = formatInfo?.formats.filter(f => f.hasVideo && !f.hasAudio) || []
    const audioOnlyFormats = formatInfo?.formats.filter(f => f.hasAudio && !f.hasVideo) || []
    const collectionKind = playlistInfo?.kind === 'channel' ? 'channel' : 'playlist'
    const hasSeparateTrackFormats = videoOnlyFormats.length > 0 && audioOnlyFormats.length > 0
    const combineVideoFormats = videoOnlyFormats.sort((a, b) => {
        const aH = parseResolutionHeight(a.resolution || '')
        const bH = parseResolutionHeight(b.resolution || '')
        if (aH !== bH) return bH - aH
        return (b.filesize || 0) - (a.filesize || 0)
    })
    const combineAudioFormats = audioOnlyFormats.sort((a, b) => (b.tbr || b.filesize || 0) - (a.tbr || a.filesize || 0))
    const hasCustomFormatSelection = !!selectedFormat || !!selectedVideoFormat || !!selectedAudioFormat

    const resolveDownloadQuality = () => {
        if (formatMode === 'single' && selectedFormat && formatInfo) {
            const format = findFormatByID(formatInfo.formats, selectedFormat)
            if (format?.hasAudio && !format.hasVideo) return `fa:${selectedFormat}`
            if (format?.hasVideo && !format.hasAudio) return `fv:${selectedFormat}`
            return `f:${selectedFormat}`
        }
        if (formatMode === 'combine') {
            if (selectedVideoFormat && selectedAudioFormat) return `f:${selectedVideoFormat}+${selectedAudioFormat}`
            if (selectedVideoFormat) return `fv:${selectedVideoFormat}`
            if (selectedAudioFormat) return `fa:${selectedAudioFormat}`
        }
        if (formatMode === 'audio-only') {
            if (selectedAudioFormat) return `fa:${selectedAudioFormat}`
            return 'audio'
        }
        if (formatMode === 'video-only') {
            if (selectedVideoFormat) return `fv:${selectedVideoFormat}`
            return 'best'
        }
        return quality
    }

    const buildDownloadOptions = (): DownloadOptions | undefined => {
        const {manualLangs, autoLangs} = splitSelectedSubtitleLangs(videoInfo?.subtitles, selectedSubtitleLangs)
        const hasExplicitSubtitleSelection = manualLangs.length > 0 || autoLangs.length > 0
        return {
            saveThumbnail: dlOptSaveThumbnail,
            saveDescription: dlOptSaveDescription,
            embedChapters: dlOptEmbedChapters,
            writeSubtitles: dlOptWriteSubtitles,
            writeManualSubs: hasExplicitSubtitleSelection ? manualLangs.length > 0 : undefined,
            writeAutoSubs: hasExplicitSubtitleSelection ? autoLangs.length > 0 : undefined,
            embedSubtitles: dlOptEmbedSubtitles,
            sponsorBlock: dlOptSponsorBlock,
            subtitleLangs: hasExplicitSubtitleSelection ? manualLangs.join(',') : '',
            autoSubtitleLangs: hasExplicitSubtitleSelection ? autoLangs.join(',') : '',
            filenameTemplate: dlOptFilenameTemplate.trim(),
        } as DownloadOptions
    }

    const handleSelectBestQuality = () => {
        if (!formatInfo) return
        const combined = sortFormats(formatInfo.formats.filter(f => f.hasVideo && f.hasAudio))
        if (combined.length > 0) {
            setFormatMode('single'); setSelectedFormat(combined[0].formatId)
            setSelectedVideoFormat(''); setSelectedAudioFormat(''); return
        }
        const vFormats = videoOnlyFormats.sort((a, b) => {
            const aH = parseResolutionHeight(a.resolution || '')
            const bH = parseResolutionHeight(b.resolution || '')
            if (aH !== bH) return bH - aH
            return (b.filesize || 0) - (a.filesize || 0)
        })
        const aFormats = audioOnlyFormats.sort((a, b) => (b.tbr || b.filesize || 0) - (a.tbr || a.filesize || 0))
        if (vFormats.length > 0 || aFormats.length > 0) {
            setFormatMode('combine'); setSelectedFormat('')
            setSelectedVideoFormat(vFormats.length > 0 ? vFormats[0].formatId : '')
            setSelectedAudioFormat(aFormats.length > 0 ? aFormats[0].formatId : '')
        }
    }

    const handleGetInfo = async () => {
        if (!url.trim()) return
        setIsGettingInfo(true); setIsGettingFormats(false)
        setVideoInfo(null); setPlaylistInfo(null); setFormatInfo(null)
        setFormatMode('single'); setSelectedFormat(''); setSelectedVideoFormat(''); setSelectedAudioFormat('')
        setFormatExpanded(false); setSelectedPlaylistItems(new Set()); setSelectedSubtitleLangs(new Set())
        try {
            const info = await GetVideoInfo(url.trim())
            setVideoInfo(info); setFormatExpanded(true)
            setIsGettingFormats(true)
            try { const formats = await GetFormats(url.trim()); setFormatInfo(formats) }
            catch { /* Ignore format fetch errors */ }
            finally { setIsGettingFormats(false) }
        } catch (e: any) {
            if (!shouldTryPlaylistFallback(e)) {
                toast.error(t('toast.getInfoFail') + (e?.message ? `: ${e.message}` : '')); return
            }
            try {
                const plist = await GetPlaylistInfo(url.trim())
                if (plist && plist.count > 0) {
                    setPlaylistInfo(plist)
                    setSelectedPlaylistItems(new Set(plist.videos.map((_: any, i: number) => i)))
                } else {
                    toast.error(t('toast.getInfoFail') + (e?.message ? `: ${e.message}` : ''))
                }
            } catch {
                toast.error(t('toast.getInfoFail') + (e?.message ? `: ${e.message}` : ''))
            }
        } finally { setIsGettingInfo(false) }
    }

    const handleGetFormats = async () => {
        if (!url.trim()) return
        setIsGettingFormats(true); setFormatExpanded(true)
        try {
            const info = await GetFormats(url.trim())
            setFormatInfo(info); setSelectedFormat(''); setSelectedVideoFormat(''); setSelectedAudioFormat('')
        } catch (e: any) {
            toast.error(t('toast.getFormatsFail') + (e?.message ? `: ${e.message}` : ''))
        } finally { setIsGettingFormats(false) }
    }

    const handleDownload = async () => {
        if (!url.trim()) return
        if (!outputDir) { toast.error(t('download.noDir')); return }
        if (formatMode === 'combine' && !hasSeparateTrackFormats) { toast.error(t('format.combineUnavailable')); return }
        setIsStarting(true)
        try {
            const downloadQuality = resolveDownloadQuality()
            await StartDownload({
                url: url.trim(), outputDir, quality: downloadQuality,
                videoInfo: videoInfo ? {
                    url: videoInfo.url, id: videoInfo.id, title: videoInfo.title,
                    thumbnail: videoInfo.thumbnail, duration: videoInfo.duration,
                    uploader: videoInfo.uploader, platform: videoInfo.platform,
                } : undefined,
                options: buildDownloadOptions(),
            })
            toast.success(t('toast.downloadQueued'))
        } catch (e: any) {
            toast.error(t('toast.downloadStartFail') + (e?.message ? `: ${e.message}` : ''))
        } finally { setIsStarting(false) }
    }

    const handleDownloadAll = async () => {
        if (!playlistInfo || !outputDir) {
            if (!outputDir) toast.error(t('download.noDir')); return
        }
        if (formatMode === 'combine') {
            if (!hasSeparateTrackFormats) { toast.error(t('format.combineUnavailable')); return }
            if (!selectedVideoFormat && !selectedAudioFormat) { toast.error(t('toast.selectAnyTrack')); return }
        }
        setIsStarting(true)
        try {
            const downloadQuality = resolveDownloadQuality()
            let startedCount = 0
            for (let i = 0; i < playlistInfo.videos.length; i++) {
                if (!selectedPlaylistItems.has(i)) continue
                const video = playlistInfo.videos[i]
                if (video.url) {
                    await StartDownload({
                        url: video.url, outputDir, quality: downloadQuality,
                        videoInfo: video ? {
                            url: video.url, id: video.id, title: video.title,
                            thumbnail: video.thumbnail, duration: video.duration,
                            uploader: video.uploader, platform: video.platform,
                        } : undefined,
                        options: buildDownloadOptions(),
                    })
                    startedCount++
                }
            }
            if (startedCount > 0) toast.success(t('toast.downloadQueuedCount', {count: String(startedCount)}))
        } catch (e: any) {
            toast.error(t('toast.downloadStartFail') + (e?.message ? `: ${e.message}` : ''))
        } finally { setIsStarting(false) }
    }

    const handleSelectFolder = async () => {
        const dir = await SelectFolder()
        if (dir) setOutputDir(dir)
    }

    // Show login page when auth is required but not authenticated.
    if (needsAuth && !isAuthenticated) {
        return <LoginPage onAuthenticated={() => {
            setIsAuthenticated(true)
            setNeedsAuth(false)
            // Re-run initialization after authentication.
            CheckYtDlp().then(setYtdlp).catch(() => setYtdlp({available: false, version: '', path: ''}))
            GetSettings().then(s => applySettingsToUI(s)).catch(() => {})
            GetDownloads().then(tasks => { if (tasks) setDownloads(tasks) }).catch(() => {})
        }} />
    }

    return (
        <div className="min-h-screen bg-background">
            {/* Header */}
            <header className="sticky top-0 z-20 flex h-12 items-center gap-2 border-b border-primary/15 bg-background/85 backdrop-blur-xl px-4 relative after:absolute after:bottom-0 after:left-0 after:right-0 after:h-px after:bg-gradient-to-r after:from-transparent after:via-primary/25 after:to-transparent">
                <div className="flex items-center gap-2 shrink-0">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-primary to-primary/70 text-primary-foreground shadow-sm glow-primary">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                            <path d="M23.495 6.205a3.007 3.007 0 0 0-2.088-2.088c-1.87-.501-9.396-.501-9.396-.501s-7.507-.01-9.396.501A3.007 3.007 0 0 0 .527 6.205a31.247 31.247 0 0 0-.522 5.805 31.247 31.247 0 0 0 .522 5.783 3.007 3.007 0 0 0 2.088 2.088c1.868.502 9.396.502 9.396.502s7.506 0 9.396-.502a3.007 3.007 0 0 0 2.088-2.088 31.247 31.247 0 0 0 .5-5.783 31.247 31.247 0 0 0-.5-5.805zM9.609 15.601V8.408l6.264 3.602z"/>
                        </svg>
                    </div>
                    <span className="text-sm font-bold tracking-tight select-none">{t('app.title')}</span>
                </div>
                <div className="flex-1 flex items-center gap-1.5 pl-1">
                    {ytdlp && (
                        <Badge variant={ytdlp.available ? 'secondary' : 'destructive'} className="text-[11px] h-5 px-2 font-medium tracking-wide">
                            {ytdlp.available ? t('ytdlp.version', {version: ytdlp.version}) : t('ytdlp.notFound')}
                        </Badge>
                    )}
                    {ytdlp?.available && (
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleUpdateYtDlp} disabled={isUpdatingYtDlp}>
                                    <RefreshCw className={`h-3.5 w-3.5 ${isUpdatingYtDlp ? 'animate-spin' : ''}`} />
                                </Button>
                            </TooltipTrigger>
                            <TooltipContent>{t('ytdlp.update')}</TooltipContent>
                        </Tooltip>
                    )}
                </div>
                <div className="flex items-center gap-0.5 shrink-0">
                    <Button variant="ghost" size="sm" onClick={handleQuickLanguageToggle} className="text-xs h-7 px-2.5 font-medium tracking-wide">
                        {lang === 'zh-CN' ? 'EN' : '中'}
                    </Button>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleQuickThemeToggle}>
                                {theme === 'dark' ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{theme === 'dark' ? t('app.theme.light') : t('app.theme.dark')}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowSettings(true)}>
                                <SettingsIcon className="h-3.5 w-3.5" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('settings.title')}</TooltipContent>
                    </Tooltip>
                </div>
            </header>

            {/* Main workspace */}
            <main className="mx-auto max-w-3xl px-4 py-5 flex flex-col gap-4">
                {/* yt-dlp Install Guide */}
                {ytdlp && !ytdlp.available && (
                    <div className="rounded-xl border bg-card/70 backdrop-blur-sm p-5 text-center space-y-3 shadow-sm animate-fade-in-up">
                        <div className="text-4xl leading-none">⚠️</div>
                        <h3 className="text-base font-semibold">{t('ytdlp.notFound')}</h3>
                        <p className="text-xs text-muted-foreground">{t('ytdlp.installGuide')}</p>
                        <div className="rounded-md border bg-muted/40 p-2.5 text-left space-y-2">
                            <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.windowsWinget')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">winget install yt-dlp</code></div>
                            <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.windowsScoop')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">scoop install yt-dlp</code></div>
                            <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.macHomebrew')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">brew install yt-dlp</code></div>
                            <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.linuxPip')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">pip install yt-dlp</code></div>
                        </div>
                        <p className="text-[11px] text-muted-foreground">{t('ytdlp.installNote')}</p>
                        <div className="flex items-center justify-center gap-2">
                            <Button size="sm" onClick={handleInstallYtDlp} disabled={isInstallingYtDlp}>
                                {isInstallingYtDlp ? t('ytdlp.installing') : t('ytdlp.autoInstall')}
                            </Button>
                            <Button size="sm" variant="outline" onClick={() => CheckYtDlp().then(setYtdlp)}>{t('ytdlp.recheck')}</Button>
                        </div>
                    </div>
                )}

                {/* URL Input */}
                {ytdlp?.available && (
                    <div className="flex gap-2.5 items-center animate-fade-in-up">
                        <div className="relative flex-1">
                            <Input
                                type="text"
                                value={url}
                                onChange={e => setUrl(e.target.value)}
                                onKeyDown={e => { if (e.key === 'Enter') handleGetInfo() }}
                                placeholder={t('url.placeholder')}
                                disabled={isGettingInfo}
                                className="pr-8 h-10 text-sm bg-card/60 backdrop-blur-sm border-glow focus-visible:ring-primary/30 transition-all duration-200"
                            />
                            {url && (
                                <Button variant="ghost" size="icon" className="absolute right-1 top-1/2 -translate-y-1/2 h-6 w-6 text-muted-foreground hover:text-foreground" onClick={clearCurrentInput}>
                                    <X className="h-3 w-3" />
                                </Button>
                            )}
                        </div>
                        <Button onClick={handleGetInfo} disabled={isGettingInfo || !url.trim()} className="h-10 px-5 font-semibold shadow-sm hover:shadow-md transition-shadow">
                            {isGettingInfo ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Search className="h-4 w-4 mr-2" />}
                            {isGettingInfo ? t('url.gettingInfo') : t('url.getInfo')}
                        </Button>
                    </div>
                )}

                {/* Video/Playlist Info */}
                {ytdlp?.available && (videoInfo || playlistInfo) && (
                    <div className="space-y-2.5 animate-fade-in-up delay-1">
                        {videoInfo && (
                            <div className="flex gap-3.5 rounded-xl border bg-card/70 backdrop-blur-sm p-3.5 shadow-sm">
                                {videoInfo.thumbnail && (
                                    <img src={videoInfo.thumbnail} alt={videoInfo.title} className="w-36 h-[81px] object-cover rounded-lg shrink-0 shadow-sm" onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
                                )}
                                <div className="flex-1 min-w-0 space-y-1">
                                    <div className="font-semibold text-sm line-clamp-2 leading-snug">{videoInfo.title}</div>
                                    <div className="flex gap-x-3 gap-y-0.5 text-xs text-muted-foreground flex-wrap">
                                        {videoInfo.duration > 0 && <span>{t('video.duration')}: {formatDuration(videoInfo.duration)}</span>}
                                        {videoInfo.uploader && <span>{t('video.uploader')}: {videoInfo.uploader}</span>}
                                        {videoInfo.platform && <span>{t('video.platform')}: {videoInfo.platform}</span>}
                                    </div>
                                </div>
                            </div>
                        )}

                        {playlistInfo && (
                            <>
                                <div className="flex gap-3.5 rounded-xl border bg-card/70 backdrop-blur-sm p-3.5 shadow-sm">
                                    <div className="w-12 h-12 flex items-center justify-center rounded-lg bg-gradient-to-br from-primary/15 to-primary/5 text-primary shrink-0">
                                        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
                                    </div>
                                    <div className="flex-1 min-w-0 space-y-1">
                                        <div className="font-semibold text-sm leading-snug">
                                            {t(`collection.${collectionKind}.detected` as any)}{playlistInfo.title ? `: ${playlistInfo.title}` : ''}
                                        </div>
                                        <div className="flex gap-x-3 gap-y-0.5 text-xs text-muted-foreground flex-wrap">
                                            <span>{t(`collection.${collectionKind}.count` as any, {count: String(playlistInfo.count)})}</span>
                                            {playlistInfo.uploader && <span>{t('playlist.uploader')}: {playlistInfo.uploader}</span>}
                                            <span>{t('collection.selected', {count: String(selectedPlaylistItems.size)})}</span>
                                        </div>
                                    </div>
                                </div>
                                <div className="rounded-xl border overflow-hidden shadow-sm">
                                    <div className="flex gap-1.5 px-2.5 py-1.5 border-b bg-muted/40">
                                        <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setSelectedPlaylistItems(new Set(playlistInfo.videos.map((_: any, i: number) => i)))}>
                                            {t('collection.selectAll')}
                                        </Button>
                                        <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setSelectedPlaylistItems(new Set())}>
                                            {t('collection.selectNone')}
                                        </Button>
                                    </div>
                                    <div className="max-h-60 overflow-y-auto">
                                        {playlistInfo.videos.map((video, idx) => (
                                            <label key={idx} className="flex items-center gap-2.5 px-2.5 py-1.5 hover:bg-muted/50 cursor-pointer text-sm border-b last:border-b-0">
                                                <Checkbox
                                                    checked={selectedPlaylistItems.has(idx)}
                                                    onCheckedChange={(checked: boolean) => {
                                                        const next = new Set(selectedPlaylistItems)
                                                        if (checked) next.add(idx); else next.delete(idx)
                                                        setSelectedPlaylistItems(next)
                                                    }}
                                                />
                                                <span className="text-xs text-muted-foreground w-6 text-right shrink-0">{idx + 1}</span>
                                                <span className="flex-1 min-w-0 truncate text-xs">{video.title || video.url || video.id}</span>
                                                {video.duration > 0 && <span className="text-[11px] text-muted-foreground shrink-0">{formatDuration(video.duration)}</span>}
                                            </label>
                                        ))}
                                    </div>
                                </div>
                            </>
                        )}
                    </div>
                )}

                {/* Controls Zone */}
                {ytdlp?.available && (videoInfo || playlistInfo) && (
                    <div className="rounded-xl border bg-card/70 backdrop-blur-sm p-4 space-y-3.5 shadow-sm animate-fade-in-up delay-2">
                        {/* Output Directory */}
                        {!(backendMode === 'web' && getWebConfig()?.hasFixedDir) && (
                            <div className="space-y-1.5">
                                <Label className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium">{t('outputDir.label')}</Label>
                                <div className="flex gap-2">
                                    <Input
                                        type="text"
                                        value={outputDir}
                                        onChange={e => setOutputDir(e.target.value)}
                                        placeholder={backendMode === 'web' ? t('outputDir.serverPathPlaceholder') : t('outputDir.placeholder')}
                                    />
                                    {backendMode === 'desktop' ? (
                                        <Button variant="outline" size="sm" onClick={handleSelectFolder}>
                                            <FolderOpen className="h-4 w-4 mr-1" />{t('outputDir.browse')}
                                        </Button>
                                    ) : (
                                        <Button variant="outline" size="sm" onClick={() => setShowDirBrowser(true)}>
                                            <FolderOpen className="h-4 w-4 mr-1" />{t('outputDir.browse')}
                                        </Button>
                                    )}
                                </div>
                            </div>
                        )}

                        {/* Format Section */}
                        {videoInfo && (
                            <Collapsible open={formatExpanded} onOpenChange={setFormatExpanded}>
                                <CollapsibleTrigger className="flex items-center gap-1.5 w-full rounded-lg border px-3 py-2 text-sm font-medium hover:bg-muted/40 transition-all duration-150">
                                    {formatExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                                    <span>{t('format.label')}</span>
                                    {!formatExpanded && hasCustomFormatSelection && (
                                        <span className="text-xs text-primary truncate ml-1.5">
                                            {selectedFormat && formatInfo
                                                ? formatOptionLabel(formatInfo.formats.find(f => f.formatId === selectedFormat)!)
                                                : (selectedVideoFormat || selectedAudioFormat)
                                                    ? `${selectedVideoFormat ? t('format.video') : ''}${selectedVideoFormat && selectedAudioFormat ? ' + ' : ''}${selectedAudioFormat ? t('format.audio') : ''}`
                                                    : ''}
                                        </span>
                                    )}
                                </CollapsibleTrigger>
                                <CollapsibleContent className="mt-2 space-y-2.5">
                                    {formatInfo ? (
                                        <div className="space-y-3">
                                            <div className="flex items-center gap-2">
                                                <Select value={formatMode} onValueChange={(val: string) => {
                                                    const nextMode = val as 'single' | 'combine' | 'audio-only' | 'video-only'
                                                    setFormatMode(nextMode)
                                                    setSelectedFormat(''); setSelectedVideoFormat(''); setSelectedAudioFormat('')
                                                }}>
                                                    <SelectTrigger className="w-48"><SelectValue /></SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="single">{t('format.mode.single')}</SelectItem>
                                                        <SelectItem value="combine">{t('format.mode.combine')}</SelectItem>
                                                        <SelectItem value="audio-only">{t('format.mode.audioOnly')}</SelectItem>
                                                        <SelectItem value="video-only">{t('format.mode.videoOnly')}</SelectItem>
                                                    </SelectContent>
                                                </Select>
                                                <Button variant="outline" size="sm" onClick={handleSelectBestQuality}>
                                                    {t('format.bestQuality')}
                                                </Button>
                                            </div>

                                            {formatMode === 'single' && (
                                                <div className="max-h-52 overflow-y-auto rounded-lg border">
                                                    {sortFormats(formatInfo.formats.filter(f => f.hasVideo || f.hasAudio)).map(f => (
                                                        <label key={f.formatId} className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedFormat === f.formatId ? 'bg-primary/5' : ''}`}>
                                                            <input type="radio" name="format-single" checked={selectedFormat === f.formatId}
                                                                onChange={() => { setSelectedFormat(f.formatId); setSelectedVideoFormat(''); setSelectedAudioFormat('') }}
                                                                className="mt-0.5 accent-primary" />
                                                            <span className="flex-1 break-words leading-relaxed">{formatOptionLabel(f)}</span>
                                                        </label>
                                                    ))}
                                                </div>
                                            )}

                                            {formatMode === 'audio-only' && (
                                                <div className="max-h-52 overflow-y-auto rounded-md border">
                                                    <label className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedAudioFormat ? 'bg-primary/5' : ''}`}>
                                                        <input type="radio" name="format-audio-only" checked={!selectedAudioFormat} onChange={() => setSelectedAudioFormat('')} className="mt-0.5 accent-primary" />
                                                        <span>{t('format.auto')}</span>
                                                    </label>
                                                    {audioOnlyFormats.sort((a, b) => (b.tbr || b.filesize || 0) - (a.tbr || a.filesize || 0)).map(f => (
                                                        <label key={f.formatId} className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedAudioFormat === f.formatId ? 'bg-primary/5' : ''}`}>
                                                            <input type="radio" name="format-audio-only" checked={selectedAudioFormat === f.formatId}
                                                                onChange={() => { setSelectedAudioFormat(f.formatId); setSelectedVideoFormat(''); setSelectedFormat('') }}
                                                                className="mt-0.5 accent-primary" />
                                                            <span className="flex-1 break-words leading-relaxed">{formatOptionLabel(f)}</span>
                                                        </label>
                                                    ))}
                                                </div>
                                            )}

                                            {formatMode === 'video-only' && (
                                                <div className="max-h-52 overflow-y-auto rounded-md border">
                                                    <label className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedVideoFormat ? 'bg-primary/5' : ''}`}>
                                                        <input type="radio" name="format-video-only" checked={!selectedVideoFormat} onChange={() => setSelectedVideoFormat('')} className="mt-0.5 accent-primary" />
                                                        <span>{t('format.auto')}</span>
                                                    </label>
                                                    {videoOnlyFormats.sort((a, b) => {
                                                        const aH = parseResolutionHeight(a.resolution || '')
                                                        const bH = parseResolutionHeight(b.resolution || '')
                                                        if (aH !== bH) return bH - aH
                                                        return (b.filesize || 0) - (a.filesize || 0)
                                                    }).map(f => (
                                                        <label key={f.formatId} className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedVideoFormat === f.formatId ? 'bg-primary/5' : ''}`}>
                                                            <input type="radio" name="format-video-only" checked={selectedVideoFormat === f.formatId}
                                                                onChange={() => { setSelectedVideoFormat(f.formatId); setSelectedAudioFormat(''); setSelectedFormat('') }}
                                                                className="mt-0.5 accent-primary" />
                                                            <span className="flex-1 break-words leading-relaxed">{formatOptionLabel(f)}</span>
                                                        </label>
                                                    ))}
                                                </div>
                                            )}

                                            {formatMode === 'combine' && hasSeparateTrackFormats ? (
                                                <div className="grid grid-cols-2 gap-3">
                                                    <div className="space-y-1.5">
                                                        <Label className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium">{t('format.video')}</Label>
                                                        <div className="max-h-52 overflow-y-auto rounded-md border">
                                                            <label className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedVideoFormat ? 'bg-primary/5' : ''}`}>
                                                                <input type="radio" name="format-video" checked={!selectedVideoFormat} onChange={() => setSelectedVideoFormat('')} className="mt-0.5 accent-primary" />
                                                                <span>{t('format.selectVideo')}</span>
                                                            </label>
                                                            {combineVideoFormats.map(f => (
                                                                <label key={f.formatId} className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedVideoFormat === f.formatId ? 'bg-primary/5' : ''}`}>
                                                                    <input type="radio" name="format-video" checked={selectedVideoFormat === f.formatId} onChange={() => setSelectedVideoFormat(f.formatId)} className="mt-0.5 accent-primary" />
                                                                    <span className="flex-1 break-words leading-relaxed">{formatOptionLabel(f)}</span>
                                                                </label>
                                                            ))}
                                                        </div>
                                                    </div>
                                                    <div className="space-y-1.5">
                                                        <Label className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium">{t('format.audio')}</Label>
                                                        <div className="max-h-52 overflow-y-auto rounded-md border">
                                                            <label className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedAudioFormat ? 'bg-primary/5' : ''}`}>
                                                                <input type="radio" name="format-audio" checked={!selectedAudioFormat} onChange={() => setSelectedAudioFormat('')} className="mt-0.5 accent-primary" />
                                                                <span>{t('format.selectAudio')}</span>
                                                            </label>
                                                            {combineAudioFormats.map(f => (
                                                                <label key={f.formatId} className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedAudioFormat === f.formatId ? 'bg-primary/5' : ''}`}>
                                                                    <input type="radio" name="format-audio" checked={selectedAudioFormat === f.formatId} onChange={() => setSelectedAudioFormat(f.formatId)} className="mt-0.5 accent-primary" />
                                                                    <span className="flex-1 break-words leading-relaxed">{formatOptionLabel(f)}</span>
                                                                </label>
                                                            ))}
                                                        </div>
                                                    </div>
                                                </div>
                                            ) : formatMode === 'combine' && !hasSeparateTrackFormats ? (
                                                <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
                                                    {t('format.combineUnavailable')}
                                                </div>
                                            ) : null}
                                        </div>
                                    ) : (
                                        <div className="flex items-center gap-2">
                                            {isGettingFormats ? (
                                                <span className="text-sm text-muted-foreground">{t('format.loading')}</span>
                                            ) : (
                                                <Button variant="outline" size="sm" onClick={handleGetFormats} disabled={isGettingFormats}>
                                                    {isGettingFormats ? t('format.loading') : t('format.detect')}
                                                </Button>
                                            )}
                                        </div>
                                    )}
                                </CollapsibleContent>
                            </Collapsible>
                        )}

                        {/* Download Options */}
                        {videoInfo && (
                            <div className="rounded-lg border bg-muted/20 p-3.5 space-y-2.5">
                                <div className="text-[11px] font-semibold uppercase tracking-widest text-muted-foreground flex items-center gap-1.5">
                                    <SettingsIcon className="h-3 w-3" /> {t('downloadOpt.title')}
                                </div>
                                <div className="space-y-1">
                                    <div className="flex items-center gap-1.5">
                                        <Label className="text-[11px] text-muted-foreground">{t('downloadOpt.filenameTemplate')}</Label>
                                        <Tooltip>
                                            <TooltipTrigger><Info className="h-3 w-3 text-muted-foreground" /></TooltipTrigger>
                                            <TooltipContent className="max-w-xs text-xs space-y-1">
                                                <div>{t('downloadOpt.filenameTemplateHelp')}</div>
                                                <div className="font-mono text-[10px] break-all">%(title)s · %(uploader)s · %(upload_date)s · %(id)s · %(ext)s</div>
                                            </TooltipContent>
                                        </Tooltip>
                                    </div>
                                    <Input
                                        type="text"
                                        value={dlOptFilenameTemplate}
                                        onChange={e => setDlOptFilenameTemplate(e.target.value)}
                                        placeholder={currentSettings?.filenameTemplate || '%(title)s.%(ext)s'}
                                        className="h-7 text-xs"
                                    />
                                </div>
                                <div className="grid grid-cols-2 sm:grid-cols-3 gap-x-3 gap-y-1.5">
                                    {([
                                        { key: 'saveThumbnail', checked: dlOptSaveThumbnail, set: setDlOptSaveThumbnail, label: t('downloadOpt.saveThumbnail') },
                                        { key: 'saveDescription', checked: dlOptSaveDescription, set: setDlOptSaveDescription, label: t('downloadOpt.saveDescription') },
                                        { key: 'embedChapters', checked: dlOptEmbedChapters, set: setDlOptEmbedChapters, label: t('downloadOpt.embedChapters') },
                                        { key: 'writeSubtitles', checked: dlOptWriteSubtitles, set: setDlOptWriteSubtitles, label: t('downloadOpt.writeSubtitles') },
                                        ...(dlOptWriteSubtitles ? [{ key: 'embedSubtitles', checked: dlOptEmbedSubtitles, set: setDlOptEmbedSubtitles, label: t('downloadOpt.embedSubtitles') }] : []),
                                        { key: 'sponsorBlock', checked: dlOptSponsorBlock, set: setDlOptSponsorBlock, label: t('downloadOpt.sponsorBlock') },
                                    ] as const).map(opt => (
                                        <label key={opt.key} className="flex items-center gap-1.5 cursor-pointer">
                                            <Checkbox checked={opt.checked}
                                                onCheckedChange={(checked: boolean) => {
                                                    const val = !!checked; opt.set(val)
                                                    persistSettingsPatch({[opt.key]: val} as any)
                                                }} />
                                            <span className="text-xs font-normal">
                                                {opt.label}
                                            </span>
                                            {opt.key === 'sponsorBlock' && (
                                                <Tooltip>
                                                    <TooltipTrigger><Info className="h-3 w-3 text-muted-foreground" /></TooltipTrigger>
                                                    <TooltipContent className="max-w-xs">{t('downloadOpt.sponsorBlockDesc')}</TooltipContent>
                                                </Tooltip>
                                            )}
                                        </label>
                                    ))}
                                </div>

                                {dlOptWriteSubtitles && videoInfo.subtitles && videoInfo.subtitles.length > 0 && (
                                    <div className="space-y-1.5">
                                        <Label className="text-[11px] text-muted-foreground">{t('downloadOpt.subtitleLangs')}</Label>
                                        <Input
                                            type="text"
                                            value={subtitleSearch}
                                            onChange={e => setSubtitleSearch(e.target.value)}
                                            placeholder={t('downloadOpt.subtitleSearch')}
                                            className="h-7 text-xs"
                                        />
                                        <div className="max-h-28 overflow-y-auto rounded-md border">
                                            {videoInfo.subtitles
                                                .filter(sub => {
                                                    if (!subtitleSearch.trim()) return true
                                                    const q = subtitleSearch.toLowerCase()
                                                    return (sub.name || '').toLowerCase().includes(q) || sub.code.toLowerCase().includes(q)
                                                })
                                                .map(sub => (
                                                <label key={getSubtitleSelectionKey(sub)} className="flex items-center gap-2 px-2.5 py-1 hover:bg-muted/50 cursor-pointer text-xs border-b last:border-b-0">
                                                    <Checkbox checked={selectedSubtitleLangs.has(getSubtitleSelectionKey(sub))}
                                                        onCheckedChange={(checked: boolean) => {
                                                            const next = new Set(selectedSubtitleLangs)
                                                            const key = getSubtitleSelectionKey(sub)
                                                            if (checked) next.add(key); else next.delete(key)
                                                            setSelectedSubtitleLangs(next)
                                                        }} />
                                                    <span className="flex-1 truncate">{sub.name || sub.code}</span>
                                                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0 shrink-0">
                                                        {sub.auto ? t('downloadOpt.subtitleAuto') : t('downloadOpt.subtitleManual')}
                                                    </Badge>
                                                </label>
                                            ))}
                                        </div>
                                    </div>
                                )}
                                {dlOptWriteSubtitles && (!videoInfo.subtitles || videoInfo.subtitles.length === 0) && (
                                    <p className="text-xs text-muted-foreground">{t('downloadOpt.noSubtitles')}</p>
                                )}
                            </div>
                        )}

                        {/* Action Buttons */}
                        <div className="flex gap-2.5 flex-wrap items-center pt-1">
                            {backendMode === 'web' && !getWebConfig()?.hasFixedDir && !outputDir && (
                                <div className="flex items-center gap-2 mr-auto text-yellow-500 text-sm">
                                    <span>{t('web.noDirHint')}</span>
                                    <Button variant="outline" size="sm" onClick={() => setShowSettings(true)}>
                                        {t('web.openSettings')}
                                    </Button>
                                </div>
                            )}
                            <Button onClick={handleDownload} disabled={isStarting || !url.trim() || !outputDir} className="ml-auto h-10 px-6 font-semibold shadow-sm hover:shadow-md glow-primary transition-all duration-200">
                                {isStarting ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
                                {isStarting ? t('download.downloading') : t('download.start')}
                            </Button>
                            {playlistInfo && playlistInfo.count > 0 && (
                                <Button onClick={handleDownloadAll} disabled={isStarting || !outputDir || selectedPlaylistItems.size === 0}>
                                    {isStarting ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
                                    {isStarting ? t('playlist.startingAll') : `${t(`collection.${collectionKind}.downloadAll` as any)} (${selectedPlaylistItems.size})`}
                                </Button>
                            )}
                        </div>
                    </div>
                )}

                {/* Console & Downloads */}
                <div className="space-y-3 animate-fade-in-up delay-3">
                    {consoleLogs.length > 0 && (
                        <div className="rounded-xl border overflow-hidden shadow-sm">
                            <div className="flex items-center justify-between px-2.5 py-1.5 border-b bg-muted/40">
                                <Button variant="ghost" size="sm" onClick={() => setShowConsole(!showConsole)} className="text-xs h-7 px-1.5">
                                    {showConsole ? <ChevronDown className="h-3 w-3 mr-1" /> : <ChevronRight className="h-3 w-3 mr-1" />}
                                    {t('console.title')} ({consoleLogs.length})
                                </Button>
                                <Button variant="ghost" size="sm" onClick={() => { setConsoleLogs([]); setShowConsole(false) }} className="text-xs h-7 px-1.5">
                                    {t('console.clear')}
                                </Button>
                            </div>
                            {showConsole && (
                                <div className="max-h-72 overflow-y-auto overflow-x-hidden">
                                    <pre className="bg-muted/20 p-2 text-[11px] font-mono leading-snug whitespace-pre-wrap break-words">
                                        {consoleLogs.map((line, i) => (
                                            <div key={i} className={`px-1.5 py-0.5 border-l-2 rounded-r-sm ${
                                                getConsoleLogType(line) === 'error' ? 'border-l-red-500 bg-red-500/5 text-red-400' :
                                                getConsoleLogType(line) === 'warning' ? 'border-l-yellow-500 bg-yellow-500/5 text-yellow-400' :
                                                getConsoleLogType(line) === 'command' ? 'border-l-blue-500 bg-blue-500/5 text-blue-400' :
                                                'border-l-transparent'
                                            }`}>{line}</div>
                                        ))}
                                        <div ref={consoleEndRef} />
                                    </pre>
                                </div>
                            )}
                        </div>
                    )}

                    <DownloadList downloads={downloads} onUpdate={setDownloads} />
                </div>
            </main>

            {/* Dialogs */}
            <SettingsDialog
                open={showSettings} initialSettings={currentSettings}
                onClose={() => setShowSettings(false)}
                onSaved={(s) => {
                    applySettingsToUI(s)
                    if (s.notifications && 'Notification' in window && Notification.permission === 'default') {
                        Notification.requestPermission()
                    }
                }}
                onThemePreview={setTheme}
                onLanguagePreview={setLang}
            />

            {showSetupWizard && (
                <SetupWizard
                    onComplete={async (outputDir, cookiesFrom, cookiesFile, proxy, language, theme) => {
                        const settings: Settings = {
                            outputDir, quality: 'best', language, theme, proxy, rateLimit: '',
                            maxConcurrent: 3, notifications: true, saveDescription: false, saveThumbnail: false,
                            writeSubtitles: false, subtitleLangs: '', embedSubtitles: false, embedChapters: false,
                            sponsorBlock: false, filenameTemplate: '', mergeOutputFormat: '', audioFormat: '',
                            cookiesFrom, cookiesFile,
                        }
                        await SaveSettings(settings)
                        setCurrentSettings(settings); setOutputDir(outputDir); setTheme(theme); setLang(language)
                        setShowSetupWizard(false)
                    }}
                />
            )}

            <UpdateDialog
                open={showUpdateDialog} updateInfo={updateInfo} loading={updateLoading} error={updateError}
                onClose={() => setShowUpdateDialog(false)}
                onOpenReleasePage={handleOpenReleasePage}
                onCheckUpdate={handleCheckUpdate}
            />

            <DirBrowser
                open={showDirBrowser} initialPath={outputDir}
                onSelect={dir => {
                    setOutputDir(dir)
                    if (currentSettings) {
                        const next = {...currentSettings, outputDir: dir}
                        setCurrentSettings(next)
                        SaveSettings(next).catch(console.error)
                    }
                }}
                onClose={() => setShowDirBrowser(false)}
            />
        </div>
    )
}

export default App
