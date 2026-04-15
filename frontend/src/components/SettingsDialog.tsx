import {useState, useEffect} from 'react'
import {Settings} from '../types'
import {useI18n} from '../i18n/context'
import {SaveSettings, GetSettings, SelectFolder, SelectCookiesFile, GetDiagnosticInfo, UpdateYtDlp, UpdateDeno, ResetSettings, CheckForUpdate, OpenReleasePage, GetAboutInfo, GetDepStatus} from '../lib/backend'

interface DiagnosticInfo {
    ytdlpPath: string
    ytdlpVersion: string
    ytdlpFound: boolean
    ffmpegPath: string
    ffmpegVersion: string
    ffmpegFound: boolean
    nodeVersion: string
    appVersion: string
    testOutput: string
    error: string
}

interface AboutInfo {
    appVersion: string
    systemVersion: string
    githubRepo: string
    githubUrl: string
    authorEmail: string
}

interface DepItem {
    found: boolean
    version: string
    path: string
}

interface DepStatus {
    ytdlp: DepItem
    ffmpeg: DepItem
    jsRuntime: DepItem
    jsRuntimeName: string
}

interface Props {
    open: boolean
    initialSettings: Settings | null
    onClose: () => void
    onSaved: (settings: Settings) => void
    onThemePreview: (theme: 'dark' | 'light') => void
    onLanguagePreview: (lang: 'zh-CN' | 'en-US') => void
}

const QUALITY_OPTIONS = ['best', '1080p', '720p', '480p', '360p', 'audio']
const THEME_OPTIONS = ['dark', 'light']
const LANGUAGE_OPTIONS = ['zh-CN', 'en-US']

type SettingsTab = 'download' | 'media' | 'network' | 'deps' | 'tools' | 'appearance'

const TAB_KEYS: SettingsTab[] = ['download', 'media', 'network', 'deps', 'tools', 'appearance']

function SettingsDialog({open, initialSettings, onClose, onSaved, onThemePreview, onLanguagePreview}: Props) {
    const {t, lang} = useI18n()
    const [settings, setSettings] = useState<Settings | null>(null)
    const [saving, setSaving] = useState(false)
    const [activeTab, setActiveTab] = useState<SettingsTab>('download')
    const [diagnostic, setDiagnostic] = useState<DiagnosticInfo | null>(null)
    const [aboutInfo, setAboutInfo] = useState<AboutInfo | null>(null)
    const [loadingDiag, setLoadingDiag] = useState(false)
    const [isUpdatingYtDlp, setIsUpdatingYtDlp] = useState(false)
    const [updateResult, setUpdateResult] = useState<string | null>(null)
    const [isUpdatingDeno, setIsUpdatingDeno] = useState(false)
    const [denoUpdateResult, setDenoUpdateResult] = useState<string | null>(null)
    const [isResetting, setIsResetting] = useState(false)
    const [isCheckingUpdate, setIsCheckingUpdate] = useState(false)
    const [depStatus, setDepStatus] = useState<DepStatus | null>(null)
    const [loadingDeps, setLoadingDeps] = useState(false)
    const [updateInfo, setUpdateInfo] = useState<{
        hasUpdate: boolean
        currentVersion: string
        latestVersion: string
        releaseName: string
        releaseBody: string
        htmlUrl: string
        publishedAt: string
    } | null>(null)

    const handleCheckForUpdate = async () => {
        setIsCheckingUpdate(true)
        try {
            const info = await CheckForUpdate()
            setUpdateInfo(info)
        } catch (e: any) {
            setUpdateInfo({
                hasUpdate: false,
                currentVersion: '0.0.1',
                latestVersion: '0.0.0',
                releaseName: '',
                releaseBody: e?.message || 'Failed to check for updates',
                htmlUrl: '',
                publishedAt: ''
            })
        } finally {
            setIsCheckingUpdate(false)
        }
    }

    const handleOpenReleasePage = async () => {
        try {
            await OpenReleasePage()
        } catch (e) {
            console.error('Failed to open release page:', e)
        }
    }

    useEffect(() => {
        if (open) {
            setSettings(initialSettings)
            setDiagnostic(null)
            setActiveTab('download')
            setUpdateInfo(null)
            setDepStatus(null)
            GetAboutInfo().then(setAboutInfo).catch(console.error)
        }
    }, [open, initialSettings])

    useEffect(() => {
        if (open && activeTab === 'deps' && !depStatus && !loadingDeps) {
            handleRefreshDeps()
        }
    }, [open, activeTab])

    useEffect(() => {
        if (!open || !settings?.theme) return
        onThemePreview(settings.theme as 'dark' | 'light')
    }, [open, settings?.theme, onThemePreview])

    useEffect(() => {
        if (!open || !settings?.language) return
        onLanguagePreview(settings.language as 'zh-CN' | 'en-US')
    }, [open, settings?.language, onLanguagePreview])

    const handleGetDiagnostic = async () => {
        setLoadingDiag(true)
        try {
            const info = await GetDiagnosticInfo()
            setDiagnostic(info as DiagnosticInfo)
        } catch (e) {
            console.error('Failed to get diagnostic info:', e)
        } finally {
            setLoadingDiag(false)
        }
    }

    const handleRefreshDeps = async () => {
        setLoadingDeps(true)
        try {
            const status = await GetDepStatus()
            setDepStatus(status as DepStatus)
        } catch (e) {
            console.error('Failed to get dep status:', e)
        } finally {
            setLoadingDeps(false)
        }
    }

    const handleUpdateYtDlp = async () => {
        setIsUpdatingYtDlp(true)
        setUpdateResult(null)
        try {
            const result = await UpdateYtDlp()
            setUpdateResult(result || t('ytdlp.updateSuccess'))
            // Refresh diagnostic info after update
            const info = await GetDiagnosticInfo()
            setDiagnostic(info as DiagnosticInfo)
        } catch (e: any) {
            setUpdateResult(t('ytdlp.updateFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsUpdatingYtDlp(false)
        }
    }

    const handleUpdateDeno = async () => {
        setIsUpdatingDeno(true)
        setDenoUpdateResult(null)
        try {
            const result = await UpdateDeno()
            setDenoUpdateResult(result || t('dep.denoUpdateSuccess'))
            const status = await GetDepStatus()
            setDepStatus(status as DepStatus)
        } catch (e: any) {
            setDenoUpdateResult(t('dep.denoUpdateFail') + (e?.message ? `: ${e.message}` : ''))
        } finally {
            setIsUpdatingDeno(false)
        }
    }

    const handleResetSettings = async () => {
        setIsResetting(true)
        try {
            await ResetSettings()
            setUpdateResult(t('settings.resetSuccess') + ' ' + (lang === 'zh-CN' ? '请重启应用使设置生效' : 'Please restart the app for changes to take effect'))
        } catch (e: any) {
            setUpdateResult(e?.message ? `${e.message}` : t('settings.resetFailed'))
        } finally {
            setIsResetting(false)
        }
    }

    if (!open || !settings) return null

    const handleDismiss = () => {
        if (initialSettings?.theme) {
            onThemePreview(initialSettings.theme as 'dark' | 'light')
        }
        if (initialSettings?.language) {
            onLanguagePreview(initialSettings.language as 'zh-CN' | 'en-US')
        }
        onClose()
    }

    const handleSave = async () => {
        if (!settings) return
        setSaving(true)
        try {
            await SaveSettings(settings)
            onSaved(settings)
            onClose()
        } catch (e) {
            console.error('Failed to save settings:', e)
        } finally {
            setSaving(false)
        }
    }

    const handleSelectFolder = async () => {
        const dir = await SelectFolder()
        if (dir) {
            setSettings({...settings, outputDir: dir})
        }
    }

    const update = (key: keyof Settings, value: any) => {
        setSettings({...settings, [key]: value})
    }

    const renderDependencyCard = ({
        title,
        status,
        tone,
        rows,
        actions,
        note,
        guide,
    }: {
        title: string
        status: string
        tone: 'ready' | 'missing' | 'loading'
        rows?: Array<{label: string; value?: string}>
        actions?: JSX.Element
        note?: string | null
        guide?: JSX.Element | null
    }) => {
        const visibleRows = (rows || []).filter(row => row.value)

        return (
            <div className="dep-card">
                <div className="dep-card-header">
                    <div className="dep-card-heading">
                        <div className="dep-card-title">{title}</div>
                        <div className="dep-card-subtitle">{t('settings.diagStatus')}</div>
                    </div>
                    <span className={`dep-badge dep-badge-${tone}`}>{status}</span>
                </div>

                {visibleRows.length > 0 && (
                    <div className="dep-card-grid">
                        {visibleRows.map(row => (
                            <div key={`${title}-${row.label}`} className="dep-meta-card">
                                <span className="dep-meta-label">{row.label}</span>
                                <span className="dep-meta-value">{row.value}</span>
                            </div>
                        ))}
                    </div>
                )}

                {actions && <div className="dep-card-actions">{actions}</div>}

                {note && (
                    <div className="dep-card-note">
                        <span className="dep-card-note-label">Output</span>
                        <span className="dep-card-note-value">{note}</span>
                    </div>
                )}

                {guide}
            </div>
        )
    }

    const renderDownloadTab = () => (
        <>
            <div className="setting-item">
                <label className="setting-label">{t('settings.outputDir')}</label>
                <div className="setting-row">
                    <input
                        type="text"
                        className="setting-input flex-1"
                        value={settings.outputDir}
                        onChange={e => update('outputDir', e.target.value)}
                    />
                    <button className="btn-secondary btn-sm" onClick={handleSelectFolder}>
                        {t('outputDir.browse')}
                    </button>
                </div>
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.quality')}</label>
                <select
                    className="setting-select"
                    value={settings.quality}
                    onChange={e => update('quality', e.target.value)}
                >
                    {QUALITY_OPTIONS.map(q => (
                        <option key={q} value={q}>{t(`quality.${q}` as any)}</option>
                    ))}
                </select>
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.rateLimit')}</label>
                <input
                    type="text"
                    className="setting-input"
                    value={settings.rateLimit}
                    onChange={e => update('rateLimit', e.target.value)}
                    placeholder="e.g. 1M, 500K (empty = unlimited)"
                />
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.maxConcurrent')}</label>
                <input
                    type="number"
                    className="setting-input setting-input-sm"
                    value={settings.maxConcurrent}
                    min={1}
                    max={10}
                    onChange={e => update('maxConcurrent', parseInt(e.target.value) || 1)}
                />
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.filenameTemplate')}</label>
                <input
                    type="text"
                    className="setting-input"
                    value={settings.filenameTemplate || ''}
                    onChange={e => update('filenameTemplate', e.target.value)}
                    placeholder="%(title)s.%(ext)s"
                />
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.mergeOutputFormat')}</label>
                <select
                    className="setting-select"
                    value={settings.mergeOutputFormat || ''}
                    onChange={e => update('mergeOutputFormat', e.target.value)}
                >
                    <option value="">{t('settings.mergeOutputFormatAuto')}</option>
                    <option value="mp4">MP4</option>
                    <option value="mkv">MKV</option>
                    <option value="webm">WebM</option>
                </select>
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.audioFormat')}</label>
                <select
                    className="setting-select"
                    value={settings.audioFormat || ''}
                    onChange={e => update('audioFormat', e.target.value)}
                >
                    <option value="">{t('settings.audioFormatDefault')}</option>
                    <option value="mp3">MP3</option>
                    <option value="m4a">M4A</option>
                    <option value="opus">Opus</option>
                    <option value="flac">FLAC</option>
                    <option value="wav">WAV</option>
                </select>
            </div>
        </>
    )

    const renderMediaTab = () => (
        <>
            <div className="setting-item setting-item-row">
                <label className="setting-label">{t('settings.saveDescription')}</label>
                <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={settings.saveDescription || false}
                    onChange={e => update('saveDescription', e.target.checked)}
                />
            </div>
            <div className="setting-item setting-item-row">
                <label className="setting-label">{t('settings.saveThumbnail')}</label>
                <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={settings.saveThumbnail || false}
                    onChange={e => update('saveThumbnail', e.target.checked)}
                />
            </div>
            <div className="setting-item setting-item-row">
                <label className="setting-label">{t('settings.writeSubtitles')}</label>
                <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={settings.writeSubtitles || false}
                    onChange={e => update('writeSubtitles', e.target.checked)}
                />
            </div>
            {settings.writeSubtitles && (
                <>
                    <div className="setting-item">
                        <label className="setting-label">{t('settings.subtitleLangs')}</label>
                        <input
                            type="text"
                            className="setting-input"
                            value={settings.subtitleLangs || ''}
                            onChange={e => update('subtitleLangs', e.target.value)}
                            placeholder="e.g. en,zh-Hans,ja (empty = all)"
                        />
                    </div>
                    <div className="setting-item setting-item-row">
                        <label className="setting-label">{t('settings.embedSubtitles')}</label>
                        <input
                            type="checkbox"
                            className="setting-checkbox"
                            checked={settings.embedSubtitles || false}
                            onChange={e => update('embedSubtitles', e.target.checked)}
                        />
                    </div>
                </>
            )}
            <div className="setting-item setting-item-row">
                <label className="setting-label">{t('settings.embedChapters')}</label>
                <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={settings.embedChapters || false}
                    onChange={e => update('embedChapters', e.target.checked)}
                />
            </div>
            <div className="setting-item setting-item-row">
                <label className="setting-label">{t('settings.sponsorBlock')}</label>
                <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={settings.sponsorBlock || false}
                    onChange={e => update('sponsorBlock', e.target.checked)}
                />
            </div>
        </>
    )

    const renderNetworkTab = () => (
        <>
            <div className="setting-item">
                <label className="setting-label">{t('settings.proxy')}</label>
                <input
                    type="text"
                    className="setting-input"
                    value={settings.proxy}
                    onChange={e => update('proxy', e.target.value)}
                    placeholder="http://127.0.0.1:7890 or socks5://127.0.0.1:1080"
                />
            </div>
            
            {/* Cookies - Option 1: From browser */}
            <div className="setting-item">
                <label className="setting-label">{t('settings.cookiesFrom')}</label>
                <select
                    className="setting-select"
                    value={settings.cookiesFrom || ''}
                    onChange={e => {
                        const val = e.target.value
                        setSettings(prev => prev ? {...prev, cookiesFrom: val, cookiesFile: val ? '' : prev.cookiesFile} : prev)
                    }}
                >
                    <option value="">{t('settings.cookiesFromNone')}</option>
                    <option value="chrome">Chrome</option>
                    <option value="firefox">Firefox</option>
                    <option value="edge">Edge</option>
                    <option value="opera">Opera</option>
                    <option value="brave">Brave</option>
                    <option value="vivaldi">Vivaldi</option>
                    <option value="safari">Safari</option>
                </select>
            </div>

            {/* Divider - only show when neither is selected */}
            {(!settings.cookiesFrom && !settings.cookiesFile) && (
                <div className="setting-divider">
                    <span>{t('setup.or')}</span>
                </div>
            )}

            {/* Cookies - Option 2: From file - hidden when browser is selected */}
            {settings.cookiesFrom ? null : (
                <div className="setting-item">
                    <label className="setting-label">{t('settings.cookiesFile')}</label>
<div className="setting-row">
                        <input
                            type="text"
                            className="setting-input flex-1"
                            value={settings.cookiesFile || ''}
                            onChange={e => {
                                const val = e.target.value
                                setSettings(prev => prev ? {...prev, cookiesFile: val, cookiesFrom: val ? '' : prev.cookiesFrom} : prev)
                            }}
                            placeholder={t('settings.cookiesFilePlaceholder')}
                        />
                        <button
                            className="btn-secondary btn-sm"
                            onClick={async () => {
                                const file = await SelectCookiesFile()
                                if (file) {
                                    setSettings(prev => prev ? {...prev, cookiesFile: file, cookiesFrom: ''} : prev)
                                }
                            }}
                        >
                            {t('outputDir.browse')}
                        </button>
                    </div>
                    <div className="setting-hint">
                        <a 
                            href="https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc"
                            target="_blank"
 rel="noopener noreferrer"
                        >
                            {t('setup.getExtension')}
                        </a>
                    </div>
                </div>
            )}
        </>
    )

    const renderDepsTab = () => (
        <>
            <div className="setting-item">
                <div className="tools-btn-row">
                    <button
                        className="btn-secondary btn-sm"
                        onClick={handleRefreshDeps}
                        disabled={loadingDeps}
                    >
                        {loadingDeps ? t('dep.checking') : t('dep.refresh')}
                    </button>
                </div>
            </div>

            <div className="dep-panel">
                {renderDependencyCard({
                    title: 'yt-dlp',
                    status: !depStatus
                        ? t('dep.checking')
                        : loadingDeps && !depStatus
                        ? t('dep.checking')
                        : depStatus?.ytdlp.found
                            ? `✓ ${t('dep.found')}`
                            : `✗ ${t('dep.notFound')}`,
                    tone: !depStatus ? 'loading' : depStatus?.ytdlp.found ? 'ready' : 'missing',
                    rows: [
                        {label: t('settings.diagVersion'), value: depStatus?.ytdlp.version},
                        {label: t('settings.diagPath'), value: depStatus?.ytdlp.path},
                    ],
                    actions: (
                        <button
                            className="btn-secondary btn-sm"
                            onClick={handleUpdateYtDlp}
                            disabled={isUpdatingYtDlp}
                        >
                            {isUpdatingYtDlp ? t('settings.ytdlpUpdating') : t('settings.ytdlpUpdate')}
                        </button>
                    ),
                    note: updateResult,
                })}

                {renderDependencyCard({
                    title: 'FFmpeg',
                    status: !depStatus ? t('dep.checking') : depStatus.ffmpeg.found ? `✓ ${t('dep.found')}` : `✗ ${t('dep.notFound')}`,
                    tone: !depStatus ? 'loading' : depStatus.ffmpeg.found ? 'ready' : 'missing',
                    rows: [
                        {label: t('settings.diagVersion'), value: depStatus?.ffmpeg.version},
                        {label: t('settings.diagPath'), value: depStatus?.ffmpeg.path},
                    ],
                    guide: depStatus && !depStatus.ffmpeg.found ? (
                        <div className="dep-card-callout">
                            <div className="dep-card-callout-title">{t('dep.ffmpegInstallGuide')}</div>
                            <code className="install-code">{t('dep.ffmpegWindows')}</code>
                            <code className="install-code">{t('dep.ffmpegMac')}</code>
                        </div>
                    ) : null,
                })}

                {renderDependencyCard({
                    title: `${t('dep.jsRuntime')} (Deno / Node)`,
                    status: !depStatus
                        ? t('dep.checking')
                        : depStatus.jsRuntime.found
                        ? `✓ ${depStatus.jsRuntimeName || 'deno/node'} ${t('dep.found')}`
                        : `✗ ${t('dep.notFound')}`,
                    tone: !depStatus ? 'loading' : depStatus.jsRuntime.found ? 'ready' : 'missing',
                    rows: [
                        {label: t('settings.diagVersion'), value: depStatus?.jsRuntime.version},
                        {label: t('settings.diagPath'), value: depStatus?.jsRuntime.path},
                    ],
                    actions: (
                        <button
                            className="btn-secondary btn-sm"
                            onClick={handleUpdateDeno}
                            disabled={isUpdatingDeno}
                        >
                            {isUpdatingDeno ? t('dep.denoUpdating') : t('dep.denoManage')}
                        </button>
                    ),
                    note: denoUpdateResult,
                    guide: depStatus && !depStatus.jsRuntime.found ? (
                        <div className="dep-card-callout">
                            <div className="dep-card-callout-title">{t('dep.denoInstallGuide')}</div>
                            <code className="install-code">{t('dep.denoWindows')}</code>
                            <code className="install-code">{t('dep.denoMac')}</code>
                        </div>
                    ) : null,
                })}
            </div>
        </>
    )

    const renderToolsTab = () => (
        <>
            {/* App Update */}
            <div className="setting-item">
                <label className="setting-label">{t('settings.appUpdate')}</label>
                <div className="tools-btn-row">
                    <button
                        className="btn-primary btn-sm"
                        onClick={handleCheckForUpdate}
                        disabled={isCheckingUpdate}
                    >
                        {isCheckingUpdate ? t('settings.appUpdateChecking') : t('settings.appUpdateCheck')}
                    </button>
                </div>
                {updateInfo && (
                    <div className="diagnostic-info" style={{marginTop: 12}}>
                        {updateInfo.hasUpdate ? (
                            <>
                                <div className="diag-item">
                                    <span className="diag-label">Current:</span>
                                    <span className="diag-value">v{updateInfo.currentVersion}</span>
                                </div>
                                <div className="diag-item">
                                    <span className="diag-label">Latest:</span>
                                    <span className="diag-value text-green-400">v{updateInfo.latestVersion}</span>
                                </div>
                                <button
                                    className="btn-primary btn-sm"
                                    style={{marginTop: 8}}
                                    onClick={handleOpenReleasePage}
                                >
                                    {t('settings.appUpdateDownload')}
                                </button>
                            </>
                        ) : (
                            <div className="diag-item">
                                <span className="diag-value text-green-400">✓ {t('settings.appUpdateUpToDate')} (v{updateInfo.currentVersion})</span>
                            </div>
                        )}
                    </div>
                )}
            </div>
            {/* App version */}
            {diagnostic && diagnostic.appVersion && (
                <div className="setting-item setting-item-row">
                    <label className="setting-label">{t('settings.appVersion')}</label>
                    <span className="diag-value">v{diagnostic.appVersion}</span>
                </div>
            )}
            {/* Reset settings section */}
            <div className="setting-item">
                <label className="setting-label">{t('settings.reset')}</label>
                <div className="tools-btn-row">
                    <button
                        className="btn-secondary btn-sm"
                        onClick={handleResetSettings}
                        disabled={isResetting}
                    >
                        {isResetting ? t('settings.resetting') : t('settings.reset')}
                    </button>
                </div>
            </div>
            {aboutInfo && (
                <div className="setting-item">
                    <label className="setting-label">{t('settings.about')}</label>
                    <div className="diagnostic-info about-info">
                        <div className="diag-item">
                            <span className="diag-label">{t('settings.appVersion')}:</span>
                            <span className="diag-value">v{aboutInfo.appVersion}</span>
                        </div>
                        <div className="diag-item">
                            <span className="diag-label">{t('settings.systemVersion')}:</span>
                            <span className="diag-value">{aboutInfo.systemVersion}</span>
                        </div>
                        <div className="diag-item">
                            <span className="diag-label">{t('settings.github')}:</span>
                            <a className="diag-link" href={aboutInfo.githubUrl} target="_blank" rel="noopener noreferrer">
                                {aboutInfo.githubRepo}
                            </a>
                        </div>
                        <div className="diag-item">
                            <span className="diag-label">{t('settings.authorEmail')}:</span>
                            <a className="diag-link" href={`mailto:${aboutInfo.authorEmail}`}>
                                {aboutInfo.authorEmail}
                            </a>
                        </div>
                    </div>
                </div>
            )}
        </>
    )

    const renderAppearanceTab = () => (
        <>
            <div className="setting-item">
                <label className="setting-label">{t('settings.theme')}</label>
                <select
                    className="setting-select"
                    value={settings.theme || 'dark'}
                    onChange={e => update('theme', e.target.value)}
                >
                    {THEME_OPTIONS.map(th => (
                        <option key={th} value={th}>{t(`app.theme.${th}` as any)}</option>
                    ))}
                </select>
            </div>
            <div className="setting-item">
                <label className="setting-label">{t('settings.language')}</label>
                <select
                    className="setting-select"
                    value={settings.language || lang}
                    onChange={e => update('language', e.target.value)}
                >
                    {LANGUAGE_OPTIONS.map(l => (
                        <option key={l} value={l}>{l === 'zh-CN' ? t('settings.langZh') : t('settings.langEn')}</option>
                    ))}
                </select>
            </div>
            <div className="setting-item setting-item-row">
                <label className="setting-label">{t('settings.notifications')}</label>
                <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={settings.notifications}
                    onChange={e => update('notifications', e.target.checked)}
                />
            </div>
        </>
    )

    const tabContent: Record<SettingsTab, () => JSX.Element> = {
        download: renderDownloadTab,
        media: renderMediaTab,
        network: renderNetworkTab,
        deps: renderDepsTab,
        tools: renderToolsTab,
        appearance: renderAppearanceTab,
    }

    return (
        <div className="dialog-overlay" onClick={handleDismiss}>
            <div className="dialog-content settings-dialog" onClick={e => e.stopPropagation()}>
                <div className="dialog-header">
                    <h2>{t('settings.title')}</h2>
                    <button className="btn-ghost btn-sm" onClick={handleDismiss}>✕</button>
                </div>
                <div className="settings-tabs">
                    {TAB_KEYS.map(tab => (
                        <button
                            key={tab}
                            className={`settings-tab${activeTab === tab ? ' settings-tab-active' : ''}`}
                            onClick={() => setActiveTab(tab)}
                        >
                            {t(`settings.tab.${tab}` as any)}
                        </button>
                    ))}
                </div>
                <div className="dialog-body">
                    {tabContent[activeTab]()}
                </div>
                <div className="dialog-footer">
                    <button className="btn-secondary" onClick={handleDismiss}>{t('action.cancel')}</button>
                    <button className="btn-primary" onClick={handleSave} disabled={saving}>
                        {saving ? t('settings.saving') : t('settings.save')}
                    </button>
                </div>
            </div>
        </div>
    )
}

export default SettingsDialog
