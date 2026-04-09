import {useState, useEffect} from 'react'
import {Settings} from '../types'
import {useI18n} from '../i18n/context'
import {GetSettings, SaveSettings, SelectFolder} from '../../wailsjs/go/main/App'

interface Props {
    open: boolean
    onClose: () => void
    onSaved: (settings: Settings) => void
}

const QUALITY_OPTIONS = ['best', '1080p', '720p', '480p', '360p', 'audio']

function SettingsDialog({open, onClose, onSaved}: Props) {
    const {t} = useI18n()
    const [settings, setSettings] = useState<Settings | null>(null)
    const [saving, setSaving] = useState(false)

    useEffect(() => {
        if (open) {
            GetSettings().then(setSettings).catch(console.error)
        }
    }, [open])

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
