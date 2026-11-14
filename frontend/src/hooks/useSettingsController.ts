import {useCallback, useEffect, useRef, useState} from 'react'
import {ApplySettingsPatch, SettingsPatch} from '../lib/backend'
import {useI18n} from '../i18n/context'
import {useLatestValueSaver} from '../lib/useLatestValueSaver'
import {Settings} from '../types'
import {toast} from 'sonner'

export type Theme = 'dark' | 'light'
export type DownloadOptionKey = 'saveThumbnail' | 'saveDescription' | 'embedChapters' | 'writeSubtitles' | 'embedSubtitles' | 'sponsorBlock'
export type DownloadOptionState = Record<DownloadOptionKey, boolean>

const EMPTY_DOWNLOAD_OPTIONS: DownloadOptionState = {
    saveThumbnail: false,
    saveDescription: false,
    embedChapters: false,
    writeSubtitles: false,
    embedSubtitles: false,
    sponsorBlock: false,
}

export function useSettingsController() {
    const {t, lang, setLang} = useI18n()
    const [theme, setTheme] = useState<Theme>(() => {
        const saved = localStorage.getItem('YT-GOto-theme') as Theme | null
        if (saved) return saved
        return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
    })
    const [quality, setQuality] = useState('best')
    const [outputDir, setOutputDir] = useState('')
    const [currentSettings, setCurrentSettings] = useState<Settings | null>(null)
    const currentSettingsRef = useRef<Settings | null>(null)
    const [notificationsEnabled, setNotificationsEnabled] = useState(false)
    const [notifications, setNotifications] = useState<Array<{id: string; title: string; body: string}>>([])
    const [downloadOptions, setDownloadOptions] = useState<DownloadOptionState>(EMPTY_DOWNLOAD_OPTIONS)

    useEffect(() => {
        const root = document.documentElement
        if (theme === 'dark') root.classList.add('dark')
        else root.classList.remove('dark')
        localStorage.setItem('YT-GOto-theme', theme)
    }, [theme])

    const applySettingsToUI = useCallback((settings: Settings) => {
        currentSettingsRef.current = settings
        setCurrentSettings(settings)
        if (settings.outputDir) setOutputDir(settings.outputDir)
        if (settings.quality) setQuality(settings.quality)
        if (settings.theme) setTheme(settings.theme as Theme)
        if (settings.language) setLang(settings.language as 'zh-CN' | 'en-US')
        const completionNotificationsEnabled = settings.notifications || false
        setNotificationsEnabled(completionNotificationsEnabled)
        if (!completionNotificationsEnabled) setNotifications([])
        setDownloadOptions({
            saveThumbnail: settings.saveThumbnail || false,
            saveDescription: settings.saveDescription || false,
            embedChapters: settings.embedChapters || false,
            writeSubtitles: settings.writeSubtitles || false,
            embedSubtitles: settings.embedSubtitles || false,
            sponsorBlock: settings.sponsorBlock || false,
        })
    }, [setLang])

    const handleSettingsSaved = useCallback((settings: Settings) => {
        applySettingsToUI(settings)
    }, [applySettingsToUI])

    const {schedule: saveSettingsLatest, flush: flushSettings} = useLatestValueSaver<SettingsPatch>({
        save: ApplySettingsPatch,
        onSaved: settings => handleSettingsSaved(settings as Settings),
        onError: error => {
            console.error('Failed to save settings:', error)
            toast.error(t('settings.saveFailed'))
        },
    })

    const stageSettingsSave = useCallback((settings: Settings) => {
        const previous = currentSettingsRef.current
        currentSettingsRef.current = settings
        setCurrentSettings(settings)
        if (!previous) return
        const patch = Object.keys(settings).reduce<SettingsPatch>((changes, key) => {
            const typedKey = key as keyof Settings
            if (settings[typedKey] !== previous[typedKey]) {
                changes[typedKey as keyof SettingsPatch] = settings[typedKey] as never
            }
            return changes
        }, {})
        if (Object.keys(patch).length > 0) saveSettingsLatest(patch)
    }, [saveSettingsLatest])

    const persistSettingsPatch = useCallback((patch: Partial<Settings>) => {
        const current = currentSettingsRef.current
        if (!current) return
        stageSettingsSave({...current, ...patch})
    }, [stageSettingsSave])

    const setDownloadOption = useCallback((key: DownloadOptionKey, value: boolean) => {
        setDownloadOptions(current => ({...current, [key]: value}))
        persistSettingsPatch({[key]: value} as Partial<Settings>)
    }, [persistSettingsPatch])

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

    const setDirectoryAndSave = useCallback((directory: string) => {
        setOutputDir(directory)
        const current = currentSettingsRef.current
        if (current) stageSettingsSave({...current, outputDir: directory})
    }, [stageSettingsSave])

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
        const settings: Settings = {
            outputDir: directory, quality: 'best', language, theme: nextTheme, proxy, rateLimit: '',
            updateProxy: '',
            maxConcurrent: 3, notifications: true, saveDescription: false, saveThumbnail: false,
            writeSubtitles: false, subtitleLangs: '', embedSubtitles: false, embedChapters: false,
            sponsorBlock: false, filenameTemplate: '', mergeOutputFormat: '', audioFormat: '',
            cookiesFrom, cookiesFile,
        }
		const saved = await ApplySettingsPatch(settings)
		applySettingsToUI(saved as Settings)
    }, [applySettingsToUI])

    return {
        theme,
        setTheme,
        lang,
        setLang,
        quality,
        outputDir,
        setOutputDir,
        currentSettings,
        downloadOptions,
        applySettingsToUI,
        handleSettingsSaved,
        stageSettingsSave,
        flushSettings,
        setDownloadOption,
        handleQuickLanguageToggle,
        handleQuickThemeToggle,
        setDirectoryAndSave,
        sendNotification,
        notifications,
        dismissNotification,
        saveSetupSettings,
    }
}

export type SettingsController = ReturnType<typeof useSettingsController>
