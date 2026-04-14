import {useState} from 'react'
import {useI18n} from '../i18n/context'
import './UpdateDialog.css'

interface UpdateInfo {
    hasUpdate: boolean
    currentVersion: string
    latestVersion: string
    releaseName: string
    releaseBody: string
    htmlUrl: string
    publishedAt: string
}

interface UpdateDialogProps {
    open: boolean
    updateInfo: UpdateInfo | null
    loading?: boolean
    error?: string | null
    onClose: () => void
    onOpenReleasePage: () => void
    onCheckUpdate: () => void
}

function UpdateDialog({open, updateInfo, loading, error, onClose, onOpenReleasePage, onCheckUpdate}: UpdateDialogProps) {
    const {t} = useI18n()

    const handleDownload = () => {
        onOpenReleasePage()
        onClose()
    }

    const handleRetry = () => {
        onCheckUpdate()
    }

    if (!open) return null

    // Loading State
    if (loading) {
        return (
            <div className="update-banner loading">
                <div className="update-spinner"></div>
                <span>{t('update.checking')}</span>
            </div>
        )
    }

    // Error State
    if (error) {
        return (
            <div className="update-banner error">
                <span className="update-banner-icon">⚠️</span>
                <span className="update-banner-msg">{error}</span>
                <button className="update-banner-btn" onClick={handleRetry}>{t('update.retry')}</button>
                <button className="update-banner-btn-close" onClick={onClose}>×</button>
            </div>
        )
    }

    // No Update Available
    if (updateInfo && !updateInfo.hasUpdate) {
        return null
    }

    // Update Available - Banner at top
    if (updateInfo && updateInfo.hasUpdate) {
        return (
            <div className="update-banner available">
                <div className="update-banner-content">
                    <span className="update-banner-icon">⬆️</span>
                    <span className="update-banner-text">
                        {t('update.available', {version: updateInfo.latestVersion})}
                    </span>
                </div>
                <div className="update-banner-actions">
                    <button className="update-banner-btn-later" onClick={onClose}>{t('update.later')}</button>
                    <button className="update-banner-btn-download" onClick={handleDownload}>
                        ⬇️ {t('update.download')}
                    </button>
                </div>
            </div>
        )
    }

    return null
}

export default UpdateDialog
