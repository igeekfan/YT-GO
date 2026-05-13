import {useState, useEffect, useCallback} from 'react'
import {BrowseDir, BrowseDirResult} from '../lib/backend'
import {useI18n} from '../i18n/context'
import {Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter} from '@/components/ui/dialog'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {ScrollArea} from '@/components/ui/scroll-area'
import {FolderOpen, ArrowUp, Home, Folder} from 'lucide-react'

interface Props {
    open: boolean; initialPath: string; onSelect: (path: string) => void; onClose: () => void
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
            setCurrentPath(result.path); setDirs(result.dirs || [])
            setParentPath(result.parent || ''); setManualPath(result.path)
            if (result.homeDir) setHomeDir(result.homeDir)
        } catch { setDirs([]) }
        finally { setLoading(false) }
    }, [])

    useEffect(() => { if (open) loadDir(initialPath || homeDir || '/') }, [open, initialPath, homeDir, loadDir])

    const handleNavigate = (dir: string) => {
        loadDir(currentPath === '/' ? `/${dir}` : `${currentPath}/${dir}`)
    }

    const handleSelect = () => { onSelect(currentPath); onClose() }

    return (
        <Dialog open={open} onOpenChange={(v: boolean) => { if (!v) onClose() }}>
            <DialogContent className="max-w-md">
                <DialogHeader>
                    <DialogTitle>{t('dirBrowser.title')}</DialogTitle>
                </DialogHeader>
                <div className="space-y-3">
                    <div className="flex gap-2">
                        <Input type="text" value={manualPath} onChange={e => setManualPath(e.target.value)}
                            onKeyDown={e => { if (e.key === 'Enter') { const p = manualPath.trim(); if (p) loadDir(p) } }}
                            placeholder="/path/to/directory" className="font-mono text-xs" />
                        <Button variant="outline" size="sm" onClick={() => { const p = manualPath.trim(); if (p) loadDir(p) }}>
                            {t('dirBrowser.go')}
                        </Button>
                    </div>
                    <div className="flex items-center gap-2">
                        <Button variant="ghost" size="sm" onClick={() => { if (parentPath && parentPath !== currentPath) loadDir(parentPath) }}
                            disabled={!parentPath || parentPath === currentPath} className="text-xs">
                            <ArrowUp className="h-3 w-3 mr-1" />{t('dirBrowser.parent')}
                        </Button>
                        {homeDir && (
                            <Button variant="ghost" size="sm" onClick={() => loadDir(homeDir)} className="text-xs">
                                <Home className="h-3 w-3 mr-1" />{t('dirBrowser.home')}
                            </Button>
                        )}
                        <span className="flex-1 text-right text-xs text-muted-foreground font-mono truncate">{currentPath}</span>
                    </div>
                    <ScrollArea className="h-64 rounded-md border">
                        {loading ? (
                            <div className="flex items-center justify-center h-full text-sm text-muted-foreground">{t('dirBrowser.loading')}</div>
                        ) : dirs.length === 0 ? (
                            <div className="flex items-center justify-center h-full text-sm text-muted-foreground">{t('dirBrowser.empty')}</div>
                        ) : (
                            <div className="p-1">
                                {dirs.map(dir => (
                                    <Button key={dir} variant="ghost" className="w-full justify-start text-sm h-8 font-normal"
                                        onClick={() => handleNavigate(dir)}>
                                        <Folder className="h-4 w-4 mr-2 text-muted-foreground" />{dir}
                                    </Button>
                                ))}
                            </div>
                        )}
                    </ScrollArea>
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={onClose}>{t('action.cancel')}</Button>
                    <Button onClick={handleSelect}>{t('dirBrowser.select')} "{currentPath}"</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}

export default DirBrowser
