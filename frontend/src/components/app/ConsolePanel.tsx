import {useRef} from 'react'
import {useI18n} from '../../i18n/context'
import {Button} from '@/components/ui/button'
import {ChevronDown, ChevronRight} from 'lucide-react'

interface Props {
    logs: string[]
    open: boolean
    onToggle: () => void
    onClear: () => void
}

function getConsoleLogType(line: string): 'error' | 'warning' | 'command' | 'info' {
    const normalized = line.toLowerCase()
    if (normalized.includes(' failed:') || normalized.includes('error:')) return 'error'
    if (normalized.includes('warning:')) return 'warning'
    if (normalized.includes(' exec: ')) return 'command'
    return 'info'
}

export default function ConsolePanel({logs, open, onToggle, onClear}: Props) {
    const {t} = useI18n()
    const endRef = useRef<HTMLDivElement>(null)
    if (logs.length === 0) return null

    return (
        <div className="rounded-xl border overflow-hidden shadow-sm">
            <div className="flex items-center justify-between px-2.5 py-1.5 border-b bg-muted/40">
                <Button variant="ghost" size="sm" onClick={onToggle} className="text-xs h-7 px-1.5">
                    {open ? <ChevronDown className="h-3 w-3 mr-1" /> : <ChevronRight className="h-3 w-3 mr-1" />}
                    {t('console.title')} ({logs.length})
                </Button>
                <Button variant="ghost" size="sm" onClick={onClear} className="text-xs h-7 px-1.5">
                    {t('console.clear')}
                </Button>
            </div>
            {open && (
                <div className="max-h-72 overflow-y-auto overflow-x-hidden">
                    <pre className="bg-muted/20 p-2 text-[11px] font-mono leading-snug whitespace-pre-wrap break-words">
                        {logs.map((line, index) => (
                            <div key={index} className={`px-1.5 py-0.5 border-l-2 rounded-r-sm ${
                                getConsoleLogType(line) === 'error' ? 'border-l-red-500 bg-red-500/5 text-red-400' :
                                    getConsoleLogType(line) === 'warning' ? 'border-l-yellow-500 bg-yellow-500/5 text-yellow-400' :
                                        getConsoleLogType(line) === 'command' ? 'border-l-blue-500 bg-blue-500/5 text-blue-400' :
                                            'border-l-transparent'
                            }`}>{line}</div>
                        ))}
                        <div ref={endRef} />
                    </pre>
                </div>
            )}
        </div>
    )
}
