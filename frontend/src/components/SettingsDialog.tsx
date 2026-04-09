import {useState, useEffect} from 'react'
import {Settings} from '../types'
import {useI18n} from '../i18n/context'
import {GetSettings, SaveSettings, SelectFolder, SelectCookiesFile, GetDiagnosticInfo} from '../../wailsjs/go/main/App'

interface DiagnosticInfo {
    ytdlpPath: string
    ytdlpVersion: string
    ytdlpFound: boolean
    testOutput: string
    error: string
}

interface Props {
    open: boolean
    onClose: () => void
    onSaved: (settings: Settings) => void
}

const QUALITY_OPTIONS = ['best', '1080p', '720p', '480p', '360p', 'audio']
const THEME_OPTIONS = ['dark', 'light']
const LANGUAGE_OPTIONS = ['zh-CN', 'en-US']

function SettingsDialog({open, onClose, onSaved}: Props) {
    const {t, lang, setLang} = useI18n()
    const [settings, setSettings] = useState<Settings | null>(null)
    const [saving, setSaving] = useState(false)
    const [diagnostic, setDiagnostic] = useState<DiagnosticInfo | null>(null)
    const [loadingDiag, setLoadingDiag] = useState(false)

    useEffect(() => {
        if (open) {
            GetSettings().then(setSettings).catch(console.error)
            setDiagnostic(null)
        }
    }, [open])

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

    if (!open || !settings) return null

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

    return (
        <div className="dialog-overlay" onClick={onClose}>
            <div className="dialog-content settings-dialog" onClick={e => e.stopPropagation()}>
                <div className="dialog-header">
                    <h2>{t('settings.title')}</h2>
                    <button className="btn-ghost btn-sm" onClick={onClose}>✕</button>
                </div>
                <div className="dialog-body">
                    {/* Output Directory */}
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

                    {/* Default Quality */}
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

                    {/* Theme */}
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

                    {/* Language */}
                    <div className="setting-item">
                        <label className="setting-label">{t('settings.language')}</label>
                        <select
                            className="setting-select"
                            value={settings.language || lang}
                            onChange={e => update('language', e.target.value)}
                        >
                            {LANGUAGE_OPTIONS.map(l => (
                                <option key={l} value={l}>{l === 'zh-CN' ? '中文' : 'English'}</option>
                            ))}
                        </select>
                    </div>

                    {/* Proxy */}
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

                    {/* Cookies from browser */}
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

                    {/* Cookies file */}
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
                                disabled={!!settings.cookiesFrom}
                            />
                            <button
                                className="btn-secondary btn-sm"
                                disabled={!!settings.cookiesFrom}
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
                    </div>

                    {/* Rate Limit */}
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

                    {/* Max Concurrent Downloads */}
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

                    {/* Notifications */}
                    <div className="setting-item setting-item-row">
                        <label className="setting-label">{t('settings.notifications')}</label>
                        <input
                            type="checkbox"
                            className="setting-checkbox"
                            checked={settings.notifications}
                            onChange={e => update('notifications', e.target.checked)}
                        />
                    </div>

                    {/* Diagnostic Info */}
                    <div className="setting-item">
                        <label className="setting-label">{t('settings.diagnostic') || '诊断信息'}</label>
                        <button
                            className="btn-secondary btn-sm mb-2"
                            onClick={handleGetDiagnostic}
                            disabled={loadingDiag}
                        >
                            {loadingDiag ? '检查中...' : '检查 yt-dlp 状态'}
                        </button>
                        {diagnostic && (
                            <div className="diagnostic-info">
                                <div className="diag-item">
                                    <span className="diag-label">yt-dlp 路径:</span>
                                    <span className="diag-value">{diagnostic.ytdlpPath || '未找到'}</span>
                                </div>
                                <div className="diag-item">
                                    <span className="diag-label">版本:</span>
                                    <span className="diag-value">{diagnostic.ytdlpVersion || '-'}</span>
                                </div>
                                <div className="diag-item">
                                    <span className="diag-label">状态:</span>
                                    <span className={`diag-value ${diagnostic.ytdlpFound ? 'text-green-400' : 'text-red-400'}`}>
                                        {diagnostic.ytdlpFound ? '✓ 可用' : '✗ 不可用'}
                                    </span>
                                </div>
                                {diagnostic.testOutput && (
                                    <div className="diag-item">
                                        <span className="diag-label">测试:</span>
                                        <span className="diag-value text-green-400">{diagnostic.testOutput}</span>
                                    </div>
                                )}
                                {diagnostic.error && (
                                    <div className="diag-item">
                                        <span className="diag-label">错误:</span>
                                        <span className="diag-value text-red-400">{diagnostic.error}</span>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
                <div className="dialog-footer">
                    <button className="btn-secondary" onClick={onClose}>{t('action.cancel')}</button>
                    <button className="btn-primary" onClick={handleSave} disabled={saving}>
                        {saving ? t('settings.saving') : t('settings.save')}
                    </button>
                </div>
            </div>
        </div>
    )
}

export default SettingsDialog
