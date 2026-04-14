import {useState} from 'react'
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
    const formatDate = (dateStr: string) => {
        if (!dateStr) return ''
        const date = new Date(dateStr)
        return date.toLocaleDateString()
    }

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
                <span>Checking for updates...</span>
            </div>
        )
    }

    // Error State
    if (error) {
        return (
            <div className="update-banner error">
                <span className="update-banner-icon">⚠️</span>
                <span className="update-banner-msg">{error}</span>
                <button className="update-banner-btn" onClick={handleRetry}>Retry</button>
                <button className="update-banner-btn-close" onClick={onClose}>×</button>
            </div>
        )
    }

    // No Update Available
    if (updateInfo && !updateInfo.hasUpdate) {
        return null // Silent - no notification when up to date
    }

    // Update Available - Banner at top
    if (updateInfo && updateInfo.hasUpdate) {
        return (
            <div className="update-banner available">
                <div className="update-banner-content">
                    <span className="update-banner-icon">⬆️</span>
                    <span className="update-banner-text">
                        New version <strong>v{updateInfo.latestVersion}</strong> available!
                    </span>
                </div>
                <div className="update-banner-actions">
                    <button className="update-banner-btn-later" onClick={onClose}>Later</button>
                    <button className="update-banner-btn-download" onClick={handleDownload}>
                        ⬇️ Download
                    </button>
                </div>
            </div>
        )
    }

    return null
}

export default UpdateDialog
