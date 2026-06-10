import {useState, useEffect} from 'react'
import {SelectFolder, SelectCookiesFile, CheckYtDlp, GetDepStatus, backendMode, UploadCookiesFile, getWebConfig} from '../lib/backend'
import DirBrowser from './DirBrowser'
import {useI18n} from '../i18n/context'
import {Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter} from '@/components/ui/dialog'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'
import {Separator} from '@/components/ui/separator'
import {Badge} from '@/components/ui/badge'
import {FolderOpen, Upload, ExternalLink, CheckCircle2, AlertTriangle} from 'lucide-react'

const THEME_OPTIONS = ['dark', 'light'] as const
const LANGUAGE_OPTIONS = ['zh-CN', 'en-US'] as const

export default function SetupWizard({onComplete}: {onComplete: (outputDir: string, cookiesFrom: string, cookiesFile: string, proxy: string, language: 'zh-CN' | 'en-US', theme: 'dark' | 'light') => void}) {
    const {t, lang, setLang} = useI18n()
    const canBrowseLocalPaths = backendMode === 'desktop'
    const [step, setStep] = useState(1)
    const [outputDir, setOutputDir] = useState('')
    const [cookiesFrom, setCookiesFrom] = useState('')
    const [cookiesFile, setCookiesFile] = useState('')
    const [proxy, setProxy] = useState('')
    const [ytdlpOk, setYtdlpOk] = useState(false)
    const [denoOk, setDenoOk] = useState<boolean | null>(null)
    const [language, setLanguage] = useState<'zh-CN' | 'en-US'>(lang)
    const [theme, setTheme] = useState<'dark' | 'light'>(() => {
        const saved = localStorage.getItem('YT-GOto-theme') as 'dark' | 'light' | null
        if (saved) return saved
        return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
    })
    const [showDirBrowser, setShowDirBrowser] = useState(false)

    const webConfig = getWebConfig()
    const hasFixedDir = backendMode === 'web' && webConfig?.hasFixedDir

    useEffect(() => {
        if (hasFixedDir && webConfig?.downloadDir) setOutputDir(webConfig.downloadDir)
    }, [hasFixedDir, webConfig])

    const totalSteps = hasFixedDir ? 1 : 2

    useEffect(() => {
        CheckYtDlp().then(status => setYtdlpOk(status.available)).catch(() => {})
        GetDepStatus().then(status => setDenoOk(status.jsRuntime.found)).catch(() => {})
    }, [])

    useEffect(() => { setLanguage(lang) }, [lang])
    useEffect(() => {
        const root = document.documentElement
        if (theme === 'dark') root.classList.add('dark'); else root.classList.remove('dark')
        localStorage.setItem('YT-GOto-theme', theme)
    }, [theme])

    const handleSelectCookiesFile = async () => {
        if (backendMode === 'desktop') {
            const file = await SelectCookiesFile(); if (file) setCookiesFile(file)
        } else {
            const input = document.createElement('input'); input.type = 'file'; input.accept = '.txt,.cookies'
            input.onchange = async () => {
                const file = input.files?.[0]; if (!file) return
                try { const result = await UploadCookiesFile(file); setCookiesFile(result.path) }
                catch (err) { console.error('Failed to upload cookies file:', err) }
            }; input.click()
        }
    }

    return (
        <>
        <Dialog open={true} onOpenChange={() => {}}>
            <DialogContent className="max-w-lg rounded-2xl shadow-xl" onPointerDownOutside={e => e.preventDefault()} onEscapeKeyDown={e => e.preventDefault()}>
                <DialogHeader>
                    <DialogTitle className="flex items-center justify-between text-base font-bold tracking-tight">
                        {step === 1 ? t('setup.step1Title') : t('setup.step2Title')}
                        <Badge variant="secondary" className="text-xs ml-2">{step} / {totalSteps}</Badge>
                    </DialogTitle>
                </DialogHeader>

                <div className="space-y-4 py-2">
                    {step === 1 && (
                        <div className="space-y-4">
                            {!hasFixedDir && (
                                <>
                                    <p className="text-sm text-muted-foreground">{t('setup.selectDir')}</p>
                                    <div className="flex gap-2">
                                        <Input type="text" value={outputDir} onChange={e => setOutputDir(e.target.value)}
                                            placeholder={t('setup.selectDirPlaceholder')} readOnly={canBrowseLocalPaths} />
                                        <Button variant="outline" onClick={() => { if (backendMode === 'desktop') SelectFolder().then(dir => { if (dir) setOutputDir(dir) }); else setShowDirBrowser(true) }}>
                                            <FolderOpen className="h-4 w-4 mr-1" />{t('outputDir.browse')}
                                        </Button>
                                    </div>
                                </>
                            )}
                            {hasFixedDir && <p className="text-sm text-muted-foreground">{t('setup.fixedDirHint')}</p>}

                            <div className="space-y-1.5">
                                <Label className="text-xs text-muted-foreground">{t('settings.language')}</Label>
                                <Select value={language} onValueChange={(v: string) => { const l = v as 'zh-CN' | 'en-US'; setLanguage(l); setLang(l) }}>
                                    <SelectTrigger><SelectValue /></SelectTrigger>
                                    <SelectContent>
                                        {LANGUAGE_OPTIONS.map(item => <SelectItem key={item} value={item}>{item === 'zh-CN' ? t('settings.langZh') : t('settings.langEn')}</SelectItem>)}
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-muted-foreground">{t('setup.languageDesc')}</p>
                            </div>

                            <div className="space-y-1.5">
                                <Label className="text-xs text-muted-foreground">{t('settings.theme')}</Label>
                                <Select value={theme} onValueChange={(v: string) => setTheme(v as 'dark' | 'light')}>
                                    <SelectTrigger><SelectValue /></SelectTrigger>
                                    <SelectContent>
                                        {THEME_OPTIONS.map(item => <SelectItem key={item} value={item}>{t(`app.theme.${item}` as any)}</SelectItem>)}
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-muted-foreground">{t('setup.themeDesc')}</p>
                            </div>

                            {ytdlpOk && (
                                <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400 rounded-md border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-950/30 p-2">
                                    <CheckCircle2 className="h-4 w-4" /> {t('setup.ytReady')}
                                </div>
                            )}
                            {denoOk === true && (
                                <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400 rounded-md border border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-950/30 p-2">
                                    <CheckCircle2 className="h-4 w-4" /> {t('setup.denoReady')}
                                </div>
                            )}
                            {denoOk === false && (
                                <div className="rounded-md border border-yellow-300 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-950/30 p-3 space-y-2">
                                    <div className="flex items-center gap-2 text-sm font-medium text-yellow-700 dark:text-yellow-400">
                                        <AlertTriangle className="h-4 w-4" /> {t('setup.denoNotFound')}
                                    </div>
                                    <p className="text-xs text-muted-foreground">{t('setup.denoDesc')}</p>
                                    <div className="space-y-1.5">
                                        <div><span className="text-[10px] text-muted-foreground uppercase">Windows (PowerShell)</span><code className="block text-xs bg-muted p-1.5 rounded mt-0.5">irm https://deno.land/install.ps1 | iex</code></div>
                                        <div><span className="text-[10px] text-muted-foreground uppercase">macOS / Linux</span><code className="block text-xs bg-muted p-1.5 rounded mt-0.5">curl -fsSL https://deno.land/install.sh | sh</code></div>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    {step === 2 && (
                        <div className="space-y-4">
                            <p className="text-sm text-muted-foreground">{t('setup.cookiesDesc')}</p>

                            {backendMode === 'desktop' && (
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.cookiesFrom')}</Label>
                                    <Select value={cookiesFrom || '_none'} onValueChange={(v: string) => { const val = v === '_none' ? '' : v; setCookiesFrom(val); if (val) setCookiesFile('') }}>
                                        <SelectTrigger><SelectValue /></SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="_none">{t('settings.cookiesFromNone')}</SelectItem>
                                            {['chrome', 'firefox', 'edge', 'opera', 'brave', 'vivaldi', 'safari'].map(b => <SelectItem key={b} value={b}>{b.charAt(0).toUpperCase() + b.slice(1)}</SelectItem>)}
                                        </SelectContent>
                                    </Select>
                                    <p className="text-xs text-muted-foreground">{t('settings.cookiesFromHint')}</p>
                                </div>
                            )}

                            {!cookiesFrom && !cookiesFile && <Separator><span className="text-xs text-muted-foreground">{t('setup.or')}</span></Separator>}

                            {!cookiesFrom && (
                                <div className="space-y-1.5">
                                    <Label className="text-xs text-muted-foreground">{t('settings.cookiesFile')}</Label>
                                    <div className="flex gap-2">
                                        <Input type="text" value={cookiesFile} onChange={e => { setCookiesFile(e.target.value); if (e.target.value) setCookiesFrom('') }}
                                            placeholder={t('settings.cookiesFilePlaceholder')} readOnly={canBrowseLocalPaths} />
                                        <Button variant="outline" onClick={handleSelectCookiesFile}>
                                            {backendMode === 'desktop' ? <><FolderOpen className="h-4 w-4 mr-1" />{t('outputDir.browse')}</> : <><Upload className="h-4 w-4 mr-1" />{t('action.upload')}</>}
                                        </Button>
                                    </div>
                                    <p className="text-xs text-muted-foreground">{t('settings.cookiesFileHint')}</p>
                                    <details className="mt-2">
                                        <summary className="text-xs text-primary cursor-pointer hover:underline">{t('setup.howtoTitle')}</summary>
                                        <div className="mt-2 rounded-md border p-3 text-xs text-muted-foreground space-y-1.5">
                                            <p>{t('setup.howtoStep1')}:</p>
                                            <ul className="list-disc pl-4 space-y-0.5">
                                                <li><a href="https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc" target="_blank" rel="noopener" className="text-primary hover:underline">{t('setup.howtoChrome')} <ExternalLink className="h-3 w-3 inline" /></a></li>
                                                <li>{t('setup.howtoFirefox')}</li>
                                            </ul>
                                            <p>{t('setup.howtoStep2')}</p>
                                            <p>{t('setup.howtoStep3')}</p>
                                        </div>
                                    </details>
                                </div>
                            )}

                            <Separator />
                            <div className="space-y-1.5">
                                <Label className="text-xs text-muted-foreground">{t('setup.proxy')}</Label>
                                <Input type="text" value={proxy} onChange={e => setProxy(e.target.value)} placeholder="http://127.0.0.1:7890" />
                                <p className="text-xs text-muted-foreground">{t('settings.proxyHint')}</p>
                            </div>
                        </div>
                    )}
                </div>

                <DialogFooter>
                    {!hasFixedDir && step > 1 && (
                        <Button variant="outline" onClick={() => setStep(step - 1)}>{t('setup.back')}</Button>
                    )}
                    {!hasFixedDir && step < totalSteps ? (
                        <Button onClick={() => setStep(step + 1)} disabled={step === 1 && !outputDir}>{t('setup.next')}</Button>
                    ) : (
                        <Button onClick={() => onComplete(outputDir, cookiesFrom, cookiesFile, proxy, language, theme)} disabled={!hasFixedDir && !outputDir}>{t('setup.done')}</Button>
                    )}
                </DialogFooter>
            </DialogContent>
        </Dialog>
        <DirBrowser open={showDirBrowser} initialPath={outputDir} onSelect={dir => setOutputDir(dir)} onClose={() => setShowDirBrowser(false)} />
        </>
    )
}
