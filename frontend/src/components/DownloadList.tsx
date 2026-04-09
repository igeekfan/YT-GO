import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'
import DownloadItem from './DownloadItem'
import {ClearCompleted} from '../../wailsjs/go/main/App'

interface Props {
    downloads: DownloadTask[]
    onUpdate: (tasks: DownloadTask[]) => void
}

function DownloadList({downloads, onUpdate}: Props) {
    const {t} = useI18n()

    const hasCompleted = downloads.some(
        d => d.status === 'completed' || d.status === 'error' || d.status === 'cancelled'
    )

    const handleClear = async () => {
        await ClearCompleted()
        onUpdate(downloads.filter(
            d => d.status !== 'completed' && d.status !== 'error' && d.status !== 'cancelled'
        ))
    }

    const handleCancelled = (id: string) => {
        onUpdate(downloads.map(d => d.id === id ? {...d, status: 'cancelled'} : d))
    }

    const sorted = [...downloads].sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
    )

    return (
        <section className="downloads-section">
            <div className="downloads-header">
                <h2 className="section-title">{t('downloads.title')}</h2>
                {hasCompleted && (
                    <button className="btn-ghost btn-sm" onClick={handleClear}>
                        {t('downloads.clearCompleted')}
                    </button>
                )}
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
                        />
                    ))}
                </div>
            )}
        </section>
    )
}

export default DownloadList