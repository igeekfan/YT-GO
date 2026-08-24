import {useCallback, useEffect, useRef} from 'react'

interface LatestValueSaverOptions<T> {
    save: (value: T) => Promise<unknown>
    onSaved: (value: T) => void
    onError: (error: unknown) => void
    delay?: number
}

// Debounces bursts and serializes writes. If changes arrive while a request is
// in flight, only the newest pending snapshot is persisted next.
export function useLatestValueSaver<T>({save, onSaved, onError, delay = 250}: LatestValueSaverOptions<T>) {
    const saveRef = useRef(save)
    const onSavedRef = useRef(onSaved)
    const onErrorRef = useRef(onError)
    const pendingRef = useRef<T | null>(null)
    const activeFlushRef = useRef<Promise<void> | null>(null)
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    saveRef.current = save
    onSavedRef.current = onSaved
    onErrorRef.current = onError

    const flush = useCallback(async (): Promise<void> => {
        if (timerRef.current !== null) {
            clearTimeout(timerRef.current)
            timerRef.current = null
        }
        if (activeFlushRef.current) {
            await activeFlushRef.current
            if (pendingRef.current !== null) await flush()
            return
        }

        const execute = async () => {
            while (pendingRef.current !== null) {
                const value = pendingRef.current
                pendingRef.current = null
                try {
                    await saveRef.current(value)
                    // Do not publish an older successful snapshot while a
                    // newer value is already queued; it can regress parent UI.
                    if (pendingRef.current === null) onSavedRef.current(value)
                } catch (error) {
                    onErrorRef.current(error)
                }
            }
        }
        const activeFlush = execute()
        activeFlushRef.current = activeFlush
        try {
            await activeFlush
        } finally {
            if (activeFlushRef.current === activeFlush) activeFlushRef.current = null
        }
        if (pendingRef.current !== null) await flush()
    }, [])

    const schedule = useCallback((value: T) => {
        pendingRef.current = value
        if (timerRef.current !== null) clearTimeout(timerRef.current)
        timerRef.current = setTimeout(() => { void flush() }, delay)
    }, [delay, flush])

    useEffect(() => () => {
        if (timerRef.current !== null) clearTimeout(timerRef.current)
    }, [])

    return {schedule, flush}
}
