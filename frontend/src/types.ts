import {desktop} from '../wailsjs/go/models'

export type YtDlpStatus = desktop.YtDlpStatus
export type VideoInfo = desktop.VideoInfo
export type DownloadRequest = desktop.DownloadRequest
export type DownloadTask = desktop.DownloadTask
export type DownloadOptions = desktop.DownloadOptions & {
	writeManualSubs?: boolean
	writeAutoSubs?: boolean
	autoSubtitleLangs?: string
}
export type SubtitleLang = desktop.SubtitleLang & {
	selector?: string
}
export type PlaylistInfo = desktop.PlaylistInfo
export type Settings = desktop.Settings
export type Format = desktop.Format
export type FormatInfo = desktop.FormatInfo