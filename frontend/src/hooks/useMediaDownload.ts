import {useCallback, useRef, useState} from 'react'
import {toast} from 'sonner'
import {GetFormats, GetPlaylistInfo, GetVideoInfo, SelectFolder, StartDownload} from '../lib/backend'
import {useI18n} from '../i18n/context'
import {
    buildDownloadOptions,
    formatOptionLabel,
    FormatMode,
    getDownloadDefaults,
    isSingleDownloadLink,
    parseResolutionHeight,
    resolveDownloadQuality,
    sortFormats,
} from '../lib/downloadComposer'
import type {DownloadProfile, FormatInfo, PlaylistInfo, Settings, VideoInfo} from '../types'

interface MediaDownloadOptions {
    settings: Settings | null
    profile: DownloadProfile | undefined
    hasFixedDir: boolean
}

type DownloadDefaults = ReturnType<typeof getDownloadDefaults>
type TaskOverrides = Partial<Omit<DownloadDefaults, 'options'>> & {options?: Partial<DownloadDefaults['options']>}

function shouldTryPlaylistFallback(error: unknown): boolean {
    const message = String((error as any)?.message || error || '').toLowerCase()
    if (!message) return true
    const nonFallbackSignals = [
        'js runtime', 'deno', 'node.js', 'sign in to confirm', 'not a bot',
        'dpapi', 'cookies', 'storyboard', 'rejected the current access',
        'requires valid login cookies', 'wechat channels', 'yuanbao',
        'weixin.qq.com', 'channels.weixin.qq.com',
        '请安装', '拒绝了当前访问', '需要有效的登录 cookies',
        '视频号', '元宝',
    ]
    return !nonFallbackSignals.some(signal => message.includes(signal))
}

export function useMediaDownload({settings, profile, hasFixedDir}: MediaDownloadOptions) {
    const {t} = useI18n()
    const [url, setUrl] = useState('')
    const [resolvedUrl, setResolvedUrl] = useState('')
    const [videoInfo, setVideoInfo] = useState<VideoInfo | null>(null)
    const [playlistInfo, setPlaylistInfo] = useState<PlaylistInfo | null>(null)
    const [formatInfo, setFormatInfo] = useState<FormatInfo | null>(null)
    const [formatMode, setFormatMode] = useState<FormatMode>('single')
    const [selectedFormat, setSelectedFormat] = useState('')
    const [selectedVideoFormat, setSelectedVideoFormat] = useState('')
    const [selectedAudioFormat, setSelectedAudioFormat] = useState('')
    const [isGettingFormats, setIsGettingFormats] = useState(false)
    const [formatExpanded, setFormatExpanded] = useState(true)
    const [selectedPlaylistItems, setSelectedPlaylistItems] = useState<Set<number>>(new Set())
    const [isGettingInfo, setIsGettingInfo] = useState(false)
    const [isStarting, setIsStarting] = useState(false)
    const [taskOverrides, setTaskOverrides] = useState<TaskOverrides>({})
    const [selectedSubtitleLangs, setSelectedSubtitleLangs] = useState<Set<string>>(new Set())
    const [subtitleSearch, setSubtitleSearch] = useState('')
    const infoRequestRef = useRef(0)
    const startingRef = useRef(false)
    const defaults = getDownloadDefaults(settings, profile)
    const task = {...defaults, ...taskOverrides, options: {...defaults.options, ...taskOverrides.options}}
    const outputDir = (hasFixedDir ? settings?.outputDir : task.outputDir)?.trim() || ''
    const canQuickDownload = isSingleDownloadLink(url.trim())

    const resetResolvedMedia = useCallback(() => {
        setResolvedUrl('')
        setIsGettingInfo(false)
        setIsGettingFormats(false)
        setVideoInfo(null)
        setPlaylistInfo(null)
        setFormatInfo(null)
        setFormatMode('single')
        setSelectedFormat('')
        setSelectedVideoFormat('')
        setSelectedAudioFormat('')
        setSelectedPlaylistItems(new Set())
        setSelectedSubtitleLangs(new Set())
        setSubtitleSearch('')
        setFormatExpanded(false)
    }, [])

    const handleURLChange = useCallback((nextUrl: string) => {
        infoRequestRef.current++
        setUrl(nextUrl)
        setTaskOverrides({})
        resetResolvedMedia()
    }, [resetResolvedMedia])

    const clearCurrentInput = useCallback(() => {
        infoRequestRef.current++
        setUrl('')
        setTaskOverrides({})
        resetResolvedMedia()
    }, [resetResolvedMedia])

    const videoOnlyFormats = formatInfo?.formats.filter(format => format.hasVideo && !format.hasAudio) || []
    const audioOnlyFormats = formatInfo?.formats.filter(format => format.hasAudio && !format.hasVideo) || []
    const combineVideoFormats = videoOnlyFormats.sort((a, b) => {
        const heightDifference = parseResolutionHeight(b.resolution || '') - parseResolutionHeight(a.resolution || '')
        return heightDifference || (b.filesize || 0) - (a.filesize || 0)
    })
    const combineAudioFormats = audioOnlyFormats.sort((a, b) => (b.tbr || b.filesize || 0) - (a.tbr || a.filesize || 0))
    const hasSeparateTrackFormats = videoOnlyFormats.length > 0 && audioOnlyFormats.length > 0
    const hasCustomFormatSelection = !!selectedFormat || !!selectedVideoFormat || !!selectedAudioFormat
    const hasAvailableSubtitles = (videoInfo?.subtitles?.length || 0) > 0
    const isWechatChannelsVideo = videoInfo?.platform === 'WeChat Channels'
    const collectionKind = playlistInfo?.kind === 'channel' ? 'channel' : 'playlist'

    const getFormatOptionLabel = (format: FormatInfo['formats'][number]) => {
        if (format.formatId === 'wechat:origin') return t('format.wechatOriginal')
        if (format.formatId === 'wechat:preview') return t('format.wechatPreview')
        return formatOptionLabel(format)
    }

    const selectBestQuality = () => {
        if (!formatInfo) return
        const declaredBest = formatInfo.formats.find(format => format.formatId === 'wechat:best')
            || formatInfo.formats.find(format => format.formatId === 'wechat:origin')
        if (declaredBest) {
            setFormatMode('single'); setSelectedFormat(declaredBest.formatId)
            setSelectedVideoFormat(''); setSelectedAudioFormat(''); return
        }
        const combined = sortFormats(formatInfo.formats.filter(format => format.hasVideo && format.hasAudio))
        if (combined.length > 0) {
            setFormatMode('single'); setSelectedFormat(combined[0].formatId)
            setSelectedVideoFormat(''); setSelectedAudioFormat(''); return
        }
        if (videoOnlyFormats.length > 0 || audioOnlyFormats.length > 0) {
            setFormatMode('combine'); setSelectedFormat('')
            setSelectedVideoFormat(videoOnlyFormats[0]?.formatId || '')
            setSelectedAudioFormat(audioOnlyFormats[0]?.formatId || '')
        }
    }

    const getInfo = async () => {
        const requestedUrl = url.trim()
        if (!requestedUrl) return
        const requestId = ++infoRequestRef.current
        resetResolvedMedia()
        setIsGettingInfo(true)
        try {
            const info = await GetVideoInfo(requestedUrl)
            if (requestId !== infoRequestRef.current) return
            setVideoInfo(info); setFormatExpanded(true)
            setResolvedUrl(requestedUrl)
            setIsGettingFormats(true)
            try {
                const formats = await GetFormats(requestedUrl)
                if (requestId === infoRequestRef.current) setFormatInfo(formats)
            } catch {
            } finally {
                if (requestId === infoRequestRef.current) setIsGettingFormats(false)
            }
        } catch (error: any) {
            if (requestId !== infoRequestRef.current) return
            if (!shouldTryPlaylistFallback(error)) {
                toast.error(t('toast.getInfoFail') + (error?.message ? `: ${error.message}` : ''))
                return
            }
            try {
                const playlist = await GetPlaylistInfo(requestedUrl)
                if (requestId !== infoRequestRef.current) return
                if (playlist && playlist.count > 0) {
                    setPlaylistInfo(playlist)
                    setResolvedUrl(requestedUrl)
                    setSelectedPlaylistItems(new Set(playlist.videos.map((_, index) => index)))
                } else {
                    toast.error(t('toast.getInfoFail') + (error?.message ? `: ${error.message}` : ''))
                }
            } catch {
                if (requestId === infoRequestRef.current) {
                    toast.error(t('toast.getInfoFail') + (error?.message ? `: ${error.message}` : ''))
                }
            }
        } finally {
            if (requestId === infoRequestRef.current) setIsGettingInfo(false)
        }
    }

    const getFormats = async () => {
        const requestedUrl = url.trim()
        if (!requestedUrl || requestedUrl !== resolvedUrl) return
        const requestId = infoRequestRef.current
        setIsGettingFormats(true); setFormatExpanded(true)
        try {
            const info = await GetFormats(requestedUrl)
            if (requestId === infoRequestRef.current) {
                setFormatInfo(info); setSelectedFormat(''); setSelectedVideoFormat(''); setSelectedAudioFormat('')
            }
        } catch (error: any) {
            if (requestId === infoRequestRef.current) {
                toast.error(t('toast.getFormatsFail') + (error?.message ? `: ${error.message}` : ''))
            }
        } finally {
            if (requestId === infoRequestRef.current) setIsGettingFormats(false)
        }
    }

    const setTaskOption = <Key extends keyof TaskOverrides>(key: Key, value: TaskOverrides[Key]) => {
        setTaskOverrides(current => ({...current, [key]: value}))
    }

    const setDownloadOption = (key: keyof DownloadDefaults['options'], value: boolean) => {
        setTaskOverrides(current => ({...current, options: {...current.options, [key]: value}}))
    }

    const buildRequestOptions = (singleItem?: boolean) => buildDownloadOptions({
        videoInfo,
        selectedSubtitleLangs,
        subtitleLangs: task.subtitleLangs.trim() || (taskOverrides.subtitleLangs === undefined ? '' : 'all,-live_chat'),
        options: {
            ...task.options,
            filenameTemplate: task.filenameTemplate.trim() || defaults.filenameTemplate.trim() || undefined,
            videoContainer: task.videoContainer,
            audioFormat: task.audioFormat,
            singleItem,
        },
    })

    const resolveQuality = () => resolveDownloadQuality({
        formatMode,
        selectedFormat,
        selectedVideoFormat,
        selectedAudioFormat,
        formatInfo,
        quality: task.quality,
    })

    const startSingleDownload = async (quick: boolean) => {
        const requestedUrl = url.trim()
        const requestId = infoRequestRef.current
        if (!requestedUrl || startingRef.current) return
        if (quick && !canQuickDownload) { toast.error(t('download.invalidLink')); return }
        if (!quick && requestedUrl !== resolvedUrl) { toast.error(t('url.getInfo')); return }
        if (!outputDir && !hasFixedDir) { toast.error(t('download.noDir')); return }
        if (formatMode === 'combine' && !hasSeparateTrackFormats) {
            toast.error(t('format.combineUnavailable')); return
        }
        startingRef.current = true
        setIsStarting(true)
        try {
            await StartDownload({
                url: requestedUrl, outputDir, quality: resolveQuality(),
                videoInfo: videoInfo ? {
                    url: videoInfo.url, id: videoInfo.id, title: videoInfo.title,
                    thumbnail: videoInfo.thumbnail, duration: videoInfo.duration,
                    uploader: videoInfo.uploader, platform: videoInfo.platform,
                } : undefined,
                options: buildRequestOptions(quick || undefined),
            })
            if (requestId === infoRequestRef.current) resetTaskOptions()
            toast.success(t('toast.downloadQueued'))
        } catch (error: any) {
            toast.error(t('toast.downloadStartFail') + (error?.message ? `: ${error.message}` : ''))
        } finally {
            startingRef.current = false
            setIsStarting(false)
        }
    }

    const downloadAll = async () => {
        if (!playlistInfo || !outputDir) {
            if (!outputDir) toast.error(t('download.noDir'))
            return
        }
        if (url.trim() !== resolvedUrl) { toast.error(t('url.getInfo')); return }
        if (formatMode === 'combine') {
            if (!hasSeparateTrackFormats) { toast.error(t('format.combineUnavailable')); return }
            if (!selectedVideoFormat && !selectedAudioFormat) { toast.error(t('toast.selectAnyTrack')); return }
        }
        setIsStarting(true)
        try {
            const downloadQuality = resolveQuality()
            let startedCount = 0
            for (let index = 0; index < playlistInfo.videos.length; index++) {
                if (!selectedPlaylistItems.has(index)) continue
                const video = playlistInfo.videos[index]
                if (!video.url) continue
                await StartDownload({
                    url: video.url, outputDir, quality: downloadQuality,
                    videoInfo: {
                        url: video.url, id: video.id, title: video.title,
                        thumbnail: video.thumbnail, duration: video.duration,
                        uploader: video.uploader, platform: video.platform,
                    },
                    options: buildRequestOptions(),
                })
                startedCount++
            }
            if (startedCount > 0) toast.success(t('toast.downloadQueuedCount', {count: String(startedCount)}))
        } catch (error: any) {
            toast.error(t('toast.downloadStartFail') + (error?.message ? `: ${error.message}` : ''))
        } finally {
            setIsStarting(false)
        }
    }

    const selectFolder = async () => {
        const directory = await SelectFolder()
        if (directory) setTaskOption('outputDir', directory)
    }

    const resetTaskOptions = () => {
        setTaskOverrides({})
        setSelectedSubtitleLangs(new Set())
    }

    return {
        url, handleURLChange, clearCurrentInput,
        videoInfo, playlistInfo, formatInfo,
        formatMode, setFormatMode,
        selectedFormat, setSelectedFormat,
        selectedVideoFormat, setSelectedVideoFormat,
        selectedAudioFormat, setSelectedAudioFormat,
        isGettingFormats, formatExpanded, setFormatExpanded,
        selectedPlaylistItems, setSelectedPlaylistItems,
        isGettingInfo, isStarting,
        task, outputDir, setTaskOption, setDownloadOption,
        canQuickDownload, resolveQuality, resetTaskOptions,
        hasTaskOverrides: Object.keys(taskOverrides).length > 0 || selectedSubtitleLangs.size > 0,
        selectedSubtitleLangs, setSelectedSubtitleLangs,
        subtitleSearch, setSubtitleSearch,
        videoOnlyFormats, audioOnlyFormats, combineVideoFormats, combineAudioFormats,
        collectionKind, hasSeparateTrackFormats, hasCustomFormatSelection,
        hasAvailableSubtitles, isWechatChannelsVideo,
        getFormatOptionLabel, selectBestQuality, getInfo, getFormats,
        download: () => startSingleDownload(false), quickDownload: () => startSingleDownload(true), downloadAll, selectFolder,
    }
}

export type MediaDownloadController = ReturnType<typeof useMediaDownload>
