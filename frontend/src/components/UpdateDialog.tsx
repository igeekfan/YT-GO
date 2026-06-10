import {useI18n} from '../i18n/context'
import {Button} from '@/components/ui/button'
import {RefreshCw, Download, AlertTriangle, X} from 'lucide-react'

interface UpdateInfo {
    hasUpdate: boolean; currentVersion: string; latestVersion: string
    releaseName: string; releaseBody: string; htmlUrl: string; publishedAt: string
}

interface UpdateDialogProps {
    open: boolean; updateInfo: UpdateInfo | null; loading?: boolean; error?: string | null
    onClose: () => void; onOpenReleasePage: () => void; onCheckUpdate: () => void
}

function UpdateDialog({open, updateInfo, loading, error, onClose, onOpenReleasePage, onCheckUpdate}: UpdateDialogProps) {
    const {t} = useI18n()

    if (!open) return null

    if (loading) {
        return (
            <div className="flex items-center gap-2 rounded-xl border bg-card/70 backdrop-blur-sm p-3 text-sm shadow-sm">
                <RefreshCw className="h-4 w-4 animate-spin text-muted-foreground" />
                <span className="text-muted-foreground">{t('update.checking')}</span>
            </div>
        )
    }

    if (error) {
        return (
            <div className="flex items-center gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-3 text-sm shadow-sm">
                <AlertTriangle className="h-4 w-4 text-destructive shrink-0" />
                <span className="flex-1 text-destructive text-xs">{error}</span>
                <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={onCheckUpdate}>{t('update.retry')}</Button>
                <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}><X className="h-3 w-3" /></Button>
            </div>
        )
    }

    if (updateInfo && !updateInfo.hasUpdate) return null

    if (updateInfo && updateInfo.hasUpdate) {
        return (
            <div className="flex items-center gap-3 rounded-xl border bg-primary/5 border-primary/20 p-3 shadow-sm">
                <div className="flex-1 flex items-center gap-2 text-sm">
                    <span>⬆️</span>
                    <span>{t('update.available', {version: updateInfo.latestVersion})}</span>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={onClose}>{t('update.later')}</Button>
                    <Button size="sm" className="h-7 text-xs" onClick={() => { onOpenReleasePage(); onClose() }}>
                        <Download className="h-3 w-3 mr-1" />{t('update.download')}
                    </Button>
                </div>
            </div>
        )
    }

    return null
}

export default UpdateDialog
