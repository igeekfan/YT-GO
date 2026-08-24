import {useCallback, useEffect, useRef, useState} from 'react'
import {SaveSettings} from '../lib/backend'
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
        setNotificationsEnabled(settings.notifications || false)
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
        if (settings.notifications && 'Notification' in window && Notification.permission === 'default') {
            void Notification.requestPermission()
        }
    }, [applySettingsToUI])

    const {schedule: saveSettingsLatest, flush: flushSettings} = useLatestValueSaver<Settings>({
        save: SaveSettings,
        onSaved: handleSettingsSaved,
        onError: error => {
            console.error('Failed to save settings:', error)
            toast.error(t('settings.saveFailed'))
        },
    })

    const stageSettingsSave = useCallback((settings: Settings) => {
        currentSettingsRef.current = settings
        setCurrentSettings(settings)
        saveSettingsLatest(settings)
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
        if (!notificationsEnabled || !('Notification' in window)) return
        if (Notification.permission === 'granted') new Notification(title, {body})
        else if (Notification.permission !== 'denied') {
            Notification.requestPermission().then(permission => {
                if (permission === 'granted') new Notification(title, {body})
            })
        }
    }, [notificationsEnabled])

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
            maxConcurrent: 3, notifications: true, saveDescription: false, saveThumbnail: false,
            writeSubtitles: false, subtitleLangs: '', embedSubtitles: false, embedChapters: false,
            sponsorBlock: false, filenameTemplate: '', mergeOutputFormat: '', audioFormat: '',
            cookiesFrom, cookiesFile,
        }
        await SaveSettings(settings)
        applySettingsToUI(settings)
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
        saveSetupSettings,
    }
}

export type SettingsController = ReturnType<typeof useSettingsController>
