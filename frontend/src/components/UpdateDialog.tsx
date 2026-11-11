import {useI18n} from '../i18n/context'
import {backendMode} from '../lib/backend'
import {Button} from '@/components/ui/button'
import {AlertTriangle, ArrowUpRight, Download, ExternalLink, RefreshCw, X} from 'lucide-react'

interface UpdateInfo {
    hasUpdate: boolean
    currentVersion: string
    latestVersion: string
    releaseName: string
    releaseBody: string
    htmlUrl: string
    publishedAt: string
    canInstall?: boolean
}

interface UpdateDialogProps {
    open: boolean
    updateInfo: UpdateInfo | null
    loading?: boolean
    installing?: boolean
    error?: string | null
    onClose: () => void
    onOpenReleasePage: () => void
    onDownloadUpdate: () => void
    onCheckUpdate: () => void
}

function UpdateDialog({open, updateInfo, loading, installing, error, onClose, onOpenReleasePage, onDownloadUpdate, onCheckUpdate}: UpdateDialogProps) {
    const {t} = useI18n()

    if (!open) return null

    if (loading) {
        return (
            <div className="flex items-center gap-3 rounded-xl border bg-card/80 p-4 text-sm shadow-sm" role="status" aria-live="polite" aria-busy="true">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                    <RefreshCw className="h-4 w-4 animate-spin" />
                </div>
                <span className="min-w-0 flex-1 text-muted-foreground">{t('update.checking')}</span>
                <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={onClose} aria-label={t('action.close')}>
                    <X className="h-4 w-4" />
                </Button>
            </div>
        )
    }

    if (error) {
        return (
            <div className="flex flex-col gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm shadow-sm sm:flex-row sm:items-center" role="alert">
                <div className="flex min-w-0 flex-1 items-start gap-3">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-destructive/10 text-destructive">
                        <AlertTriangle className="h-4 w-4" />
                    </div>
                    <div className="min-w-0 space-y-0.5">
                        <p className="font-medium text-destructive">{t('update.error')}</p>
                        <p className="break-words text-xs text-destructive/80">{error}</p>
                    </div>
                </div>
                <div className="flex w-full shrink-0 items-center gap-2 sm:w-auto">
                    <Button variant="outline" size="sm" className="flex-1 border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive sm:flex-none" onClick={onCheckUpdate}>
                        {t('update.retry')}
                    </Button>
                    <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={onClose} aria-label={t('action.close')}>
                        <X className="h-4 w-4" />
                    </Button>
                </div>
            </div>
        )
    }

    if (updateInfo && !updateInfo.hasUpdate) return null

    if (updateInfo && updateInfo.hasUpdate) {
        const canInstall = backendMode === 'desktop' && updateInfo.canInstall !== false
        const actionLabel = !canInstall
            ? t('update.openRelease')
            : installing
                ? t('update.downloading')
                : t('update.download')

        return (
            <div className="relative overflow-hidden rounded-2xl border border-primary/25 bg-card/90 shadow-sm" role="status" aria-live="polite">
                <div className="absolute inset-y-0 left-0 w-1 bg-primary" aria-hidden="true" />
                <div className="flex flex-col gap-4 p-4 sm:flex-row sm:items-start sm:justify-between sm:p-5">
                    <div className="flex min-w-0 items-start gap-3">
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                            <ArrowUpRight className="h-5 w-5" aria-hidden="true" />
                        </div>
                        <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                                <h2 className="text-sm font-semibold leading-5">
                                    {t('update.available', {version: updateInfo.latestVersion})}
                                </h2>
                                {updateInfo.releaseName && (
                                    <span className="max-w-full truncate text-xs text-muted-foreground sm:max-w-[20rem]" title={updateInfo.releaseName}>
                                        {updateInfo.releaseName}
                                    </span>
                                )}
                            </div>
                            <div className="mt-3 grid max-w-sm grid-cols-2 gap-2">
                                <div className="rounded-lg border bg-background/70 px-3 py-2">
                                    <p className="text-[11px] text-muted-foreground">{t('update.currentVersion')}</p>
                                    <p className="mt-0.5 font-mono text-sm font-medium">v{updateInfo.currentVersion}</p>
                                </div>
                                <div className="rounded-lg border border-primary/20 bg-primary/5 px-3 py-2">
                                    <p className="text-[11px] text-primary/80">{t('update.latestVersion')}</p>
                                    <p className="mt-0.5 font-mono text-sm font-semibold text-primary">v{updateInfo.latestVersion}</p>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div className="flex w-full shrink-0 flex-col gap-2 sm:w-auto sm:flex-row sm:pt-0.5">
                        <Button variant="outline" size="sm" className="w-full sm:w-auto" onClick={onClose}>
                            {t('update.later')}
                        </Button>
                        <Button
                            size="sm"
                            className="w-full sm:w-auto"
                            disabled={installing}
                            onClick={() => {
                                if (!canInstall) {
                                    onOpenReleasePage()
                                    onClose()
                                } else {
                                    onDownloadUpdate()
                                }
                            }}
                        >
                            {installing ? <RefreshCw className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : !canInstall ? <ExternalLink className="mr-1.5 h-3.5 w-3.5" /> : <Download className="mr-1.5 h-3.5 w-3.5" />}
                            {actionLabel}
                        </Button>
                    </div>
                </div>
            </div>
        )
    }

    return null
}

export default UpdateDialog
