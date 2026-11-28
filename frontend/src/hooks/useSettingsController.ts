import {useCallback, useEffect, useRef, useState} from 'react'
import {ApplySettingsPatch, SettingsPatch} from '../lib/backend'
import {useI18n} from '../i18n/context'
import {EMPTY_DOWNLOAD_PROFILES} from '../lib/downloadProfiles'
import {useLatestValueSaver} from '../lib/useLatestValueSaver'
import type {DownloadProfiles, Settings} from '../types'
import {toast} from 'sonner'

export type Theme = 'dark' | 'light'

function mergeSettingsPatches(current: SettingsPatch, next: SettingsPatch): SettingsPatch {
    return {...current, ...next}
}

export function useSettingsController() {
    const {t, lang, setLang} = useI18n()
    const [theme, setTheme] = useState<Theme>(() => {
        const saved = localStorage.getItem('YT-GOto-theme') as Theme | null
        if (saved) return saved
        return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
    })
    const [currentSettings, setCurrentSettings] = useState<Settings | null>(null)
    const currentSettingsRef = useRef<Settings | null>(null)
    const settingsWrites = useRef(Promise.resolve())
    const [savingDownloadProfiles, setSavingDownloadProfiles] = useState(false)
    const [notificationsEnabled, setNotificationsEnabled] = useState(false)
    const [notifications, setNotifications] = useState<Array<{id: string; title: string; body: string}>>([])
    const downloadProfiles = currentSettings?.downloadProfiles ?? EMPTY_DOWNLOAD_PROFILES
    const selectedDownloadProfile = downloadProfiles.items?.find(profile => profile.id === downloadProfiles.selectedId)

    useEffect(() => {
        const root = document.documentElement
        if (theme === 'dark') root.classList.add('dark')
        else root.classList.remove('dark')
        localStorage.setItem('YT-GOto-theme', theme)
    }, [theme])

    const applySettingsToUI = useCallback((settings: Settings) => {
        currentSettingsRef.current = settings
        setCurrentSettings(settings)
        if (settings.theme) setTheme(settings.theme as Theme)
        if (settings.language) setLang(settings.language as 'zh-CN' | 'en-US')
        const completionNotificationsEnabled = settings.notifications || false
        setNotificationsEnabled(completionNotificationsEnabled)
        if (!completionNotificationsEnabled) setNotifications([])
    }, [setLang])

    const handleSettingsSaved = useCallback((settings: Settings) => {
        applySettingsToUI(settings)
    }, [applySettingsToUI])

    const writeSettings = useCallback((patch: SettingsPatch) => {
        const result = settingsWrites.current.then(() => ApplySettingsPatch(patch))
        settingsWrites.current = result.then(() => undefined, () => undefined)
        return result
    }, [])

    const {schedule: saveSettingsLatest, flush: flushSettings} = useLatestValueSaver<SettingsPatch, Settings>({
        save: writeSettings,
        onSaved: handleSettingsSaved,
        mergePending: mergeSettingsPatches,
        onError: error => {
            console.error('Failed to save settings:', error)
            toast.error(t('settings.saveFailed'))
        },
    })

    const stageSettingsSave = useCallback((settings: Settings) => {
        const previous = currentSettingsRef.current
        if (!previous) return
        const next = {...settings, downloadProfiles: previous.downloadProfiles}
        currentSettingsRef.current = next
        setCurrentSettings(next)
        const patch = Object.keys(next).reduce<SettingsPatch>((changes, key) => {
            const typedKey = key as keyof Settings
            if (next[typedKey] !== previous[typedKey]) {
                changes[typedKey] = next[typedKey] as never
            }
            return changes
        }, {})
        if (Object.keys(patch).length > 0) saveSettingsLatest(patch)
    }, [saveSettingsLatest])

    const saveDownloadProfiles = useCallback(async (profiles: DownloadProfiles) => {
        setSavingDownloadProfiles(true)
        try {
            await flushSettings()
            const saved = await writeSettings({downloadProfiles: profiles})
            const next = {...(currentSettingsRef.current ?? saved), downloadProfiles: saved.downloadProfiles}
            currentSettingsRef.current = next
            setCurrentSettings(next)
        } finally {
            setSavingDownloadProfiles(false)
        }
    }, [flushSettings, writeSettings])

    const handleQuickLanguageToggle = useCallback(() => {
        const nextLanguage = lang === 'zh-CN' ? 'en-US' : 'zh-CN'
        setLang(nextLanguage)
        const current = currentSettingsRef.current
        if (current) stageSettingsSave({...current, language: nextLanguage})
    }, [lang, setLang, stageSettingsSave])

    const handleQuickThemeToggle = useCallback(() => {
        const nextTheme = theme === 'dark' ? 'light' : 'dark'
        setTheme(nextTheme)
        const current = currentSettingsRef.current
        if (current) stageSettingsSave({...current, theme: nextTheme})
    }, [stageSettingsSave, theme])

    const sendNotification = useCallback((title: string, body: string) => {
        if (!notificationsEnabled) return
        const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`
        setNotifications(current => [{id, title, body}, ...current].slice(0, 3))
    }, [notificationsEnabled])

    const dismissNotification = useCallback((id: string) => {
        setNotifications(current => current.filter(notification => notification.id !== id))
    }, [])

    const saveSetupSettings = useCallback(async (
        directory: string,
        cookiesFrom: string,
        cookiesFile: string,
        proxy: string,
        language: 'zh-CN' | 'en-US',
        nextTheme: Theme,
    ) => {
        const settings: SettingsPatch = {
            outputDir: directory, quality: 'best', language, theme: nextTheme, proxy, rateLimit: '',
            updateProxy: '',
            maxConcurrent: 3, notifications: true, saveDescription: false, saveThumbnail: false,
            writeSubtitles: false, subtitleLangs: '', embedSubtitles: false, embedChapters: false,
            sponsorBlock: false, filenameTemplate: '', mergeOutputFormat: '', audioFormat: '',
            cookiesFrom, cookiesFile,
        }
        const saved = await writeSettings(settings)
        applySettingsToUI(saved)
    }, [applySettingsToUI, writeSettings])

    return {
        theme,
        setTheme,
        lang,
        setLang,
        currentSettings,
        downloadProfiles,
        selectedDownloadProfile,
        savingDownloadProfiles,
        saveDownloadProfiles,
        applySettingsToUI,
        handleSettingsSaved,
        stageSettingsSave,
        flushSettings,
        handleQuickLanguageToggle,
        handleQuickThemeToggle,
        sendNotification,
        notifications,
        dismissNotification,
        saveSetupSettings,
    }
}

export type SettingsController = ReturnType<typeof useSettingsController>
