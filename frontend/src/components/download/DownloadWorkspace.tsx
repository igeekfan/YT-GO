import {lazy, Suspense} from 'react'
import {backendMode, getWebConfig} from '../../lib/backend'
import {MediaDownloadController} from '../../hooks/useMediaDownload'
import {SettingsController} from '../../hooks/useSettingsController'
import {YtDlpStatus} from '../../types'
import {useI18n} from '../../i18n/context'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {Download, FolderOpen, RefreshCw, Search, X} from 'lucide-react'

const MediaSummary = lazy(() => import('./MediaSummary'))
const FormatSelector = lazy(() => import('./FormatSelector'))
const DownloadOptionsPanel = lazy(() => import('./DownloadOptionsPanel'))
const YtDlpInstallGuide = lazy(() => import('./YtDlpInstallGuide'))

interface Props {
    ytdlp: YtDlpStatus | null
    installingYtDlp: boolean
    settings: SettingsController
    media: MediaDownloadController
    onInstallYtDlp: () => void
    onRecheckYtDlp: () => void
    onBrowseDirectory: () => void
    onOpenSettings: () => void
}

export default function DownloadWorkspace({
    ytdlp,
    installingYtDlp,
    settings,
    media,
    onInstallYtDlp,
    onRecheckYtDlp,
    onBrowseDirectory,
    onOpenSettings,
}: Props) {
    const {t} = useI18n()
    const hasResolvedMedia = !!(media.videoInfo || media.playlistInfo)

    return (
        <>
            {ytdlp && !ytdlp.available && (
                <Suspense fallback={null}>
                    <YtDlpInstallGuide installing={installingYtDlp} onInstall={onInstallYtDlp} onRecheck={onRecheckYtDlp} />
                </Suspense>
            )}

            {ytdlp && (
                <div className="flex gap-2.5 items-center animate-fade-in-up">
                    <div className="relative flex-1">
                        <Input
                            type="text"
                            value={media.url}
                            onChange={event => media.handleURLChange(event.target.value)}
                            onKeyDown={event => { if (event.key === 'Enter') void media.getInfo() }}
                            placeholder={t('url.placeholder')}
                            disabled={media.isGettingInfo}
                            className="h-10 bg-card/60 pr-8 text-sm backdrop-blur-xs"
                        />
                        {media.url && (
                            <Button variant="ghost" size="icon" className="absolute right-1 top-1/2 -translate-y-1/2 h-6 w-6 text-muted-foreground hover:text-foreground" onClick={media.clearCurrentInput} aria-label={t('action.clear')}>
                                <X className="h-3 w-3" />
                            </Button>
                        )}
                    </div>
                    <Button onClick={media.getInfo} disabled={media.isGettingInfo || !media.url.trim()} className="h-10 px-5 font-semibold shadow-xs hover:shadow-md transition-shadow">
                        {media.isGettingInfo ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Search className="h-4 w-4 mr-2" />}
                        {media.isGettingInfo ? t('url.gettingInfo') : t('url.getInfo')}
                    </Button>
                </div>
            )}

            {hasResolvedMedia && (
                <Suspense fallback={null}>
                    <MediaSummary media={media} />
                </Suspense>
            )}

            {hasResolvedMedia && (
                <div className="rounded-xl border bg-card/70 backdrop-blur-xs p-4 space-y-3.5 shadow-xs animate-fade-in-up delay-2">
                    {!(backendMode === 'web' && getWebConfig()?.hasFixedDir) && (
                        <div className="space-y-1.5">
                            <Label className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium">{t('outputDir.label')}</Label>
                            <div className="flex gap-2">
                                <Input
                                    type="text"
                                    value={settings.outputDir}
                                    onChange={event => settings.setOutputDir(event.target.value)}
                                    placeholder={backendMode === 'web' ? t('outputDir.serverPathPlaceholder') : t('outputDir.placeholder')}
                                />
                                <Button variant="outline" size="sm" onClick={backendMode === 'desktop' ? media.selectFolder : onBrowseDirectory}>
                                    <FolderOpen className="h-4 w-4 mr-1" />{t('outputDir.browse')}
                                </Button>
                            </div>
                        </div>
                    )}

                    {media.videoInfo && (
                        <Suspense fallback={null}>
                            <FormatSelector media={media} />
                            <DownloadOptionsPanel media={media} settings={settings} />
                        </Suspense>
                    )}

                    <div className="flex gap-2.5 flex-wrap items-center pt-1">
                        {backendMode === 'web' && !getWebConfig()?.hasFixedDir && !settings.outputDir && (
                            <div className="flex items-center gap-2 mr-auto text-yellow-500 text-sm">
                                <span>{t('web.noDirHint')}</span>
                                <Button variant="outline" size="sm" onClick={onOpenSettings}>{t('web.openSettings')}</Button>
                            </div>
                        )}
                        <Button onClick={media.download} disabled={media.isStarting || !media.url.trim() || !settings.outputDir} className="ml-auto h-10 px-6 font-semibold shadow-xs hover:shadow-md glow-primary transition-all duration-200">
                            {media.isStarting ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
                            {media.isStarting ? t('download.downloading') : t('download.start')}
                        </Button>
                        {media.playlistInfo && media.playlistInfo.count > 0 && (
                            <Button onClick={media.downloadAll} disabled={media.isStarting || !settings.outputDir || media.selectedPlaylistItems.size === 0}>
                                {media.isStarting ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
                                {media.isStarting ? t('playlist.startingAll') : `${t(`collection.${media.collectionKind}.downloadAll` as any)} (${media.selectedPlaylistItems.size})`}
                            </Button>
                        )}
                    </div>
                </div>
            )}
        </>
    )
}
