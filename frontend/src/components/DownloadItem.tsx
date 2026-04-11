import {useState, useEffect, useRef} from 'react'
import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'
import {OpenFile, OpenFolder, CancelDownload} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

interface Props {
    task: DownloadTask
    onCancelled: (id: string) => void
    onRetry: (task: DownloadTask) => void
    onRedownload: (task: DownloadTask) => void
}

function formatDuration(seconds: number): string {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = Math.floor(seconds % 60)
    if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    return `${m}:${String(s).padStart(2, '0')}`
}

const STATUS_COLORS: Record<string, string> = {
    pending: 'bg-yellow-500/20 text-yellow-400',
    downloading: 'bg-blue-500/20 text-blue-400',
    completed: 'bg-green-500/20 text-green-400',
    error: 'bg-red-500/20 text-red-400',
    cancelled: 'bg-gray-500/20 text-gray-400',
}

function DownloadItem({task, onCancelled, onRetry, onRedownload}: Props) {
    const {t} = useI18n()
    const [showLogs, setShowLogs] = useState(false)
    const [logs, setLogs] = useState<string[]>([])
    const logEndRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        const off = EventsOn('download:log', (data: {taskId: string; line: string}) => {
            if (data.taskId === task.id) {
                setLogs(prev => {
                    const next = [...prev, data.line]
                    // Keep last 200 lines to avoid memory issues
                    return next.length > 200 ? next.slice(-200) : next
                })
            }
        })
        return () => { if (typeof off === 'function') off() }
    }, [task.id])



    const handleCancel = async () => {
        try {
            await CancelDownload(task.id)
        } catch {
            // already cancelled or completed
        }
        onCancelled(task.id)
    }

    const handleOpenFile = () => {
        if (task.outputPath) {
            OpenFile(task.outputPath).catch(console.error)
        }
    }

    const handleOpenFolder = () => {
        const dir = task.outputPath
            ? task.outputPath.substring(0, Math.max(task.outputPath.lastIndexOf('/'), task.outputPath.lastIndexOf('\\')))
            : task.outputDir
        if (dir) OpenFolder(dir).catch(console.error)
    }

    const statusLabel = t(`status.${task.status}` as any)
    const statusColor = STATUS_COLORS[task.status] || STATUS_COLORS.pending

    return (
        <div className="download-item">
            {task.thumbnail && (
                <img
                    src={task.thumbnail}
                    alt=""
                    className="download-thumb"
                    onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                />
            )}
            {!task.thumbnail && (
                <div className="download-thumb-placeholder">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <polygon points="5 3 19 12 5 21 5 3"/>
                    </svg>
                </div>
            )}
            <div className="download-info">
                <div className="download-title" title={task.url}>
                    {task.title || task.url}
                </div>
                {task.status === 'downloading' && (
                    <div className="download-progress-wrap">
                        <div className="download-progress-bar">
                            <div
                                className="download-progress-fill"
                                style={{width: `${task.progress || 0}%`}}
                            />
                        </div>
                        <span className="download-progress-text">
                            {(task.progress || 0).toFixed(1)}%
                            {task.speed && ` · ${task.speed}`}
                            {task.eta && ` · ETA ${task.eta}`}
                            {task.size && ` · ${task.size}`}
                        </span>
                    </div>
                )}
                {task.status === 'error' && task.error && (
                    <div className="download-error">{task.error}</div>
                )}
                {task.status === 'completed' && task.outputPath && (
                    <div className="download-path" title={task.outputPath}>
                        {task.outputPath}
                    </div>
                )}
            </div>
            <div className="download-actions">
                <span className={`status-badge ${statusColor}`}>{statusLabel}</span>
                {(task.status === 'downloading' || logs.length > 0) && (
                    <button
                        className="btn-ghost btn-sm"
                        onClick={() => setShowLogs(!showLogs)}
                        title={showLogs ? t('action.hideLogs') : t('action.showLogs')}
                    >
                        {showLogs ? '▼' : '▶'} {t('action.logs')}
                    </button>
                )}
                {task.status === 'downloading' || task.status === 'pending' ? (
                    <button className="btn-ghost btn-sm" onClick={handleCancel}>
                        {t('action.cancel')}
                    </button>
                ) : null}
                {(task.status === 'error' || task.status === 'cancelled') && (
                    <button className="btn-ghost btn-sm" onClick={() => onRetry(task)}>
                        {t('action.retry')}
                    </button>
                )}
                {task.status === 'completed' && (
                    <>
                        <button className="btn-ghost btn-sm" onClick={handleOpenFile}>
                            {t('action.open')}
                        </button>
                        <button className="btn-ghost btn-sm" onClick={handleOpenFolder}>
                            {t('action.openFolder')}
                        </button>
                        <button className="btn-ghost btn-sm" onClick={() => onRedownload(task)}>
                            {t('action.redownload')}
                        </button>
                    </>
                )}
            </div>
            {showLogs && logs.length > 0 && (
                <div className="download-logs">
                    <pre className="download-logs-content">
                        {logs.map((line, i) => (
                            <div key={i} className="log-line">{line}</div>
                        ))}
                        <div ref={logEndRef} />
                    </pre>
                </div>
            )}
        </div>
    )
}

export default DownloadItem