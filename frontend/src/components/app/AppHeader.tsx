import {YtDlpStatus} from '../../types'
import {useI18n} from '../../i18n/context'
import {Theme} from '../../hooks/useSettingsController'
import {Button} from '@/components/ui/button'
import {Badge} from '@/components/ui/badge'
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip'
import {Moon, RefreshCw, Settings, Sun} from 'lucide-react'

interface Props {
    ytdlp: YtDlpStatus | null
    isUpdatingYtDlp: boolean
    language: 'zh-CN' | 'en-US'
    theme: Theme
    onUpdateYtDlp: () => void
    onToggleLanguage: () => void
    onToggleTheme: () => void
    onOpenSettings: () => void
}

export default function AppHeader({
    ytdlp,
    isUpdatingYtDlp,
    language,
    theme,
    onUpdateYtDlp,
    onToggleLanguage,
    onToggleTheme,
    onOpenSettings,
}: Props) {
    const {t} = useI18n()

    return (
        <header className="sticky top-0 z-20 flex h-12 items-center gap-2 border-b border-primary/15 bg-background/85 backdrop-blur-xl px-4 relative after:absolute after:bottom-0 after:left-0 after:right-0 after:h-px after:bg-gradient-to-r after:from-transparent after:via-primary/25 after:to-transparent">
            <div className="flex items-center gap-2 shrink-0">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-primary to-primary/70 text-primary-foreground shadow-sm glow-primary">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M23.495 6.205a3.007 3.007 0 0 0-2.088-2.088c-1.87-.501-9.396-.501-9.396-.501s-7.507-.01-9.396.501A3.007 3.007 0 0 0 .527 6.205a31.247 31.247 0 0 0-.522 5.805 31.247 31.247 0 0 0 .522 5.783 3.007 3.007 0 0 0 2.088 2.088c1.868.502 9.396.502 9.396.502s7.506 0 9.396-.502a3.007 3.007 0 0 0 2.088-2.088 31.247 31.247 0 0 0 .5-5.783 31.247 31.247 0 0 0-.5-5.805zM9.609 15.601V8.408l6.264 3.602z"/>
                    </svg>
                </div>
                <span className="text-sm font-bold tracking-tight select-none">{t('app.title')}</span>
            </div>
            <div className="flex-1 flex items-center gap-1.5 pl-1">
                {ytdlp && (
                    <Badge variant={ytdlp.available ? 'secondary' : 'destructive'} className="text-[11px] h-5 px-2 font-medium tracking-wide">
                        {ytdlp.available ? t('ytdlp.version', {version: ytdlp.version}) : t('ytdlp.notFound')}
                    </Badge>
                )}
                {ytdlp?.available && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onUpdateYtDlp} disabled={isUpdatingYtDlp} aria-label={t('ytdlp.update')}>
                                <RefreshCw className={`h-3.5 w-3.5 ${isUpdatingYtDlp ? 'animate-spin' : ''}`} />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>{t('ytdlp.update')}</TooltipContent>
                    </Tooltip>
                )}
            </div>
            <div className="flex items-center gap-0.5 shrink-0">
                <Button variant="ghost" size="sm" onClick={onToggleLanguage} className="text-xs h-7 px-2.5 font-medium tracking-wide">
                    {language === 'zh-CN' ? 'EN' : t('lang.zh')}
                </Button>
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onToggleTheme} aria-label={theme === 'dark' ? t('app.theme.light') : t('app.theme.dark')}>
                            {theme === 'dark' ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>{theme === 'dark' ? t('app.theme.light') : t('app.theme.dark')}</TooltipContent>
                </Tooltip>
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onOpenSettings} aria-label={t('settings.title')}>
                            <Settings className="h-3.5 w-3.5" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t('settings.title')}</TooltipContent>
                </Tooltip>
            </div>
        </header>
    )
}
