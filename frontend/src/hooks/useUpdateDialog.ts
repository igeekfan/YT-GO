import {useCallback, useEffect, useState} from 'react'
import {CheckForUpdate, OpenReleasePage} from '../lib/backend'
import {AuthStatus} from '../lib/auth'
import {useI18n} from '../i18n/context'

interface UpdateInfo {
    hasUpdate: boolean
    currentVersion: string
    latestVersion: string
    releaseName: string
    releaseBody: string
    htmlUrl: string
    publishedAt: string
}

export function useUpdateDialog(authStatus: AuthStatus) {
    const {t} = useI18n()
    const [open, setOpen] = useState(false)
    const [info, setInfo] = useState<UpdateInfo | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const check = useCallback(async () => {
        setLoading(true)
        setError(null)
        setOpen(true)
        try {
            setInfo(await CheckForUpdate())
        } catch (checkError: any) {
            setError(checkError?.message || t('update.error'))
        } finally {
            setLoading(false)
        }
    }, [t])

    const openReleasePage = useCallback(async () => {
        try { await OpenReleasePage() }
        catch (openError) { console.error('Failed to open release page:', openError) }
    }, [])

    useEffect(() => {
        if (authStatus !== 'ready') return
        const timer = setTimeout(() => { void check() }, 3000)
        return () => clearTimeout(timer)
    }, [authStatus, check])

    const close = useCallback(() => setOpen(false), [])

    return {open, info, loading, error, check, openReleasePage, close}
}
