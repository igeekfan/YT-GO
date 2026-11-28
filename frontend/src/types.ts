import type * as desktop from '../bindings/YT-GO/desktop/models'

export type {
    DownloadOptions, DownloadRequest, DownloadTask, DownloadURLReview, Format, SubtitleLang, YtDlpStatus,
} from '../bindings/YT-GO/desktop/models'

export type VideoInfo = Omit<desktop.VideoInfo, 'subtitles'> & {subtitles: desktop.SubtitleLang[]}
export type PlaylistInfo = Omit<desktop.PlaylistInfo, 'videos'> & {videos: desktop.VideoInfo[]}
export type FormatInfo = Omit<desktop.FormatInfo, 'formats'> & {formats: desktop.Format[]}
export type Settings = Omit<desktop.Settings, 'updateProxy'> & {updateProxy?: string}
