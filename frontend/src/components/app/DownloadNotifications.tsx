import {useEffect} from 'react'
import {CheckCircle2, X} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {useI18n} from '../../i18n/context'

interface DownloadNotification {
    id: string
    title: string
    body: string
}

interface Props {
    notifications: DownloadNotification[]
    onDismiss: (id: string) => void
}

function DownloadNotifications({notifications, onDismiss}: Props) {
    const {t} = useI18n()

    useEffect(() => {
        const timers = notifications.map(notification => window.setTimeout(() => onDismiss(notification.id), 7000))
        return () => timers.forEach(timer => window.clearTimeout(timer))
    }, [notifications, onDismiss])

    if (notifications.length === 0) return null

    return (
        <aside className="pointer-events-none fixed right-4 top-16 z-50 flex w-[min(22rem,calc(100vw-2rem))] flex-col gap-2" aria-label={t('notification.downloadComplete')} aria-live="polite">
            {notifications.map(notification => (
                <div key={notification.id} className="download-notification pointer-events-auto overflow-hidden rounded-lg border border-border bg-card text-card-foreground shadow-lg">
                    <div className="flex items-start gap-3 px-3.5 py-3">
                        <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-green-600 dark:text-green-400" aria-hidden="true" />
                        <div className="min-w-0 flex-1">
                            <p className="text-sm font-semibold">{notification.title}</p>
                            <p className="mt-0.5 line-clamp-2 break-words text-xs text-muted-foreground">{notification.body}</p>
                        </div>
                        <Button variant="ghost" size="icon" className="-mr-1 -mt-1 h-7 w-7 shrink-0 text-muted-foreground" onClick={() => onDismiss(notification.id)} aria-label={t('action.close')}>
                            <X className="h-3.5 w-3.5" />
                        </Button>
                    </div>
                    <div className="h-0.5 bg-primary/70" aria-hidden="true" />
                </div>
            ))}
        </aside>
    )
}

export default DownloadNotifications
