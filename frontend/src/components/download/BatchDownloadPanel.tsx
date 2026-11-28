import {type FormEvent, type ReactNode, useId, useRef, useState} from 'react'
import {ListPlus} from 'lucide-react'
import {toast} from 'sonner'
import {useI18n} from '../../i18n/context'
import {ReviewDownloadURLs, StartDownload} from '../../lib/backend'
import type {DownloadOptions, DownloadURLReview} from '../../types'
import {Badge, type BadgeProps} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'
import {Label} from '@/components/ui/label'

interface Props {
    outputDir: string
    hasFixedDir: boolean
    quality: string
    options: DownloadOptions
    directoryField: ReactNode
}

interface BatchEntry extends DownloadURLReview {
    selected: boolean
}

const STATUS_VARIANTS: Record<string, BadgeProps['variant']> = {
    valid: 'outline',
    invalid: 'destructive',
    duplicate: 'outline',
    queued: 'secondary',
    error: 'destructive',
}

export default function BatchDownloadPanel({outputDir, hasFixedDir, quality, options, directoryField}: Props) {
    const {t} = useI18n()
    const inputId = useId()
    const [draft, setDraft] = useState('')
    const [reviewedInput, setReviewedInput] = useState('')
    const [entries, setEntries] = useState<BatchEntry[]>([])
    const [activity, setActivity] = useState<'reviewing' | 'queueing' | null>(null)
    const [reviewError, setReviewError] = useState('')
    const queuedKeys = useRef(new Set<string>())
    const needsReview = draft !== reviewedInput
    const readyCount = entries.filter(entry => entry.status === 'valid').length
    const queuedCount = entries.filter(entry => entry.status === 'queued').length
    const selected = entries.filter(entry => entry.selected)

    const reviewLinks = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        if (!draft.trim() || !needsReview || activity) return
        setReviewError('')
        setActivity('reviewing')
        try {
            const results = await ReviewDownloadURLs(draft)
            setEntries(results.map(entry => {
                if (entry.status === 'valid' && queuedKeys.current.has(entry.key)) {
                    return {...entry, status: 'queued', selected: false}
                }
                return {...entry, selected: entry.status === 'valid'}
            }))
            setReviewedInput(draft)
        } catch (error) {
            setReviewError(error instanceof Error ? error.message : String(error))
        } finally {
            setActivity(null)
        }
    }

    const queueSelected = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        if (needsReview || activity || selected.length === 0) return
        if (!hasFixedDir && !outputDir.trim()) {
            toast.error(t('download.noDir'))
            return
        }

        setActivity('queueing')
        try {
            for (const entry of selected) {
                let result: BatchEntry
                try {
                    await StartDownload({url: entry.url, outputDir, quality, options})
                    queuedKeys.current.add(entry.key)
                    result = {...entry, status: 'queued', selected: false, error: ''}
                } catch (error) {
                    result = {...entry, status: 'error', error: error instanceof Error ? error.message : String(error)}
                }
                setEntries(current => current.map(row => row.line === entry.line ? result : row))
            }
        } finally {
            setActivity(null)
        }
    }

    const clearLinks = () => {
        setDraft('')
        setReviewedInput('')
        setEntries([])
        setReviewError('')
        queuedKeys.current.clear()
    }

    return (
        <section className="space-y-3 rounded-xl border bg-card/70 p-4 shadow-xs">
            <form className="space-y-2.5" method="post" onSubmit={reviewLinks} aria-labelledby={`${inputId}-label`}>
                <Label id={`${inputId}-label`} htmlFor={inputId}>{t('batch.links')}</Label>
                <p id={`${inputId}-hint`} className="text-xs text-muted-foreground">{t('batch.hint')}</p>
                <textarea
                    id={inputId}
                    name="links"
                    rows={4}
                    required
                    autoComplete="off"
                    autoCorrect="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    aria-describedby={`${inputId}-hint`}
                    value={draft}
                    readOnly={activity !== null}
                    onChange={event => {
                        setDraft(event.target.value)
                        setReviewError('')
                    }}
                    className="block w-full resize-y rounded-md border border-input bg-background px-3 py-2 text-base focus-visible:border-primary focus-visible:outline-2 focus-visible:outline-primary/30 sm:text-sm"
                />
                <div className="flex justify-end gap-2">
                    <Button type="button" variant="ghost" size="sm" disabled={!!activity || (!draft && entries.length === 0)} onClick={clearLinks}>
                        {t('action.clear')}
                    </Button>
                    <Button type="submit" variant="outline" size="sm" disabled={!!activity || !draft.trim() || !needsReview}>
                        {activity === 'reviewing' ? t('batch.reviewing') : t('batch.review')}
                    </Button>
                </div>
            </form>

            <div role="status">
                {reviewError && <p className="text-sm text-destructive">{t('batch.reviewFailed')} · {reviewError}</p>}
                {entries.length > 0 && !needsReview && (
                    <p id={`${inputId}-summary`} className="text-xs text-muted-foreground">
                        {t('batch.summary', {ready: readyCount, queued: queuedCount, attention: entries.length - readyCount - queuedCount})}
                    </p>
                )}
            </div>

            {entries.length > 0 && !needsReview && (
                <form className="space-y-3" method="post" onSubmit={queueSelected} aria-label={t('batch.reviewedLinks')} aria-describedby={`${inputId}-summary`}>
                    <ul className="max-h-64 divide-y overflow-y-auto rounded-lg border">
                        {entries.map(entry => {
                            const rowId = `${inputId}-${entry.line}`
                            const adjustable = entry.status === 'valid' || entry.status === 'error'
                            return (
                                <li key={entry.line} className="flex min-h-11 flex-wrap items-center gap-x-2.5 gap-y-1 px-3 py-2">
                                    <div className="flex min-w-0 flex-1 items-center gap-2.5">
                                        {adjustable && (
                                            <Checkbox
                                                id={rowId}
                                                name="selectedLinks"
                                                value={entry.url}
                                                checked={entry.selected}
                                                disabled={!!activity}
                                                onCheckedChange={checked => setEntries(current => current.map(row =>
                                                    row.line === entry.line ? {...row, selected: checked === true} : row
                                                ))}
                                            />
                                        )}
                                        <div className="min-w-0 break-all text-sm">
                                            {adjustable
                                                ? <Label htmlFor={rowId} className="cursor-pointer py-1 leading-snug">{entry.url}</Label>
                                                : entry.url}
                                            <span className="block text-xs text-muted-foreground">{t('batch.line', {line: entry.line})}</span>
                                        </div>
                                    </div>
                                    <Badge variant={STATUS_VARIANTS[entry.status]}>{t(`batch.status.${entry.status}`)}</Badge>
                                    {entry.error && <p className="basis-full break-words text-xs text-destructive">{entry.error}</p>}
                                </li>
                            )
                        })}
                    </ul>

                    {entries.some(entry => entry.status === 'valid' || entry.status === 'error') && (
                        <>
                            {directoryField}
                            <div className="flex justify-end">
                                <Button type="submit" disabled={!!activity || selected.length === 0}>
                                    <ListPlus aria-hidden="true" className="h-4 w-4" />
                                    {activity === 'queueing' ? t('batch.queueing') : t('batch.queueSelected', {count: selected.length})}
                                </Button>
                            </div>
                        </>
                    )}
                </form>
            )}
        </section>
    )
}
