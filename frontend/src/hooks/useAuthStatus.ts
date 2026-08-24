import {useCallback, useEffect, useState} from 'react'
import {backendMode, onUnauthorized} from '../lib/backend'
import {AuthStatus, resolveAuthStatus} from '../lib/auth'

export function useAuthStatus() {
    const [status, setStatus] = useState<AuthStatus>(() => backendMode === 'desktop' ? 'ready' : 'checking')
    const [attempt, setAttempt] = useState(0)

    useEffect(() => {
        if (backendMode === 'desktop') return
        return onUnauthorized(() => setStatus('login-required'))
    }, [])

    useEffect(() => {
        if (backendMode === 'desktop') return
        let active = true
        resolveAuthStatus()
            .then(nextStatus => { if (active) setStatus(nextStatus) })
            .catch(() => { if (active) setStatus('connection-error') })
        return () => { active = false }
    }, [attempt])

    const retry = useCallback(() => {
        setStatus('checking')
        setAttempt(current => current + 1)
    }, [])

    const markAuthenticated = useCallback(() => setStatus('ready'), [])

    return {status, retry, markAuthenticated}
}
