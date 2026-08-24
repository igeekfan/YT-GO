import {useCallback, useEffect, useState} from 'react'
import {
    backendMode,
    CheckYtDlp,
    fetchWebConfig,
    GetSettings,
    InstallYtDlp,
    IsFirstRun,
    NeedsCookieConfig,
    UpdateYtDlp,
} from '../lib/backend'
import {AuthStatus} from '../lib/auth'
import {Settings, YtDlpStatus} from '../types'
import {useI18n} from '../i18n/context'
import {toast} from 'sonner'

export function useAppBootstrap(authStatus: AuthStatus, applySettings: (settings: Settings) => void) {
    const {t} = useI18n()
    const [ytdlp, setYtdlp] = useState<YtDlpStatus | null>(null)
    const [isUpdatingYtDlp, setIsUpdatingYtDlp] = useState(false)
    const [isInstallingYtDlp, setIsInstallingYtDlp] = useState(false)
    const [showSetupWizard, setShowSetupWizard] = useState(false)

    useEffect(() => {
        if (authStatus !== 'ready') return
        let active = true

        const initialize = async () => {
            const [, statusResult, settingsResult] = await Promise.allSettled([
                backendMode === 'web' ? fetchWebConfig() : Promise.resolve(null),
                CheckYtDlp(),
                GetSettings(),
            ])
            if (!active) return

            if (statusResult.status === 'fulfilled') setYtdlp(statusResult.value)
            else setYtdlp({available: false, version: '', path: ''})

            if (settingsResult.status === 'fulfilled') {
                const settings = settingsResult.value
                applySettings(settings)
                if (settings.notifications && 'Notification' in window && Notification.permission === 'default') {
                    void Notification.requestPermission()
                }
            }

            if (backendMode === 'desktop') {
                try {
                    const firstRun = await IsFirstRun()
                    if (!active) return
                    if (firstRun || await NeedsCookieConfig()) setShowSetupWizard(true)
                } catch {
                    if (active) setShowSetupWizard(true)
                }
            }
        }

        void initialize()
        return () => { active = false }
    }, [authStatus, applySettings])

    const updateYtDlp = useCallback(async () => {
        setIsUpdatingYtDlp(true)
        try {
            await UpdateYtDlp()
            toast.success(t('ytdlp.updateSuccess'))
            setYtdlp(await CheckYtDlp())
        } catch (error: any) {
            toast.error(t('ytdlp.updateFail') + (error?.message ? `: ${error.message}` : ''))
        } finally {
            setIsUpdatingYtDlp(false)
        }
    }, [t])

    const installYtDlp = useCallback(async () => {
        setIsInstallingYtDlp(true)
        try {
            await InstallYtDlp()
            toast.success(t('ytdlp.installSuccess'))
            setYtdlp(await CheckYtDlp())
        } catch (error: any) {
            toast.error(t('ytdlp.installFail') + (error?.message ? `: ${error.message}` : ''))
        } finally {
            setIsInstallingYtDlp(false)
        }
    }, [t])

    const recheckYtDlp = useCallback(() => {
        void CheckYtDlp().then(setYtdlp)
    }, [])

    const closeSetupWizard = useCallback(() => setShowSetupWizard(false), [])

    return {
        ytdlp,
        isUpdatingYtDlp,
        isInstallingYtDlp,
        showSetupWizard,
        updateYtDlp,
        installYtDlp,
        recheckYtDlp,
        closeSetupWizard,
    }
}

export type AppBootstrap = ReturnType<typeof useAppBootstrap>
