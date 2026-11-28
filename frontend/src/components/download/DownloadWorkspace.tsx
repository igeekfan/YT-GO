import {lazy, Suspense} from 'react'
import {backendMode, getWebConfig} from '../../lib/backend'
import {MediaDownloadController} from '../../hooks/useMediaDownload'
import {SettingsController} from '../../hooks/useSettingsController'
import {YtDlpStatus} from '../../types'
import {useI18n} from '../../i18n/context'
import DownloadProfileSelect from './DownloadProfileSelect'
import {Button} from '@/components/ui/button'
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible'
import {Input} from '@/components/ui/input'
import {ChevronDown, Download, RefreshCw, Search, X} from 'lucide-react'
import {toast} from 'sonner'

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
    const hasFixedDir = getWebConfig()?.hasFixedDir ?? false
    const canSubmit = (hasFixedDir || !!media.outputDir) && !media.isStarting && !settings.savingDownloadProfiles

    const selectProfile = async (selectedId: string) => {
        try {
            await settings.saveDownloadProfiles({...settings.downloadProfiles, selectedId})
        } catch (error) {
            toast.error(t('settings.saveFailed'), {description: error instanceof Error ? error.message : String(error)})
        }
    }

    return (
        <>
            {ytdlp && !ytdlp.available && (
                <Suspense fallback={null}>
                    <YtDlpInstallGuide installing={installingYtDlp} onInstall={onInstallYtDlp} onRecheck={onRecheckYtDlp} />
                </Suspense>
            )}

            {ytdlp && (
                <div className="flex flex-wrap gap-2.5 items-center animate-fade-in-up">
                    <div className="relative w-full sm:min-w-44 sm:flex-1">
                        <Input
                            type="text"
                            inputMode="url"
                            autoCapitalize="none"
                            spellCheck={false}
                            aria-label={t('download.link')}
                            value={media.url}
                            onChange={event => media.handleURLChange(event.target.value)}
                            onKeyDown={event => {
                                if (event.key === 'Enter' && !event.nativeEvent.isComposing && !media.isStarting) void media.getInfo()
                            }}
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
                    <Button onClick={media.getInfo} disabled={media.isGettingInfo || media.isStarting || !media.url.trim()} className="h-10 px-4 font-semibold shadow-xs hover:shadow-md transition-shadow">
                        {media.isGettingInfo ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Search className="h-4 w-4 mr-2" />}
                        {media.isGettingInfo ? t('url.gettingInfo') : t('url.getInfo')}
                    </Button>
                    <Button variant="outline" onClick={media.quickDownload}
                        disabled={!canSubmit || !media.canQuickDownload || media.isGettingInfo || !!media.playlistInfo}>
                        <Download className="mr-2 h-4 w-4" />{t('download.quick')}
                    </Button>
                </div>
            )}

            {settings.currentSettings && (
                <div className="flex flex-wrap items-end gap-2.5">
                    <div className="w-full max-w-xs">
                        <DownloadProfileSelect profiles={settings.downloadProfiles}
                            disabled={settings.savingDownloadProfiles || media.isStarting}
                            onSelect={selectedId => void selectProfile(selectedId)} />
                    </div>
                    <Button variant="ghost" onClick={onOpenSettings}>{t('profile.manage')}</Button>
                </div>
            )}

            {hasResolvedMedia && (
                <Suspense fallback={null}>
                    <MediaSummary media={media} />
                </Suspense>
            )}

            {(hasResolvedMedia || media.canQuickDownload) && (
                <div className="rounded-xl border bg-card/70 backdrop-blur-xs p-4 space-y-3.5 shadow-xs animate-fade-in-up delay-2">
                    {media.videoInfo && (
                        <Suspense fallback={null}>
                            <FormatSelector media={media} />
                        </Suspense>
                    )}
                    <Collapsible key={hasResolvedMedia ? 'resolved' : 'quick'} defaultOpen={hasResolvedMedia || !canSubmit}>
                        <CollapsibleTrigger className="group flex w-full items-center gap-1.5 rounded-lg border px-3 py-2 text-left text-sm font-medium hover:bg-muted/40">
                            <ChevronDown aria-hidden="true" className="h-3.5 w-3.5 -rotate-90 group-data-[state=open]:rotate-0" />
                            {t('download.taskOptions')}
                        </CollapsibleTrigger>
                        <CollapsibleContent className="mt-2">
                            <Suspense fallback={null}>
                                <DownloadOptionsPanel media={media} hasFixedDir={hasFixedDir} onBrowseDirectory={onBrowseDirectory} />
                            </Suspense>
                        </CollapsibleContent>
                    </Collapsible>

                    {(hasResolvedMedia || (backendMode === 'web' && !hasFixedDir && !media.outputDir)) && (
                        <div className="flex gap-2.5 flex-wrap items-center pt-1">
                            {backendMode === 'web' && !hasFixedDir && !media.outputDir && (
                                <div className="flex items-center gap-2 mr-auto text-yellow-500 text-sm">
                                    <span>{t('web.noDirHint')}</span>
                                    <Button variant="outline" size="sm" onClick={onOpenSettings}>{t('web.openSettings')}</Button>
                                </div>
                            )}
                            {hasResolvedMedia && (
                                <Button onClick={media.download} disabled={!canSubmit || !media.url.trim()} className="ml-auto h-10 px-6 font-semibold shadow-xs hover:shadow-md glow-primary transition-all duration-200">
                                    {media.isStarting ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
                                    {media.isStarting ? t('download.downloading') : t('download.start')}
                                </Button>
                            )}
                            {media.playlistInfo && media.playlistInfo.count > 0 && (
                                <Button onClick={media.downloadAll} disabled={!canSubmit || media.selectedPlaylistItems.size === 0}>
                                    {media.isStarting ? <RefreshCw className="h-4 w-4 animate-spin mr-2" /> : <Download className="h-4 w-4 mr-2" />}
                                    {media.isStarting ? t('playlist.startingAll') : `${t(`collection.${media.collectionKind}.downloadAll`)} (${media.selectedPlaylistItems.size})`}
                                </Button>
                            )}
                        </div>
                    )}
                </div>
            )}
        </>
    )
}
