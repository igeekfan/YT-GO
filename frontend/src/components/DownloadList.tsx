import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'
import DownloadItem from './DownloadItem'
import {ClearCompleted, StartDownload} from '../../wailsjs/go/main/App'

interface Props {
    downloads: DownloadTask[]
    onUpdate: (tasks: DownloadTask[]) => void
}

function DownloadList({downloads, onUpdate}: Props) {
    const {t} = useI18n()

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

    const sorted = [...downloads].sort(
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
                        />
                    ))}
                </div>
            )}
        </section>
    )
}

export default DownloadList