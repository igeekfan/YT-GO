import {useState, useEffect} from 'react'
import {Settings} from '../types'
import {useI18n} from '../i18n/context'
import {SaveSettings, GetSettings, SelectFolder, SelectCookiesFile, GetDiagnosticInfo, UpdateYtDlp, UpdateDeno, ResetSettings, CheckForUpdate, OpenReleasePage, GetAboutInfo, GetDepStatus, CheckYtDlpVersion, backendMode, UploadCookiesFile, getWebConfig} from '../lib/backend'
import DirBrowser from './DirBrowser'
import {Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter} from '@/components/ui/dialog'
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {Checkbox} from '@/components/ui/checkbox'
import {Select as SelectComp, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'
import {Badge} from '@/components/ui/badge'
import {Separator} from '@/components/ui/separator'
import {ScrollArea} from '@/components/ui/scroll-area'
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip'
import {toast} from 'sonner'
import {FolderOpen, RefreshCw, Upload, ExternalLink, X, Globe, Heart, Info} from 'lucide-react'

interface DiagnosticInfo {
    ytdlpPath: string; ytdlpVersion: string; ytdlpFound: boolean
    ffmpegPath: string; ffmpegVersion: string; ffmpegFound: boolean
    nodeVersion: string; appVersion: string; testOutput: string; error: string
}

interface AboutInfo { appVersion: string; systemVersion: string; githubRepo: string; githubUrl: string; authorEmail: string }

interface DepItem { found: boolean; version: string; path: string }
interface DepStatus { ytdlp: DepItem; ffmpeg: DepItem; jsRuntime: DepItem; jsRuntimeName: string }

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

function SettingsDialog({open, initialSettings, onClose, onSaved, onThemePreview, onLanguagePreview}: Props) {
    const {t, lang} = useI18n()
    const [settings, setSettings] = useState<Settings | null>(null)
    const [diagnostic, setDiagnostic] = useState<DiagnosticInfo | null>(null)
    const [aboutInfo, setAboutInfo] = useState<AboutInfo | null>(null)
    const [loadingDiag, setLoadingDiag] = useState(false)
    const [isUpdatingYtDlp, setIsUpdatingYtDlp] = useState(false)
    const [updateResult, setUpdateResult] = useState<string | null>(null)
    const [isUpdatingDeno, setIsUpdatingDeno] = useState(false)
    const [denoUpdateResult, setDenoUpdateResult] = useState<string | null>(null)
    const [isResetting, setIsResetting] = useState(false)
    const [isCheckingYtDlpVersion, setIsCheckingYtDlpVersion] = useState(false)
    const [ytdlpVersionCheck, setYtdlpVersionCheck] = useState<{currentVersion: string; latestVersion: string; isLatest: boolean} | null>(null)
    const [isCheckingUpdate, setIsCheckingUpdate] = useState(false)
    const [depStatus, setDepStatus] = useState<DepStatus | null>(null)
    const [loadingDeps, setLoadingDeps] = useState(false)
    const [showDirBrowser, setShowDirBrowser] = useState(false)
    const [updateInfo, setUpdateInfo] = useState<{
        hasUpdate: boolean; currentVersion: string; latestVersion: string
        releaseName: string; releaseBody: string; htmlUrl: string; publishedAt: string
    } | null>(null)

    const handleCheckYtDlpVersion = async () => {
        setIsCheckingYtDlpVersion(true); setYtdlpVersionCheck(null)
        try { const result = await CheckYtDlpVersion(); setYtdlpVersionCheck(result as any) }
        catch (e) { console.error('Failed to check yt-dlp version:', e) }
        finally { setIsCheckingYtDlpVersion(false) }
    }

    const handleCheckForUpdate = async () => {
        setIsCheckingUpdate(true)
        try { const info = await CheckForUpdate(); setUpdateInfo(info) }
        catch (e: any) { setUpdateInfo({ hasUpdate: false, currentVersion: '0.0.1', latestVersion: '0.0.0', releaseName: '', releaseBody: e?.message || 'Failed', htmlUrl: '', publishedAt: '' }) }
        finally { setIsCheckingUpdate(false) }
    }

    const handleOpenReleasePage = async () => {
        try { await OpenReleasePage() } catch (e) { console.error('Failed to open release page:', e) }
    }

    useEffect(() => {
        if (open) {
            setSettings(initialSettings); setDiagnostic(null); setUpdateInfo(null)
            setDepStatus(null); setYtdlpVersionCheck(null)
            GetAboutInfo().then(setAboutInfo).catch(console.error)
        }
    }, [open])

    useEffect(() => {
        if (open && !depStatus && !loadingDeps) { handleRefreshDeps() }
    }, [open])

    useEffect(() => { if (open && settings?.theme) onThemePreview(settings.theme as 'dark' | 'light') }, [open, settings?.theme, onThemePreview])
    useEffect(() => { if (open && settings?.language) onLanguagePreview(settings.language as 'zh-CN' | 'en-US') }, [open, settings?.language, onLanguagePreview])

    const handleGetDiagnostic = async () => {
        setLoadingDiag(true)
        try { const info = await GetDiagnosticInfo(); setDiagnostic(info as DiagnosticInfo) }
        catch (e) { console.error('Failed to get diagnostic info:', e) }
        finally { setLoadingDiag(false) }
    }

    const handleRefreshDeps = async () => {
        setLoadingDeps(true)
        try { const status = await GetDepStatus(); setDepStatus(status as DepStatus) }
        catch (e) { console.error('Failed to get dep status:', e) }
        finally { setLoadingDeps(false) }
    }

    const handleUpdateYtDlp = async () => {
        setIsUpdatingYtDlp(true); setUpdateResult(null)
        try {
            const result = await UpdateYtDlp(); setUpdateResult(result || t('ytdlp.updateSuccess'))
            const info = await GetDiagnosticInfo(); setDiagnostic(info as DiagnosticInfo)
        } catch (e: any) { setUpdateResult(t('ytdlp.updateFail') + (e?.message ? `: ${e.message}` : '')) }
        finally { setIsUpdatingYtDlp(false) }
    }

    const handleUpdateDeno = async () => {
        setIsUpdatingDeno(true); setDenoUpdateResult(null)
        try {
            const result = await UpdateDeno(); setDenoUpdateResult(result || t('dep.denoUpdateSuccess'))
            const status = await GetDepStatus(); setDepStatus(status as DepStatus)
        } catch (e: any) { setDenoUpdateResult(t('dep.denoUpdateFail') + (e?.message ? `: ${e.message}` : '')) }
        finally { setIsUpdatingDeno(false) }
    }

    const handleResetSettings = async () => {
        setIsResetting(true)
        try { await ResetSettings(); toast.success(t('settings.resetSuccess')) }
        catch (e: any) { toast.error(e?.message || t('settings.resetFailed')) }
        finally { setIsResetting(false) }
    }

    if (!settings) return null

    const autoSave = (next: Settings) => {
        SaveSettings(next).then(() => onSaved(next)).catch(e => console.error('Failed to auto-save settings:', e))
    }

    const update = (key: keyof Settings, value: any) => {
        setSettings(prev => {
            if (!prev) return prev
            const next = {...prev, [key]: value}
            autoSave(next)
            if (key === 'theme') onThemePreview(value as 'dark' | 'light')
            if (key === 'language') onLanguagePreview(value as 'zh-CN' | 'en-US')
            return next
        })
    }

    const renderDepCard = ({title, status, tone, rows, actions, note, guide}: {
        title: string; status: string; tone: 'ready' | 'missing' | 'loading'
        rows?: Array<{label: string; value?: string}>; actions?: JSX.Element
        note?: string | null; guide?: JSX.Element | null
    }) => {
        const visibleRows = (rows || []).filter(row => row.value)
        return (
            <div className="rounded-lg border p-4 space-y-3">
                <div className="flex items-start justify-between gap-3">
                    <div>
                        <div className="font-semibold text-sm">{title}</div>
                        <div className="text-xs text-muted-foreground">{t('settings.diagStatus')}</div>
                    </div>
                    <Badge variant={tone === 'ready' ? 'secondary' : tone === 'missing' ? 'destructive' : 'outline'} className="text-xs">
                        {status}
                    </Badge>
                </div>
                {visibleRows.length > 0 && (
                    <div className="grid grid-cols-2 gap-2">
                        {visibleRows.map(row => (
                            <div key={`${title}-${row.label}`} className="rounded-md border p-2 space-y-0.5">
                                <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-medium">{row.label}</span>
                                <span className="text-xs break-all">{row.value}</span>
                            </div>
                        ))}
                    </div>
                )}
                {actions}
                {note && (
                    <div className="rounded-md border p-2">
                        <span className="text-[10px] text-muted-foreground uppercase tracking-wider">{t('settings.diagOutput')}</span>
                        <p className="text-xs mt-0.5 whitespace-pre-wrap break-words">{note}</p>
                    </div>
                )}
                {guide}
            </div>
        )
    }

    return (
        <>
        <Dialog open={open} onOpenChange={(v: boolean) => { if (!v) onClose() }}>
            <DialogContent className="max-w-2xl w-full h-[640px] max-h-[90vh] flex flex-col p-0 gap-0 rounded-2xl shadow-xl">
                <DialogHeader className="px-6 py-4 border-b border-primary/10">
                    <DialogTitle className="text-base font-bold tracking-tight">{t('settings.title')}</DialogTitle>
                </DialogHeader>
                <Tabs defaultValue="download" className="flex-1 flex flex-col min-h-0">
                    <TabsList className="w-full justify-start rounded-none border-b bg-transparent h-auto p-0 px-6">
                        {(['download', 'media', 'network', 'deps', 'appearance', 'about'] as const).map(tab => (
                            <TabsTrigger key={tab} value={tab} className="rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:shadow-none px-3 py-2.5 text-xs">
                                {t(`settings.tab.${tab}` as any)}
                            </TabsTrigger>
                        ))}
                    </TabsList>

                    <ScrollArea className="flex-1 min-h-0">
                        <div className="p-6 space-y-4">
                            {/* Download Tab */}
                            <TabsContent value="download" className="mt-0 space-y-4">
                                {!(backendMode === 'web' && getWebConfig()?.hasFixedDir) && (
                                    <div className="space-y-1.5">
                                        <Label className="text-xs text-muted-foreground">{t('settings.outputDir')}</Label>
                                        <div className="flex gap-2">
                                            <Input type="text" value={settings.outputDir} onChange={e => update('outputDir', e.target.value)}
                                                placeholder={backendMode === 'web' ? t('outputDir.serverPathPlaceholder') : undefined} />
                                            <Button variant="outline" size="sm" onClick={backendMode === 'desktop' ? async () => { const dir = await SelectFolder(); if (dir) update('outputDir', dir) } : () => setShowDirBrowser(true)}>
                                                <FolderOpen className="h-4 w-4 mr-1" />{t('outputDir.browse')}
                                            </Button>
                                        </div>
                                        {backendMode === 'web' && <p className="text-xs text-muted-foreground">{t('settings.outputDirWebHint')}</p>}
                                    </div>
                                )}
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.quality')}</Label>
                                    <SelectComp value={settings.quality} onValueChange={(v: string) => update('quality', v)}>
                                        <SelectTrigger><SelectValue /></SelectTrigger>
                                        <SelectContent>{QUALITY_OPTIONS.map(q => <SelectItem key={q} value={q}>{t(`quality.${q}` as any)}</SelectItem>)}</SelectContent>
                                    </SelectComp>
                                </div>
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.rateLimit')}</Label>
                                    <Input type="text" value={settings.rateLimit} onChange={e => update('rateLimit', e.target.value)} placeholder={t('settings.rateLimitPlaceholder')} />
                                </div>
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.maxConcurrent')}</Label>
                                    <Input type="number" value={settings.maxConcurrent} min={1} max={10} onChange={e => update('maxConcurrent', parseInt(e.target.value) || 1)} className="w-20" />
                                </div>
                                <div className="space-y-1.5">
                                    <div className="flex items-center gap-1.5">
                                        <Label className="text-xs text-muted-foreground">{t('settings.filenameTemplate')}</Label>
                                        <Tooltip>
                                            <TooltipTrigger><Info className="h-3 w-3 text-muted-foreground" /></TooltipTrigger>
                                            <TooltipContent className="max-w-xs text-xs space-y-1">
                                                <div>{t('settings.filenameTemplateHelp')}</div>
                                                <div className="font-mono text-[10px] break-all">%(title)s · %(uploader)s · %(upload_date)s · %(id)s · %(ext)s · %(playlist_index)s</div>
                                            </TooltipContent>
                                        </Tooltip>
                                    </div>
                                    <Input type="text" value={settings.filenameTemplate || ''} onChange={e => update('filenameTemplate', e.target.value)} placeholder="%(title)s.%(ext)s" />
                                    <div className="flex gap-1 flex-wrap pt-0.5">
                                        {([
                                            {label: t('settings.filenamePreset.title'), value: '%(title)s.%(ext)s'},
                                            {label: t('settings.filenamePreset.uploaderTitle'), value: '%(uploader)s - %(title)s.%(ext)s'},
                                            {label: t('settings.filenamePreset.dateTitle'), value: '%(upload_date)s_%(title)s.%(ext)s'},
                                            {label: t('settings.filenamePreset.titleId'), value: '%(title)s [%(id)s].%(ext)s'},
                                        ] as const).map(p => (
                                            <Button key={p.value} variant="outline" size="sm" className="h-6 text-[11px] px-2"
                                                onClick={() => update('filenameTemplate', p.value)}>
                                                {p.label}
                                            </Button>
                                        ))}
                                    </div>
                                </div>
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.mergeOutputFormat')}</Label>
                                    <SelectComp value={settings.mergeOutputFormat || '_auto'} onValueChange={(v: string) => update('mergeOutputFormat', v === '_auto' ? '' : v)}>
                                        <SelectTrigger><SelectValue /></SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="_auto">{t('settings.mergeOutputFormatAuto')}</SelectItem>
                                            <SelectItem value="mp4">MP4</SelectItem>
                                            <SelectItem value="mkv">MKV</SelectItem>
                                            <SelectItem value="webm">WebM</SelectItem>
                                        </SelectContent>
                                    </SelectComp>
                                </div>
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.audioFormat')}</Label>
                                    <SelectComp value={settings.audioFormat || '_default'} onValueChange={(v: string) => update('audioFormat', v === '_default' ? '' : v)}>
                                        <SelectTrigger><SelectValue /></SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="_default">{t('settings.audioFormatDefault')}</SelectItem>
                                            <SelectItem value="mp3">MP3</SelectItem>
                                            <SelectItem value="m4a">M4A</SelectItem>
                                            <SelectItem value="opus">Opus</SelectItem>
                                            <SelectItem value="flac">FLAC</SelectItem>
                                            <SelectItem value="wav">WAV</SelectItem>
                                        </SelectContent>
                                    </SelectComp>
                                </div>
                            </TabsContent>

                            {/* Media Tab */}
                            <TabsContent value="media" className="mt-0 space-y-3">
                                {([
                                    { key: 'saveDescription', label: t('settings.saveDescription') },
                                    { key: 'saveThumbnail', label: t('settings.saveThumbnail') },
                                    { key: 'writeSubtitles', label: t('settings.writeSubtitles') },
                                    ...(settings.writeSubtitles ? [
                                        { key: 'embedSubtitles', label: t('settings.embedSubtitles') },
                                    ] : []),
                                    { key: 'embedChapters', label: t('settings.embedChapters') },
                                    { key: 'sponsorBlock', label: t('settings.sponsorBlock') },
                                ] as const).map(opt => (
                                    <label key={opt.key} className="flex items-center justify-between cursor-pointer">
                                        <span className="text-sm cursor-pointer">{opt.label}</span>
                                        <Checkbox checked={!!settings[opt.key as keyof Settings]}
                                            onCheckedChange={(checked: boolean) => update(opt.key as keyof Settings, !!checked)} />
                                    </label>
                                ))}
                                {settings.writeSubtitles && (
                                    <div className="space-y-1.5">
                                        <Label className="text-xs text-muted-foreground">{t('settings.subtitleLangs')}</Label>
                                        <Input type="text" value={settings.subtitleLangs || ''} onChange={e => update('subtitleLangs', e.target.value)} placeholder={t('settings.subtitleLangsPlaceholder')} />
                                    </div>
                                )}
                            </TabsContent>

                            {/* Network Tab */}
                            <TabsContent value="network" className="mt-0 space-y-4">
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.proxy')}</Label>
                                    <Input type="text" value={settings.proxy} onChange={e => update('proxy', e.target.value)} placeholder={t('settings.proxyPlaceholder')} />
                                </div>
                                {backendMode === 'desktop' && (
                                    <div className="space-y-1.5">
                                        <Label className="text-xs text-muted-foreground">{t('settings.cookiesFrom')}</Label>
                                        <SelectComp value={settings.cookiesFrom || '_none'} onValueChange={(val: string) => {
                                            const v = val === '_none' ? '' : val
                                            setSettings(prev => {
                                                if (!prev) return prev
                                                const next = {...prev, cookiesFrom: v, cookiesFile: v ? '' : prev.cookiesFile}
                                                autoSave(next); return next
                                            })
                                        }}>
                                            <SelectTrigger><SelectValue /></SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="_none">{t('settings.cookiesFromNone')}</SelectItem>
                                                {['chrome', 'firefox', 'edge', 'opera', 'brave', 'vivaldi', 'safari'].map(b => <SelectItem key={b} value={b}>{b.charAt(0).toUpperCase() + b.slice(1)}</SelectItem>)}
                                            </SelectContent>
                                        </SelectComp>
                                    </div>
                                )}
                                {(!settings.cookiesFrom && !settings.cookiesFile) && <Separator><span className="text-xs text-muted-foreground">{t('setup.or')}</span></Separator>}
                                {!settings.cookiesFrom && (
                                    <div className="space-y-1.5">
                                        <Label className="text-xs text-muted-foreground">{t('settings.cookiesFile')}</Label>
                                        <div className="flex gap-2">
                                            <Input type="text" value={settings.cookiesFile || ''} onChange={e => {
                                                const val = e.target.value
                                                setSettings(prev => {
                                                    if (!prev) return prev
                                                    const next = {...prev, cookiesFile: val, cookiesFrom: val ? '' : prev.cookiesFrom}
                                                    autoSave(next); return next
                                                })
                                            }} placeholder={t('settings.cookiesFilePlaceholder')} />
                                            {backendMode === 'desktop' ? (
                                                <Button variant="outline" size="sm" onClick={async () => {
                                                    const file = await SelectCookiesFile()
                                                    if (file) setSettings(prev => { if (!prev) return prev; const next = {...prev, cookiesFile: file, cookiesFrom: ''}; autoSave(next); return next })
                                                }}>{t('outputDir.browse')}</Button>
                                            ) : (
                                                <Button variant="outline" size="sm" onClick={() => {
                                                    const input = document.createElement('input'); input.type = 'file'; input.accept = '.txt,.cookies'
                                                    input.onchange = async () => {
                                                        const file = input.files?.[0]; if (!file) return
                                                        try { const result = await UploadCookiesFile(file); setSettings(prev => { if (!prev) return prev; const next = {...prev, cookiesFile: result.path, cookiesFrom: ''}; autoSave(next); return next }) }
                                                        catch (err) { console.error('Failed to upload cookies file:', err) }
                                                    }; input.click()
                                                }}><Upload className="h-4 w-4 mr-1" />{t('action.upload')}</Button>
                                            )}
                                        </div>
                                        {backendMode === 'web' && <p className="text-xs text-muted-foreground">{t('settings.cookiesFileWebHint')}</p>}
                                        {backendMode === 'desktop' && (
                                            <a className="text-xs text-primary hover:underline" href="https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc" target="_blank" rel="noopener noreferrer">
                                                {t('setup.getExtension')} <ExternalLink className="h-3 w-3 inline" />
                                            </a>
                                        )}
                                    </div>
                                )}
                            </TabsContent>

                            {/* Deps Tab */}
                            <TabsContent value="deps" className="mt-0 space-y-4">
                                <Button variant="outline" size="sm" onClick={handleRefreshDeps} disabled={loadingDeps}>
                                    <RefreshCw className={`h-4 w-4 mr-1 ${loadingDeps ? 'animate-spin' : ''}`} />
                                    {loadingDeps ? t('dep.checking') : t('dep.refresh')}
                                </Button>
                                <div className="space-y-4">
                                    {renderDepCard({
                                        title: 'yt-dlp', status: !depStatus ? t('dep.checking') : depStatus.ytdlp.found ? `✓ ${t('dep.found')}` : `✗ ${t('dep.notFound')}`,
                                        tone: !depStatus ? 'loading' : depStatus.ytdlp.found ? 'ready' : 'missing',
                                        rows: [{label: t('settings.diagVersion'), value: depStatus?.ytdlp.version}, {label: t('settings.diagPath'), value: depStatus?.ytdlp.path}],
                                        actions: (
                                            <div className="flex gap-2 flex-wrap items-center">
                                                <Button variant="outline" size="sm" onClick={handleCheckYtDlpVersion} disabled={isCheckingYtDlpVersion || !depStatus?.ytdlp.found}>
                                                    {isCheckingYtDlpVersion ? t('dep.ytdlpCheckingVersion') : t('dep.ytdlpCheckVersion')}
                                                </Button>
                                                <Button variant="outline" size="sm" onClick={handleUpdateYtDlp} disabled={isUpdatingYtDlp}>
                                                    {isUpdatingYtDlp ? t('settings.ytdlpUpdating') : t('settings.ytdlpUpdate')}
                                                </Button>
                                                {ytdlpVersionCheck && (
                                                    <Badge variant={ytdlpVersionCheck.isLatest ? 'secondary' : 'destructive'} className="text-xs">
                                                        {ytdlpVersionCheck.isLatest ? t('dep.ytdlpLatest') : `${t('dep.ytdlpOutdated')}: ${ytdlpVersionCheck.latestVersion}`}
                                                    </Badge>
                                                )}
                                            </div>
                                        ), note: updateResult,
                                    })}
                                    {renderDepCard({
                                        title: 'FFmpeg', status: !depStatus ? t('dep.checking') : depStatus.ffmpeg.found ? `✓ ${t('dep.found')}` : `✗ ${t('dep.notFound')}`,
                                        tone: !depStatus ? 'loading' : depStatus.ffmpeg.found ? 'ready' : 'missing',
                                        rows: [{label: t('settings.diagVersion'), value: depStatus?.ffmpeg.version}, {label: t('settings.diagPath'), value: depStatus?.ffmpeg.path}],
                                        guide: depStatus && !depStatus.ffmpeg.found ? (
                                            <div className="rounded-md border border-dashed p-3 space-y-2">
                                                <div className="text-xs font-medium">{t('dep.ffmpegInstallGuide')}</div>
                                                <code className="block text-xs bg-muted p-1.5 rounded">{t('dep.ffmpegWindows')}</code>
                                                <code className="block text-xs bg-muted p-1.5 rounded">{t('dep.ffmpegMac')}</code>
                                            </div>
                                        ) : null,
                                    })}
                                    {renderDepCard({
                                        title: `${t('dep.jsRuntime')} (Deno / Node)`, status: !depStatus ? t('dep.checking') : depStatus.jsRuntime.found ? `✓ ${depStatus.jsRuntimeName || 'deno/node'} ${t('dep.found')}` : `✗ ${t('dep.notFound')}`,
                                        tone: !depStatus ? 'loading' : depStatus.jsRuntime.found ? 'ready' : 'missing',
                                        rows: [{label: t('settings.diagVersion'), value: depStatus?.jsRuntime.version}, {label: t('settings.diagPath'), value: depStatus?.jsRuntime.path}],
                                        actions: (
                                            <Button variant="outline" size="sm" onClick={handleUpdateDeno} disabled={isUpdatingDeno}>
                                                {isUpdatingDeno ? t('dep.denoUpdating') : t('dep.denoManage')}
                                            </Button>
                                        ), note: denoUpdateResult,
                                        guide: depStatus && !depStatus.jsRuntime.found ? (
                                            <div className="rounded-md border border-dashed p-3 space-y-2">
                                                <div className="text-xs font-medium">{t('dep.denoInstallGuide')}</div>
                                                <code className="block text-xs bg-muted p-1.5 rounded">{t('dep.denoWindows')}</code>
                                                <code className="block text-xs bg-muted p-1.5 rounded">{t('dep.denoMac')}</code>
                                            </div>
                                        ) : null,
                                    })}
                                </div>
                            </TabsContent>

                            {/* Appearance Tab */}
                            <TabsContent value="appearance" className="mt-0 space-y-4">
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.theme')}</Label>
                                    <SelectComp value={settings.theme || 'dark'} onValueChange={(v: string) => update('theme', v)}>
                                        <SelectTrigger><SelectValue /></SelectTrigger>
                                        <SelectContent>{THEME_OPTIONS.map(th => <SelectItem key={th} value={th}>{t(`app.theme.${th}` as any)}</SelectItem>)}</SelectContent>
                                    </SelectComp>
                                </div>
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.language')}</Label>
                                    <SelectComp value={settings.language || lang} onValueChange={(v: string) => update('language', v)}>
                                        <SelectTrigger><SelectValue /></SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="zh-CN">{t('settings.langZh')}</SelectItem>
                                            <SelectItem value="en-US">{t('settings.langEn')}</SelectItem>
                                        </SelectContent>
                                    </SelectComp>
                                </div>
                                <label className="flex items-center justify-between cursor-pointer">
                                    <span className="text-sm cursor-pointer">{t('settings.notifications')}</span>
                                    <Checkbox checked={!!settings.notifications} onCheckedChange={(checked: boolean) => update('notifications', !!checked)} />
                                </label>
                            </TabsContent>

                            {/* About Tab */}
                            <TabsContent value="about" className="mt-0">
                                <div className="flex flex-col items-center py-8 px-2">
                                    {/* Hero area with ambient glow */}
                                    <div className="relative mb-6">
                                        <div className="absolute inset-0 rounded-full blur-2xl bg-primary/15 scale-150 pointer-events-none" />
                                        <div className="relative flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-red-600 via-red-500 to-red-700 shadow-lg shadow-red-500/20">
                                            <svg width="36" height="36" viewBox="0 0 24 24" fill="white" className="drop-shadow-sm ml-0.5">
                                                <path d="M23.495 6.205a3.007 3.007 0 0 0-2.088-2.088c-1.87-.501-9.396-.501-9.396-.501s-7.507-.01-9.396.501A3.007 3.007 0 0 0 .527 6.205a31.247 31.247 0 0 0-.522 5.805 31.247 31.247 0 0 0 .522 5.783 3.007 3.007 0 0 0 2.088 2.088c1.868.502 9.396.502 9.396.502s7.506 0 9.396-.502a3.007 3.007 0 0 0 2.088-2.088 31.247 31.247 0 0 0 .5-5.783 31.247 31.247 0 0 0-.5-5.805zM9.609 15.601V8.408l6.264 3.602z"/>
                                            </svg>
                                        </div>
                                    </div>

                                    <div className="text-center space-y-1.5 mb-7">
                                        <h2 className="text-2xl font-bold tracking-tight">YT-GO</h2>
                                        {aboutInfo && (
                                            <span className="inline-block text-xs font-medium text-primary bg-primary/10 px-2.5 py-0.5 rounded-full tracking-wide">
                                                v{aboutInfo.appVersion}
                                            </span>
                                        )}
                                    </div>

                                    {aboutInfo && (
                                        <div className="w-full max-w-sm space-y-2.5">
                                            {/* Info card */}
                                            <div className="rounded-xl border bg-card/50 backdrop-blur-sm divide-y divide-border/50 overflow-hidden shadow-sm">
                                                <div className="flex items-center gap-3 px-4 py-3 group">
                                                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                                                        <Globe className="h-4 w-4" />
                                                    </div>
                                                    <div className="flex-1 min-w-0">
                                                        <div className="text-[10px] uppercase tracking-widest font-medium text-muted-foreground">{t('settings.github')}</div>
                                                        <a className="text-sm font-medium text-primary hover:underline break-all leading-snug" href={aboutInfo.githubUrl} target="_blank" rel="noopener noreferrer">
                                                            {aboutInfo.githubRepo} <ExternalLink className="h-3 w-3 inline opacity-50 group-hover:opacity-100 transition-opacity" />
                                                        </a>
                                                    </div>
                                                </div>

                                                <div className="flex items-center gap-3 px-4 py-3">
                                                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                                                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                            <rect x="2" y="3" width="20" height="14" rx="2"/>
                                                            <line x1="8" y1="21" x2="16" y2="21"/>
                                                            <line x1="12" y1="17" x2="12" y2="21"/>
                                                        </svg>
                                                    </div>
                                                    <div className="flex-1 min-w-0">
                                                        <div className="text-[10px] uppercase tracking-widest font-medium text-muted-foreground">{t('settings.systemVersion')}</div>
                                                        <div className="text-sm font-medium leading-snug break-all">{aboutInfo.systemVersion}</div>
                                                    </div>
                                                </div>

                                                <div className="flex items-center gap-3 px-4 py-3">
                                                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                                                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                                            <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
                                                            <polyline points="22,6 12,13 2,6"/>
                                                        </svg>
                                                    </div>
                                                    <div className="flex-1 min-w-0">
                                                        <div className="text-[10px] uppercase tracking-widest font-medium text-muted-foreground">{t('settings.authorEmail')}</div>
                                                        <a className="text-sm font-medium text-primary hover:underline leading-snug" href={`mailto:${aboutInfo.authorEmail}`}>{aboutInfo.authorEmail}</a>
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Check update */}
                                            <div className="flex flex-col items-center gap-2 pt-2">
                                                <Button size="sm" onClick={handleCheckForUpdate} disabled={isCheckingUpdate}>
                                                    <RefreshCw className={`h-4 w-4 mr-1 ${isCheckingUpdate ? 'animate-spin' : ''}`} />
                                                    {isCheckingUpdate ? t('settings.appUpdateChecking') : t('settings.appUpdateCheck')}
                                                </Button>
                                                {updateInfo?.hasUpdate && (
                                                    <Button size="sm" variant="outline" onClick={handleOpenReleasePage}>
                                                        {t('settings.appUpdateDownload')}
                                                    </Button>
                                                )}
                                                {updateInfo && (
                                                    <div className="rounded-md border p-3 text-xs space-y-1 w-full">
                                                        {updateInfo.hasUpdate ? (
                                                            <>
                                                                <div className="flex gap-2"><span className="text-muted-foreground">{t('settings.diagCurrent')}</span><span>v{updateInfo.currentVersion}</span></div>
                                                                <div className="flex gap-2"><span className="text-muted-foreground">{t('settings.diagLatest')}</span><span className="text-green-500">v{updateInfo.latestVersion}</span></div>
                                                            </>
                                                        ) : (
                                                            <span className="text-green-500">✓ {t('settings.appUpdateUpToDate')} (v{updateInfo.currentVersion})</span>
                                                        )}
                                                    </div>
                                                )}
                                            </div>

                                            {/* Reset */}
                                            <div className="flex flex-col items-center gap-2 pt-2">
                                                <Button variant="outline" size="sm" onClick={handleResetSettings} disabled={isResetting}>
                                                    {isResetting ? t('settings.resetting') : t('settings.reset')}
                                                </Button>
                                            </div>

                                            {/* Footer attribution */}
                                            <div className="text-center text-xs text-muted-foreground space-y-1.5 pt-3">
                                                <p className="flex items-center justify-center gap-1.5">
                                                    <Heart className="h-3.5 w-3.5 text-primary fill-primary/30" />
                                                    <span className="font-medium">{t('about.openSource')}</span>
                                                </p>
                                                <p className="opacity-70">{t('about.feedback')}</p>
                                            </div>
                                        </div>
                                    )}
                                </div>
                            </TabsContent>
                        </div>
                    </ScrollArea>
                </Tabs>
                <DialogFooter className="px-6 py-3 border-t">
                    <Button variant="outline" onClick={onClose}>{t('action.close')}</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
        <DirBrowser open={showDirBrowser} initialPath={settings?.outputDir || ''} onSelect={dir => update('outputDir', dir)} onClose={() => setShowDirBrowser(false)} />
        </>
    )
}

export default SettingsDialog
