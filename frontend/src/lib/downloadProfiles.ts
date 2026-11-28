import type {DownloadProfile, DownloadProfiles, Settings} from '../types'

export const DOWNLOAD_QUALITIES = ['best', '1080p', '720p', '480p', '360p', 'audio'] as const
export const VIDEO_CONTAINERS = [['mp4', 'MP4'], ['mkv', 'MKV'], ['webm', 'WebM']] as const
export const AUDIO_FORMATS = [['mp3', 'MP3'], ['m4a', 'M4A'], ['opus', 'Opus'], ['flac', 'FLAC'], ['wav', 'WAV']] as const
export const DEFAULT_FILENAME_TEMPLATE = '%(title)s.%(ext)s'
export const DEFAULT_PROFILE_VALUE = ':default'
export const EMPTY_DOWNLOAD_PROFILES: DownloadProfiles = {items: [], selectedId: ''}

export function createDownloadProfile(settings: Settings): DownloadProfile {
    const bytes = crypto.getRandomValues(new Uint8Array(16))
    return {
        id: Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join(''),
        name: '',
        quality: DOWNLOAD_QUALITIES.find(quality => quality === settings.quality) || 'best',
        outputDir: '',
        filenameTemplate: settings.filenameTemplate,
        videoContainer: VIDEO_CONTAINERS.find(([value]) => value === settings.mergeOutputFormat)?.[0] || 'auto',
        audioFormat: AUDIO_FORMATS.find(([value]) => value === settings.audioFormat)?.[0] || 'default',
        writeSubtitles: settings.writeSubtitles,
        subtitleLangs: settings.subtitleLangs,
        embedSubtitles: settings.embedSubtitles,
    }
}
