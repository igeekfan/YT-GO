import {useState} from 'react'
import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'
import DownloadItem from './DownloadItem'
import {ClearCompleted, StartDownload} from '../../wailsjs/go/main/App'

interface Props {
    downloads: DownloadTask[]
    onUpdate: (tasks: DownloadTask[]) => void
}

type StatusFilter = 'all' | 'downloading' | 'completed' | 'error'

function DownloadList({downloads, onUpdate}: Props) {
    const {t} = useI18n()
    const [searchQuery, setSearchQuery] = useState('')
    const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')

    const hasCompleted = downloads.some(
        d => d.status === 'completed' || d.status === 'error' || d.status === 'cancelled'
    )
    const retryable = downloads.filter(d => d.status === 'error' || d.status === 'cancelled')

    const handleRetryTask = async (task: DownloadTask) => {
        await StartDownload({
            url: task.url,
            outputDir: task.outputDir,
            quality: task.quality || 'best',
            videoInfo: task.title || task.thumbnail ? {
                url: task.url,
                title: task.title,
                thumbnail: task.thumbnail,
            } : undefined,
        } as any)
    }

    const handleRetryAll = async () => {
        for (const task of retryable) {
            await handleRetryTask(task)
        }
    }

    const handleClear = async () => {
        await ClearCompleted()
        onUpdate(downloads.filter(
            d => d.status !== 'completed' && d.status !== 'error' && d.status !== 'cancelled'
        ))
    }

    const handleCancelled = (id: string) => {
        onUpdate(downloads.filter(d => d.id !== id))
    }

    const filtered = downloads.filter(d => {
        if (statusFilter !== 'all') {
            if (statusFilter === 'downloading' && d.status !== 'downloading' && d.status !== 'pending') return false
            if (statusFilter === 'completed' && d.status !== 'completed') return false
            if (statusFilter === 'error' && d.status !== 'error' && d.status !== 'cancelled') return false
        }
        if (searchQuery.trim()) {
            const q = searchQuery.toLowerCase()
            const title = (d.title || '').toLowerCase()
            const url = (d.url || '').toLowerCase()
            if (!title.includes(q) && !url.includes(q)) return false
        }
        return true
    })

    const sorted = [...filtered].sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
    )

    return (
        <section className="downloads-section">
            <div className="downloads-header">
                <h2 className="section-title">{t('downloads.title')}</h2>
                <div className="downloads-header-actions">
                    {retryable.length > 0 && (
                        <button className="btn-ghost btn-sm" onClick={handleRetryAll}>
                            {t('downloads.retryFailed')}
                        </button>
                    )}
                    {hasCompleted && (
                        <button className="btn-ghost btn-sm" onClick={handleClear}>
                            {t('downloads.clearCompleted')}
                        </button>
                    )}
                </div>
            </div>
            {downloads.length > 0 && (
                <div className="downloads-filter-row">
                    <input
                        type="text"
                        className="downloads-search"
                        value={searchQuery}
                        onChange={e => setSearchQuery(e.target.value)}
                        placeholder={t('downloads.search')}
                    />
                    <select
                        className="downloads-status-filter"
                        value={statusFilter}
                        onChange={e => setStatusFilter(e.target.value as StatusFilter)}
                    >
                        <option value="all">{t('downloads.filterAll')}</option>
                        <option value="downloading">{t('downloads.filterDownloading')}</option>
                        <option value="completed">{t('downloads.filterCompleted')}</option>
                        <option value="error">{t('downloads.filterError')}</option>
                    </select>
                </div>
            )}
            {sorted.length === 0 ? (
                <div className="downloads-empty">{t('downloads.empty')}</div>
            ) : (
                <div className="download-list">
                    {sorted.map(task => (
                        <DownloadItem
                            key={task.id}
                            task={task}
                            onCancelled={handleCancelled}
                            onRetry={handleRetryTask}
                            onRedownload={handleRetryTask}
                        />
                    ))}
                </div>
            )}
        </section>
    )
}

export default DownloadList