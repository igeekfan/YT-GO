import {useState, useEffect, useCallback} from 'react'
import {BrowseDir, BrowseDirResult} from '../lib/backend'
import {useI18n} from '../i18n/context'

interface Props {
    open: boolean
    initialPath: string
    onSelect: (path: string) => void
    onClose: () => void
}

function DirBrowser({open, initialPath, onSelect, onClose}: Props) {
    const {t} = useI18n()
    const [currentPath, setCurrentPath] = useState(initialPath || '/')
    const [dirs, setDirs] = useState<string[]>([])
    const [parentPath, setParentPath] = useState('')
    const [homeDir, setHomeDir] = useState('')
    const [loading, setLoading] = useState(false)
    const [manualPath, setManualPath] = useState('')

    const loadDir = useCallback(async (path: string) => {
        setLoading(true)
        try {
            const result = await BrowseDir(path)
            setCurrentPath(result.path)
            setDirs(result.dirs || [])
            setParentPath(result.parent || '')
            setManualPath(result.path)
            if (result.homeDir) setHomeDir(result.homeDir)
        } catch {
            setDirs([])
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        if (open) {
            loadDir(initialPath || homeDir || '/')
        }
    }, [open, initialPath, homeDir, loadDir])

    if (!open) return null

    const handleNavigate = (dir: string) => {
        const newPath = currentPath === '/' ? `/${dir}` : `${currentPath}/${dir}`
        loadDir(newPath)
    }

    const handleGoParent = () => {
        if (parentPath && parentPath !== currentPath) {
            loadDir(parentPath)
        }
    }

    const handleGoHome = () => {
        if (homeDir) loadDir(homeDir)
    }

    const handleManualGo = () => {
        const path = manualPath.trim()
        if (path) loadDir(path)
    }

    const handleSelect = () => {
        onSelect(currentPath)
        onClose()
    }

    return (
        <div className="dialog-overlay" onClick={onClose}>
            <div className="dir-browser" onClick={e => e.stopPropagation()}>
                <div className="dir-browser-header">
                    <h3>{t('dirBrowser.title')}</h3>
                    <button className="btn-ghost btn-sm" onClick={onClose}>✕</button>
                </div>

                <div className="dir-browser-path">
                    <input
                        className="dir-browser-path-input"
                        type="text"
                        value={manualPath}
                        onChange={e => setManualPath(e.target.value)}
                        onKeyDown={e => { if (e.key === 'Enter') handleManualGo() }}
                        placeholder="/path/to/directory"
                    />
                    <button className="btn-secondary btn-sm" onClick={handleManualGo}>
                        {t('dirBrowser.go')}
                    </button>
                </div>

                <div className="dir-browser-nav">
                    <button
                        className="btn-ghost btn-sm"
                        onClick={handleGoParent}
                        disabled={!parentPath || parentPath === currentPath}
                    >
                        ↑ {t('dirBrowser.parent')}
                    </button>
                    {homeDir && (
                        <button className="btn-ghost btn-sm" onClick={handleGoHome}>
                            🏠 {t('dirBrowser.home')}
                        </button>
                    )}
                    <span className="dir-browser-current">{currentPath}</span>
                </div>

                <div className="dir-browser-list">
                    {loading ? (
                        <div className="dir-browser-loading">{t('dirBrowser.loading')}</div>
                    ) : dirs.length === 0 ? (
                        <div className="dir-browser-empty">{t('dirBrowser.empty')}</div>
                    ) : (
                        dirs.map(dir => (
                            <button
                                key={dir}
                                className="dir-browser-item"
                                onClick={() => handleNavigate(dir)}
                            >
                                📁 {dir}
                            </button>
                        ))
                    )}
                </div>

                <div className="dir-browser-footer">
                    <button className="btn-secondary" onClick={onClose}>
                        {t('action.cancel')}
                    </button>
                    <button className="btn-primary" onClick={handleSelect}>
                        {t('dirBrowser.select')} "{currentPath}"
                    </button>
                </div>
            </div>
        </div>
    )
}

export default DirBrowser
