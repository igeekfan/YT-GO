import {useCallback, useEffect, useRef, useState} from 'react'
import {backendMode, CheckForUpdate, DownloadAndInstallUpdate, OpenReleasePage} from '../lib/backend'
import type {AuthStatus} from '../lib/auth'

interface UpdateError {
    action: 'check' | 'install'
    message?: string
}

export function useAppUpdate(authStatus: AuthStatus) {
    const [info, setInfo] = useState<Awaited<ReturnType<typeof CheckForUpdate>> | null>(null)
    const [checking, setChecking] = useState(false)
    const [installing, setInstalling] = useState(false)
    const [installerStarted, setInstallerStarted] = useState(false)
    const [error, setError] = useState<UpdateError | null>(null)
    const inFlight = useRef(false)
    const canInstall = backendMode === 'desktop' && info?.canInstall === true

    const check = useCallback(async (manual = true) => {
        if (inFlight.current) return
        inFlight.current = true
        setChecking(true)
        if (manual) {
            setError(null)
            setInstallerStarted(false)
        }
        try {
            setInfo(await CheckForUpdate())
        } catch (cause) {
            if (manual) {
                setInfo(null)
                setError({action: 'check', message: cause instanceof Error ? cause.message : undefined})
            }
        } finally {
            inFlight.current = false
            setChecking(false)
        }
    }, [])

    const install = async () => {
        if (!info?.hasUpdate || inFlight.current) return
        inFlight.current = true
        setInstalling(true)
        setInstallerStarted(false)
        setError(null)
        try {
            if (canInstall) {
                await DownloadAndInstallUpdate()
                setInstallerStarted(true)
            } else {
                await OpenReleasePage()
            }
        } catch (cause) {
            setError({action: 'install', message: cause instanceof Error ? cause.message : undefined})
        } finally {
            inFlight.current = false
            setInstalling(false)
        }
    }

    useEffect(() => {
        if (authStatus !== 'ready') return
        const timer = setTimeout(() => { void check(false) }, 3000)
        return () => clearTimeout(timer)
    }, [authStatus, check])

    return {info, checking, installing, installerStarted, error, canInstall, check, install}
}

export type AppUpdate = ReturnType<typeof useAppUpdate>
