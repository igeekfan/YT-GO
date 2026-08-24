import {useCallback, useEffect, useRef, useState} from 'react'
import {backendMode, GetDownloads} from '../lib/backend'
import {EventsOn, EventsOnConnectionOpen} from '../lib/runtime'
import {AuthStatus} from '../lib/auth'
import {DownloadTask} from '../types'
import {useI18n} from '../i18n/context'

const SNAPSHOT_ATTEMPTS_PER_CYCLE = 3
const INITIAL_SNAPSHOT_RETRY_DELAY_MS = 250
const MAX_SNAPSHOT_RETRY_DELAY_MS = 5000

export function useDownloadFeed(authStatus: AuthStatus, sendNotification: (title: string, body: string) => void) {
    const {t} = useI18n()
    const [downloads, setDownloads] = useState<DownloadTask[]>([])
    const downloadsRef = useRef<DownloadTask[]>([])
    const eventRevisionRef = useRef(0)
    const snapshotGenerationRef = useRef(0)
    const snapshotRetryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const [consoleLogs, setConsoleLogs] = useState<string[]>([])
    const [showConsole, setShowConsole] = useState(false)

    const commitDownloads = useCallback((tasks: DownloadTask[]) => {
        downloadsRef.current = tasks
        setDownloads(tasks)
    }, [])

    const replaceDownloads = useCallback((tasks: DownloadTask[]) => {
        eventRevisionRef.current++
        commitDownloads(tasks)
    }, [commitDownloads])

    const cancelSnapshotReconciliation = useCallback(() => {
        snapshotGenerationRef.current++
        if (snapshotRetryTimerRef.current !== null) {
            clearTimeout(snapshotRetryTimerRef.current)
            snapshotRetryTimerRef.current = null
        }
    }, [])

    const reconcileDownloads = useCallback(() => {
        cancelSnapshotReconciliation()
        const generation = snapshotGenerationRef.current
        let retryDelay = INITIAL_SNAPSHOT_RETRY_DELAY_MS

        function scheduleRetry() {
            if (generation !== snapshotGenerationRef.current || snapshotRetryTimerRef.current !== null) return
            const delay = retryDelay
            retryDelay = Math.min(retryDelay * 2, MAX_SNAPSHOT_RETRY_DELAY_MS)
            snapshotRetryTimerRef.current = setTimeout(() => {
                snapshotRetryTimerRef.current = null
                void runCycle()
            }, delay)
        }

        async function runCycle(): Promise<void> {
            for (let attempt = 0; attempt < SNAPSHOT_ATTEMPTS_PER_CYCLE; attempt++) {
                const startingRevision = eventRevisionRef.current
                let tasks: DownloadTask[]
                try { tasks = await GetDownloads() }
                catch {
                    scheduleRetry()
                    return
                }

                if (generation !== snapshotGenerationRef.current) return
                if (startingRevision === eventRevisionRef.current) {
                    if (tasks) commitDownloads(tasks)
                    return
                }
            }

            scheduleRetry()
        }

        void runCycle()
    }, [cancelSnapshotReconciliation, commitDownloads])

    useEffect(() => {
        if (authStatus !== 'ready') return
        reconcileDownloads()
        return cancelSnapshotReconciliation
    }, [authStatus, cancelSnapshotReconciliation, reconcileDownloads])

    useEffect(() => {
        if (authStatus !== 'ready' || backendMode !== 'web') return
        return EventsOnConnectionOpen(reconcileDownloads)
    }, [authStatus, reconcileDownloads])

    useEffect(() => {
        if (authStatus !== 'ready') return
        return EventsOn('download:update', (task: DownloadTask) => {
            eventRevisionRef.current++
            const current = downloadsRef.current
            const index = current.findIndex(download => download.id === task.id)
            const wasCompleted = index >= 0 && current[index].status === 'completed'
            const next = index < 0 ? [task, ...current] : [...current]
            if (index >= 0) {
                next[index] = task
            }
            commitDownloads(next)
            if (!wasCompleted && task.status === 'completed') {
                sendNotification(t('notification.downloadComplete'), task.title || task.url)
            }
        })
    }, [authStatus, commitDownloads, sendNotification, t])

    useEffect(() => {
        if (authStatus !== 'ready') return
        return EventsOn('download:remove', (taskId: string) => {
            eventRevisionRef.current++
            commitDownloads(downloadsRef.current.filter(download => download.id !== taskId))
        })
    }, [authStatus, commitDownloads])

    useEffect(() => {
        if (authStatus !== 'ready') return
        return EventsOn('app:log', (message: string) => {
            const timestamp = new Date().toLocaleTimeString()
            setConsoleLogs(current => {
                const next = [...current, `[${timestamp}] ${message}`]
                return next.length > 200 ? next.slice(-200) : next
            })
            setShowConsole(true)
        })
    }, [authStatus])

    useEffect(() => {
        if (authStatus !== 'ready') return
        return EventsOn('download:log', (data: {taskId: string; line: string}) => {
            const timestamp = new Date().toLocaleTimeString()
            setConsoleLogs(current => {
                const next = [...current, `[${timestamp}] [${data.taskId}] ${data.line}`]
                return next.length > 200 ? next.slice(-200) : next
            })
            setShowConsole(true)
        })
    }, [authStatus])

    const toggleConsole = useCallback(() => setShowConsole(current => !current), [])
    const clearConsole = useCallback(() => {
        setConsoleLogs([])
        setShowConsole(false)
    }, [])

    return {downloads, setDownloads: replaceDownloads, consoleLogs, showConsole, toggleConsole, clearConsole}
}

export type DownloadFeed = ReturnType<typeof useDownloadFeed>
