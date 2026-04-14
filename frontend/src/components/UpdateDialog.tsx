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
    onClose: () => void
    onOpenReleasePage: () => void
}

function UpdateDialog({open, updateInfo, onClose, onOpenReleasePage}: UpdateDialogProps) {
    if (!open || !updateInfo || !updateInfo.hasUpdate) return null

    const formatDate = (dateStr: string) => {
        if (!dateStr) return ''
        const date = new Date(dateStr)
        return date.toLocaleDateString()
    }

    const handleDownload = () => {
        onOpenReleasePage()
        onClose()
    }

    return (
        <div className="update-dialog-overlay" onClick={onClose}>
            <div className="update-dialog" onClick={e => e.stopPropagation()}>
                <div className="update-dialog-header">
                    <span className="update-icon">🎉</span>
                    <h2>New Version Available</h2>
                </div>
                
                <div className="update-dialog-body">
                    <div className="update-version-info">
                        <div className="update-version-row">
                            <span className="update-label">Current:</span>
                            <span className="update-value">{updateInfo.currentVersion}</span>
                        </div>
                        <div className="update-version-row">
                            <span className="update-label">Latest:</span>
                            <span className="update-value update-value-new">{updateInfo.latestVersion}</span>
                        </div>
                    </div>

                    {updateInfo.releaseBody && (
                        <div className="update-release-notes">
                            <h3>Release Notes:</h3>
                            <div className="update-notes-content">
                                {updateInfo.releaseBody.split('\n').map((line, i) => {
                                    const trimmed = line.trim()
                                    if (!trimmed) return null
                                    if (trimmed.startsWith('## ')) {
                                        return <h4 key={i}>{trimmed.slice(3)}</h4>
                                    }
                                    if (trimmed.startsWith('- ') || trimmed.startsWith('* ')) {
                                        return <li key={i}>{trimmed.slice(2)}</li>
                                    }
                                    return <p key={i}>{trimmed}</p>
                                })}
                            </div>
                        </div>
                    )}

                    {updateInfo.publishedAt && (
                        <div className="update-date">
                            Released: {formatDate(updateInfo.publishedAt)}
                        </div>
                    )}
                </div>

                <div className="update-dialog-footer">
                    <button className="btn-secondary" onClick={onClose}>
                        Later
                    </button>
                    <button className="btn-primary" onClick={handleDownload}>
                        Download Update
                    </button>
                </div>
            </div>
        </div>
    )
}

export default UpdateDialog
