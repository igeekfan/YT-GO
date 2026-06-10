import {useState, useEffect, useRef} from 'react'
import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'
import {OpenFile, OpenFolder, CancelDownload, RemoveDownload, backendMode, getDownloadFileURL} from '../lib/backend'
import {EventsOn} from '../lib/runtime'
import {formatDuration} from '../lib/formatUtils'
import {Button} from '@/components/ui/button'
import {Badge} from '@/components/ui/badge'
import {Progress} from '@/components/ui/progress'
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible'
import {Play, X, RotateCcw, FolderOpen, FileDown, ChevronDown, ChevronRight} from 'lucide-react'

interface Props {
    task: DownloadTask
    onCancelled: (id: string) => void
    onRemoved: (id: string) => void
    onRetry: (task: DownloadTask) => void
    onRedownload: (task: DownloadTask) => void
}

const STATUS_VARIANT: Record<string, 'secondary' | 'default' | 'destructive' | 'outline'> = {
    pending: 'outline', downloading: 'default', completed: 'secondary', error: 'destructive', cancelled: 'outline',
}

function DownloadItem({task, onCancelled, onRemoved, onRetry, onRedownload}: Props) {
    const {t} = useI18n()
    const [showLogs, setShowLogs] = useState(false)
    const [logs, setLogs] = useState<string[]>([])
    const [latestProgressLine, setLatestProgressLine] = useState('')
    const logEndRef = useRef<HTMLDivElement>(null)
    const isDesktop = backendMode === 'desktop'

    useEffect(() => {
        const off = EventsOn('download:log', (data: {taskId: string; line: string}) => {
            if (data.taskId === task.id) {
                setLogs(prev => { const next = [...prev, data.line]; return next.length > 200 ? next.slice(-200) : next })
                if (data.line.includes('[download]')) {
                    setLatestProgressLine(data.line)
                }
            }
        })
        return () => { if (typeof off === 'function') off() }
    }, [task.id])

    useEffect(() => {
        if (task.status !== 'downloading') {
            setLatestProgressLine('')
        }
    }, [task.status])

    const handleCancel = async () => {
        try { await CancelDownload(task.id) } catch { /* already cancelled */ }
        onCancelled(task.id)
    }

    const handleRemove = async () => {
        try { await RemoveDownload(task.id); onRemoved(task.id) }
        catch (error) { console.error(error) }
    }

    return (
        <div className="rounded-xl border p-3 space-y-2 bg-card/50 backdrop-blur-sm shadow-sm hover:shadow-md transition-shadow duration-200">
            {/* Top row: thumbnail + info + actions */}
            <div className="flex items-start gap-3">
                {/* Thumbnail */}
                {task.thumbnail ? (
                    <img src={task.thumbnail} alt="" className="w-20 h-[45px] object-cover rounded-lg shrink-0 shadow-sm"
                        onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
                ) : (
                    <div className="w-20 h-[45px] bg-muted rounded-lg shrink-0 flex items-center justify-center text-muted-foreground">
                        <Play className="h-4 w-4" />
                    </div>
                )}

                {/* Info */}
                <div className="flex-1 min-w-0 space-y-0.5">
                    <div className="text-sm font-medium truncate leading-snug" title={task.url}>{task.title || task.url}</div>
                    {task.status === 'downloading' && (
                        <div className="space-y-0.5">
                            <Progress value={task.progress || 0} className="h-1" />
                            <span className="text-[11px] text-muted-foreground">
                                {(task.progress || 0).toFixed(1)}%
                                {task.speed && ` · ${task.speed}`}
                                {task.eta && ` · ETA ${task.eta}`}
                                {task.size && ` · ${task.size}`}
                            </span>
                            {!task.speed && latestProgressLine && (
                                <div className="text-[11px] text-muted-foreground truncate" title={latestProgressLine}>{latestProgressLine}</div>
                            )}
                        </div>
                    )}
                    {task.status === 'error' && task.error && (
                        <div className="text-[11px] text-destructive truncate">{task.error}</div>
                    )}
                    {task.status === 'completed' && task.outputPath && (
                        <div className="text-[11px] text-muted-foreground truncate" title={task.outputPath}>{task.outputPath}</div>
                    )}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-1 shrink-0 flex-wrap">
                    <Badge variant={STATUS_VARIANT[task.status] || 'outline'} className="text-[10px] h-5 px-1.5">
                        {t(`status.${task.status}` as any)}
                    </Badge>
                    {(task.status === 'downloading' || logs.length > 0) && (
                        <Collapsible open={showLogs} onOpenChange={setShowLogs}>
                            <CollapsibleTrigger asChild>
                                <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5">
                                    {showLogs ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                                    {t('action.logs')}
                                </Button>
                            </CollapsibleTrigger>
                        </Collapsible>
                    )}
                    {(task.status === 'downloading' || task.status === 'pending') && (
                        <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={handleCancel}>
                            <X className="h-3 w-3 mr-0.5" />{t('action.cancel')}
                        </Button>
                    )}
                    {(task.status === 'error' || task.status === 'cancelled') && (
                        <>
                            <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={() => onRetry(task)}>
                                <RotateCcw className="h-3 w-3 mr-0.5" />{t('action.retry')}
                            </Button>
                            <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={handleRemove}>
                                <X className="h-3 w-3 mr-0.5" />{t('action.remove')}
                            </Button>
                        </>
                    )}
                    {task.status === 'completed' && (
                        <>
                            {isDesktop ? (
                                <>
                                    <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={() => OpenFile(task.outputPath!).catch(console.error)}>
                                        <FileDown className="h-3 w-3 mr-0.5" />{t('action.open')}
                                    </Button>
                                    <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={() => OpenFolder(task.outputPath || task.outputDir).catch(console.error)}>
                                        <FolderOpen className="h-3 w-3 mr-0.5" />{t('action.openFolder')}
                                    </Button>
                                </>
                            ) : (
                                <a href={getDownloadFileURL(task.id)} download target="_blank" rel="noopener"
                                    className="inline-flex items-center gap-0.5 h-6 px-1.5 text-xs rounded-md hover:bg-accent hover:text-accent-foreground">
                                    <FileDown className="h-3 w-3" />{t('action.download')}
                                </a>
                            )}
                            <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={() => onRedownload(task)}>
                                <RotateCcw className="h-3 w-3 mr-0.5" />{t('action.redownload')}
                            </Button>
                            <Button variant="ghost" size="sm" className="h-6 text-xs px-1.5" onClick={handleRemove}>
                                <X className="h-3 w-3 mr-0.5" />{t('action.remove')}
                            </Button>
                        </>
                    )}
                </div>
            </div>

            {/* Logs - full width below the top row */}
            <Collapsible open={showLogs} onOpenChange={setShowLogs}>
                <CollapsibleContent>
                    {showLogs && logs.length > 0 && (
                        <div className="max-h-48 overflow-y-auto overflow-x-hidden rounded-lg border bg-muted/30">
                            <pre className="p-2.5 text-[11px] font-mono leading-snug whitespace-pre-wrap break-words w-full">
                                {logs.map((line, i) => <div key={i}>{line}</div>)}
                                <div ref={logEndRef} />
                            </pre>
                        </div>
                    )}
                </CollapsibleContent>
            </Collapsible>
        </div>
    )
}

export default DownloadItem
