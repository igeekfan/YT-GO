import type {DownloadOptions, DownloadProfile, FormatInfo, Settings, VideoInfo} from '../types'
import {formatFileSize} from './formatUtils'
import {DEFAULT_FILENAME_TEMPLATE} from './downloadProfiles'

export type FormatMode = 'single' | 'combine' | 'audio-only' | 'video-only'

export interface ResolveDownloadQualityInput {
    formatMode: FormatMode
    selectedFormat: string
    selectedVideoFormat: string
    selectedAudioFormat: string
    formatInfo: FormatInfo | null
    quality: string
}

export type DownloadOptionKey = 'saveThumbnail' | 'saveDescription' | 'embedChapters' | 'writeSubtitles' | 'embedSubtitles' | 'sponsorBlock'
export type DownloadOptionState = Pick<Settings, DownloadOptionKey>

interface BuildDownloadOptionsInput {
    videoInfo: VideoInfo | null
    selectedSubtitleLangs: Set<string>
    subtitleLangs: string
    options: DownloadOptions
}

export function getDownloadDefaults(settings: Settings | null, profile: DownloadProfile | undefined) {
    const options: DownloadOptionState = {
        saveThumbnail: settings?.saveThumbnail ?? false,
        saveDescription: settings?.saveDescription ?? false,
        embedChapters: settings?.embedChapters ?? false,
        sponsorBlock: settings?.sponsorBlock ?? false,
        writeSubtitles: profile?.writeSubtitles ?? settings?.writeSubtitles ?? false,
        embedSubtitles: profile?.embedSubtitles ?? settings?.embedSubtitles ?? false,
    }
    return {
        quality: profile?.quality || settings?.quality || 'best',
        outputDir: profile?.outputDir || settings?.outputDir || '',
        filenameTemplate: profile ? profile.filenameTemplate || DEFAULT_FILENAME_TEMPLATE : settings?.filenameTemplate || '',
        subtitleLangs: profile ? profile.subtitleLangs || 'all,-live_chat' : settings?.subtitleLangs || '',
        videoContainer: profile ? profile.videoContainer || 'auto' : undefined,
        audioFormat: profile ? profile.audioFormat || 'default' : undefined,
        options,
    }
}

export function isSingleDownloadLink(url: string): boolean {
    const authority = /^https?:\/\/([^/?#\s]+)/i.exec(url)?.[1]
    if (!authority || authority.includes('@') || /\s|\\/.test(url)) return false
    try {
        return Boolean(new URL(url).hostname)
    } catch {
        return false
    }
}

export function isAudioDownload(quality: string): boolean {
    return quality === 'audio' || quality.startsWith('fa:')
}

export function getSubtitleSelectionKey(sub: {code: string, auto?: boolean, selector?: string}): string {
    return sub.selector || `${sub.auto ? 'auto' : 'manual'}:${sub.code}`
}

export function splitSelectedSubtitleLangs(subtitles: VideoInfo['subtitles'] | undefined, selectedKeys: Set<string>) {
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

export function parseResolutionHeight(res: string): number {
    const m = res.match(/(\d+)p/)
    if (m) return parseInt(m[1], 10)
    const m2 = res.match(/(\d+)x(\d+)/)
    if (m2) return parseInt(m2[2], 10)
    return 0
}

export function formatOptionLabel(format: FormatInfo['formats'][number]): string {
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

export function sortFormats(formats: FormatInfo['formats'][number][]): FormatInfo['formats'][number][] {
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

export function findFormatByID(formats: FormatInfo['formats'], formatId: string): FormatInfo['formats'][number] | undefined {
    return formats.find(format => format.formatId === formatId)
}

export function resolveDownloadQuality({
    formatMode,
    selectedFormat,
    selectedVideoFormat,
    selectedAudioFormat,
    formatInfo,
    quality,
}: ResolveDownloadQualityInput): string {
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

export function buildDownloadOptions({
    videoInfo,
    selectedSubtitleLangs,
    subtitleLangs,
    options,
}: BuildDownloadOptionsInput): DownloadOptions {
    const {manualLangs, autoLangs} = splitSelectedSubtitleLangs(videoInfo?.subtitles, selectedSubtitleLangs)
    const hasExplicitSubtitleSelection = manualLangs.length > 0 || autoLangs.length > 0
    return {
        ...options,
        writeManualSubs: hasExplicitSubtitleSelection ? manualLangs.length > 0 : undefined,
        writeAutoSubs: hasExplicitSubtitleSelection ? autoLangs.length > 0 : undefined,
        subtitleLangs: hasExplicitSubtitleSelection ? manualLangs.join(',') : subtitleLangs,
        autoSubtitleLangs: hasExplicitSubtitleSelection ? autoLangs.join(',') : subtitleLangs,
    }
}
