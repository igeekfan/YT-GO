import {useState} from 'react'
import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'
import DownloadItem from './DownloadItem'
import {ClearCompleted, RetryDownload} from '../lib/backend'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'
import {Search, Trash2, RefreshCw} from 'lucide-react'
import {toast} from 'sonner'

interface Props {
    downloads: DownloadTask[]
    onUpdate: (tasks: DownloadTask[]) => void
}

type StatusFilter = 'all' | 'downloading' | 'completed' | 'error'

function DownloadList({downloads, onUpdate}: Props) {
    const {t} = useI18n()
    const [searchQuery, setSearchQuery] = useState('')
    const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
    const [isRetrying, setIsRetrying] = useState(false)

    const hasCompleted = downloads.some(d => d.status === 'completed' || d.status === 'error' || d.status === 'cancelled')
    const retryable = downloads.filter(d => d.status === 'error' || d.status === 'cancelled')

    const retryTasks = async (tasks: DownloadTask[]) => {
        if (isRetrying) return
        setIsRetrying(true)
        let queued = 0
        const failures: string[] = []
        try {
            for (const task of tasks) {
                try {
                    await RetryDownload(task.id)
                    queued++
                } catch (error) {
                    failures.push(error instanceof Error ? error.message : String(error))
                }
            }
        } finally {
            setIsRetrying(false)
        }
        if (queued === 1) toast.success(t('toast.downloadQueued'))
        if (queued > 1) toast.success(t('toast.downloadQueuedCount', {count: queued}))
        if (failures.length > 0) toast.error(`${t('toast.downloadRetryFail', {count: failures.length})} · ${failures[0]}`)
    }

    const handleRetryTask = (task: DownloadTask) => { void retryTasks([task]) }

    const handleClear = async () => {
        await ClearCompleted()
        onUpdate(downloads.filter(d => d.status !== 'completed' && d.status !== 'error' && d.status !== 'cancelled'))
    }

    const handleCancelled = (id: string) => { onUpdate(downloads.filter(d => d.id !== id)) }
    const handleRemoved = (id: string) => { onUpdate(downloads.filter(d => d.id !== id)) }

    const filtered = downloads.filter(d => {
        if (statusFilter !== 'all') {
            if (statusFilter === 'downloading' && d.status !== 'downloading' && d.status !== 'pending') return false
            if (statusFilter === 'completed' && d.status !== 'completed') return false
            if (statusFilter === 'error' && d.status !== 'error' && d.status !== 'cancelled') return false
        }
        if (searchQuery.trim()) {
            const q = searchQuery.toLowerCase()
            if (!(d.title || '').toLowerCase().includes(q) && !(d.url || '').toLowerCase().includes(q)) return false
        }
        return true
    })

    const sorted = [...filtered].sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())

    return (
        <section className="rounded-xl border bg-card/70 backdrop-blur-xs shadow-xs">
            <div className="flex items-center justify-between gap-3 px-3.5 py-2.5 flex-wrap">
                <h2 className="text-sm font-semibold">{t('downloads.title')}</h2>
                <div className="flex items-center gap-1">
                    {retryable.length > 0 && (
                        <Button variant="ghost" size="sm" onClick={() => retryTasks(retryable)} disabled={isRetrying} className="text-xs h-7 px-2">
                            <RefreshCw className="h-3 w-3 mr-1" />{t('downloads.retryFailed')}
                        </Button>
                    )}
                    {hasCompleted && (
                        <Button variant="ghost" size="sm" onClick={handleClear} className="text-xs h-7 px-2">
                            <Trash2 className="h-3 w-3 mr-1" />{t('downloads.clearCompleted')}
                        </Button>
                    )}
                </div>
            </div>
            {downloads.length > 0 && (
                <div className="flex gap-2 px-3 pb-2.5">
                    <div className="relative flex-1">
                        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
                        <Input type="text" value={searchQuery} onChange={e => setSearchQuery(e.target.value)}
                            placeholder={t('downloads.search')} className="pl-8 h-8 text-xs" />
                    </div>
                    <Select value={statusFilter} onValueChange={(v: string) => setStatusFilter(v as StatusFilter)}>
                        <SelectTrigger className="w-28 h-8 text-xs"><SelectValue /></SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t('downloads.filterAll')}</SelectItem>
                            <SelectItem value="downloading">{t('downloads.filterDownloading')}</SelectItem>
                            <SelectItem value="completed">{t('downloads.filterCompleted')}</SelectItem>
                            <SelectItem value="error">{t('downloads.filterError')}</SelectItem>
                        </SelectContent>
                    </Select>
                </div>
            )}
            {sorted.length === 0 ? (
                <div className="text-center text-muted-foreground py-8 text-sm">{t('downloads.empty')}</div>
            ) : (
                <div className="space-y-1.5 px-3 pb-3">
                    {sorted.map(task => (
                        <DownloadItem key={task.id} task={task} onCancelled={handleCancelled} onRemoved={handleRemoved}
                            onRetry={handleRetryTask} onRedownload={handleRetryTask} retryDisabled={isRetrying} />
                    ))}
                </div>
            )}
        </section>
    )
}

export default DownloadList
