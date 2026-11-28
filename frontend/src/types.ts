import type * as desktop from '../bindings/YT-GO/desktop/models'

export type {
    DownloadOptions, DownloadRequest, DownloadTask, DownloadURLReview, Format,
    IncrementalDownloadRequest, IncrementalDownloadResult, SubtitleLang, YtDlpStatus,
} from '../bindings/YT-GO/desktop/models'
export type {DownloadProfile, DownloadProfiles} from '../bindings/YT-GO/internal/core/models'

export type VideoInfo = Omit<desktop.VideoInfo, 'subtitles'> & {subtitles: desktop.SubtitleLang[]}
export type PlaylistInfo = Omit<desktop.PlaylistInfo, 'videos'> & {videos: desktop.VideoInfo[]}
export type IncrementalPlaylistScan = Omit<desktop.IncrementalPlaylistScan, 'newVideos'> & {newVideos: desktop.YouTubePlaylistVideo[]}
export type FormatInfo = Omit<desktop.FormatInfo, 'formats'> & {formats: desktop.Format[]}
export type Settings = Omit<desktop.Settings, 'updateProxy'> & {updateProxy?: string}
