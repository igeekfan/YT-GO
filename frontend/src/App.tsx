import {useCallback, useEffect, useRef, useState} from 'react'
import {useAuthStatus} from './hooks/useAuthStatus'
import {useSettingsController} from './hooks/useSettingsController'
import {useAppBootstrap} from './hooks/useAppBootstrap'
import {useDownloadFeed} from './hooks/useDownloadFeed'
import {useUpdateDialog} from './hooks/useUpdateDialog'
import {useMediaDownload} from './hooks/useMediaDownload'
import AuthBoundary from './components/app/AuthBoundary'
import AppHeader from './components/app/AppHeader'
import ConsolePanel from './components/app/ConsolePanel'
import DownloadWorkspace from './components/download/DownloadWorkspace'
import DownloadList from './components/DownloadList'
import SettingsWorkspace from './components/SettingsWorkspace'
import SetupWizard from './components/SetupWizard'
import UpdateDialog from './components/UpdateDialog'
import DirBrowser from './components/DirBrowser'

function App() {
    const settings = useSettingsController()
    const auth = useAuthStatus()
    const bootstrap = useAppBootstrap(auth.status, settings.applySettingsToUI)
    const downloadFeed = useDownloadFeed(auth.status, settings.sendNotification)
    const updateDialog = useUpdateDialog(auth.status)
    const media = useMediaDownload({
        quality: settings.quality,
        outputDir: settings.outputDir,
        setOutputDir: settings.setOutputDir,
        downloadOptions: settings.downloadOptions,
    })
    const [showSettings, setShowSettings] = useState(false)
    const homeScrollYRef = useRef(0)

    const openSettings = useCallback(() => {
        if (showSettings) return

        homeScrollYRef.current = window.scrollY
        if (window.history.state?.ytGoSettings !== true) {
            window.history.pushState(
                {...(window.history.state ?? {}), ytGoSettings: true},
                '',
                window.location.href,
            )
        }
        setShowSettings(true)
    }, [showSettings])

    const closeSettings = useCallback(() => {
        if (!showSettings) return
        if (window.history.state?.ytGoSettings === true) {
            window.history.back()
            return
        }
        setShowSettings(false)
    }, [showSettings])

    useEffect(() => {
        const handlePopState = (event: PopStateEvent) => {
            const settingsOpen = event.state?.ytGoSettings === true
            setShowSettings(settingsOpen)
            if (!settingsOpen) {
                window.requestAnimationFrame(() => {
                    window.scrollTo({top: homeScrollYRef.current, left: 0, behavior: 'auto'})
                })
            }
        }

        window.addEventListener('popstate', handlePopState)
        return () => window.removeEventListener('popstate', handlePopState)
    }, [])
    const [showDirBrowser, setShowDirBrowser] = useState(false)

    return (
        <AuthBoundary status={auth.status} onRetry={auth.retry} onAuthenticated={auth.markAuthenticated}>
            <div className="min-h-screen bg-background">
                <AppHeader
                    ytdlp={bootstrap.ytdlp}
                    isUpdatingYtDlp={bootstrap.isUpdatingYtDlp}
                    language={settings.lang}
                    theme={settings.theme}
                    onUpdateYtDlp={bootstrap.updateYtDlp}
                    onToggleLanguage={settings.handleQuickLanguageToggle}
                    onToggleTheme={settings.handleQuickThemeToggle}
                    onOpenSettings={openSettings}
                />

                <main className="mx-auto max-w-3xl px-4 py-5 flex flex-col gap-4">
                    <UpdateDialog
                        open={updateDialog.open}
                        updateInfo={updateDialog.info}
                        loading={updateDialog.loading}
                        installing={updateDialog.installing}
                        error={updateDialog.error}
                        onClose={updateDialog.close}
                        onOpenReleasePage={updateDialog.openReleasePage}
                        onDownloadUpdate={updateDialog.downloadUpdate}
                        onCheckUpdate={updateDialog.check}
                    />

                    <DownloadWorkspace
                        ytdlp={bootstrap.ytdlp}
                        installingYtDlp={bootstrap.isInstallingYtDlp}
                        settings={settings}
                        media={media}
                        onInstallYtDlp={bootstrap.installYtDlp}
                        onRecheckYtDlp={bootstrap.recheckYtDlp}
                        onBrowseDirectory={() => setShowDirBrowser(true)}
                        onOpenSettings={openSettings}
                    />

                    <div className="space-y-3 animate-fade-in-up delay-3">
                        <ConsolePanel
                            logs={downloadFeed.consoleLogs}
                            open={downloadFeed.showConsole}
                            onToggle={downloadFeed.toggleConsole}
                            onClear={downloadFeed.clearConsole}
                        />
                        <DownloadList downloads={downloadFeed.downloads} onUpdate={downloadFeed.setDownloads} />
                    </div>
                </main>

                <SettingsWorkspace
                    open={showSettings}
                    initialSettings={settings.currentSettings}
                    onClose={closeSettings}
                    onSave={settings.stageSettingsSave}
                    onFlushSave={settings.flushSettings}
                    onSaved={settings.handleSettingsSaved}
                    onThemePreview={settings.setTheme}
                    onLanguagePreview={settings.setLang}
                />

                {bootstrap.showSetupWizard && (
                    <SetupWizard
                        onComplete={async (outputDir, cookiesFrom, cookiesFile, proxy, language, theme) => {
                            await settings.saveSetupSettings(outputDir, cookiesFrom, cookiesFile, proxy, language, theme)
                            bootstrap.closeSetupWizard()
                        }}
                    />
                )}

                <DirBrowser
                    open={showDirBrowser}
                    initialPath={settings.outputDir}
                    onSelect={settings.setDirectoryAndSave}
                    onClose={() => setShowDirBrowser(false)}
                />
            </div>
        </AuthBoundary>
    )
}

export default App
