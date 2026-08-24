import {backendMode} from '../../lib/backend'
import {useI18n} from '../../i18n/context'
import {Button} from '@/components/ui/button'

interface Props {
    installing: boolean
    onInstall: () => void
    onRecheck: () => void
}

export default function YtDlpInstallGuide({installing, onInstall, onRecheck}: Props) {
    const {t} = useI18n()
    return (
        <div className="rounded-xl border bg-card/70 backdrop-blur-sm p-5 text-center space-y-3 shadow-sm animate-fade-in-up">
            <div className="text-4xl leading-none">⚠️</div>
            <h3 className="text-base font-semibold">{t('ytdlp.notFound')}</h3>
            <p className="text-xs text-muted-foreground">{t('ytdlp.installGuide')}</p>
            <div className="rounded-md border bg-muted/40 p-2.5 text-left space-y-2">
                <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.windowsWinget')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">winget install yt-dlp</code></div>
                <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.windowsScoop')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">scoop install yt-dlp</code></div>
                <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.macHomebrew')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">brew install yt-dlp</code></div>
                <div><span className="text-[11px] text-muted-foreground font-medium">{t('install.linuxPip')}</span><code className="mt-0.5 block rounded bg-background px-2 py-1 text-[11px] text-primary">pip install yt-dlp</code></div>
            </div>
            <p className="text-[11px] text-muted-foreground">{t('ytdlp.installNote')}</p>
            {backendMode === 'web' && <p className="text-[11px] text-muted-foreground">{t('ytdlp.envHint')}</p>}
            <div className="flex items-center justify-center gap-2">
                <Button size="sm" onClick={onInstall} disabled={installing}>
                    {installing ? t('ytdlp.installing') : t('ytdlp.autoInstall')}
                </Button>
                <Button size="sm" variant="outline" onClick={onRecheck}>{t('ytdlp.recheck')}</Button>
            </div>
        </div>
    )
}
