import {useState} from 'react'
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
import SettingsDialog from './components/SettingsDialog'
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
                    onOpenSettings={() => setShowSettings(true)}
                />

                <main className="mx-auto max-w-3xl px-4 py-5 flex flex-col gap-4">
                    <DownloadWorkspace
                        ytdlp={bootstrap.ytdlp}
                        installingYtDlp={bootstrap.isInstallingYtDlp}
                        settings={settings}
                        media={media}
                        onInstallYtDlp={bootstrap.installYtDlp}
                        onRecheckYtDlp={bootstrap.recheckYtDlp}
                        onBrowseDirectory={() => setShowDirBrowser(true)}
                        onOpenSettings={() => setShowSettings(true)}
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

                <SettingsDialog
                    open={showSettings}
                    initialSettings={settings.currentSettings}
                    onClose={() => setShowSettings(false)}
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

                <UpdateDialog
                    open={updateDialog.open}
                    updateInfo={updateDialog.info}
                    loading={updateDialog.loading}
                    error={updateDialog.error}
                    onClose={updateDialog.close}
                    onOpenReleasePage={updateDialog.openReleasePage}
                    onCheckUpdate={updateDialog.check}
                />

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
