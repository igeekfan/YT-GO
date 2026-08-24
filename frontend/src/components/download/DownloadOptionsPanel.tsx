import {MediaDownloadController} from '../../hooks/useMediaDownload'
import {DownloadOptionKey, SettingsController} from '../../hooks/useSettingsController'
import {getSubtitleSelectionKey} from '../../lib/downloadComposer'
import {useI18n} from '../../i18n/context'
import {Badge} from '@/components/ui/badge'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip'
import {Info, Settings as SettingsIcon} from 'lucide-react'

interface Props {
    media: MediaDownloadController
    settings: SettingsController
}

export default function DownloadOptionsPanel({media, settings}: Props) {
    const {t} = useI18n()
    const {downloadOptions} = settings
    const {
        videoInfo, filenameTemplate, setFilenameTemplate,
        hasAvailableSubtitles, isWechatChannelsVideo,
        subtitleSearch, setSubtitleSearch,
        selectedSubtitleLangs, setSelectedSubtitleLangs,
    } = media
    if (!videoInfo) return null

    const options: Array<{key: DownloadOptionKey; label: string}> = [
        {key: 'saveThumbnail', label: t('downloadOpt.saveThumbnail')},
        {key: 'saveDescription', label: t('downloadOpt.saveDescription')},
        ...(!isWechatChannelsVideo ? [{key: 'embedChapters' as const, label: t('downloadOpt.embedChapters')}] : []),
        ...(hasAvailableSubtitles ? [
            {key: 'writeSubtitles' as const, label: t('downloadOpt.writeSubtitles')},
            ...(downloadOptions.writeSubtitles ? [{key: 'embedSubtitles' as const, label: t('downloadOpt.embedSubtitles')}] : []),
        ] : []),
        ...(!isWechatChannelsVideo ? [{key: 'sponsorBlock' as const, label: t('downloadOpt.sponsorBlock')}] : []),
    ]

    return (
        <div className="rounded-lg border bg-muted/20 p-3.5 space-y-2.5">
            <div className="text-[11px] font-semibold uppercase tracking-widest text-muted-foreground flex items-center gap-1.5">
                <SettingsIcon className="h-3 w-3" /> {t('downloadOpt.title')}
            </div>
            <div className="space-y-1">
                <div className="flex items-center gap-1.5">
                    <Label className="text-[11px] text-muted-foreground">{t('downloadOpt.filenameTemplate')}</Label>
                    <Tooltip>
                        <TooltipTrigger aria-label={t('downloadOpt.filenameTemplateHelp')}><Info className="h-3 w-3 text-muted-foreground" /></TooltipTrigger>
                        <TooltipContent className="max-w-xs text-xs space-y-1">
                            <div>{t('downloadOpt.filenameTemplateHelp')}</div>
                            <div className="font-mono text-[10px] break-all">%(title)s · %(uploader)s · %(upload_date)s · %(id)s · %(ext)s</div>
                        </TooltipContent>
                    </Tooltip>
                </div>
                <Input
                    type="text"
                    value={filenameTemplate}
                    onChange={event => setFilenameTemplate(event.target.value)}
                    placeholder={settings.currentSettings?.filenameTemplate || '%(title)s.%(ext)s'}
                    className="h-7 text-xs"
                />
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-x-3 gap-y-1.5">
                {options.map(option => (
                    <label key={option.key} className="flex items-center gap-1.5 cursor-pointer">
                        <Checkbox checked={downloadOptions[option.key]}
                            onCheckedChange={(checked: boolean) => settings.setDownloadOption(option.key, !!checked)} />
                        <span className="text-xs font-normal">{option.label}</span>
                        {option.key === 'sponsorBlock' && (
                            <Tooltip>
                                <TooltipTrigger aria-label={t('downloadOpt.sponsorBlockDesc')}><Info className="h-3 w-3 text-muted-foreground" /></TooltipTrigger>
                                <TooltipContent className="max-w-xs">{t('downloadOpt.sponsorBlockDesc')}</TooltipContent>
                            </Tooltip>
                        )}
                    </label>
                ))}
            </div>

            {downloadOptions.writeSubtitles && hasAvailableSubtitles && (
                <div className="space-y-1.5">
                    <Label className="text-[11px] text-muted-foreground">{t('downloadOpt.subtitleLangs')}</Label>
                    <Input
                        type="text"
                        value={subtitleSearch}
                        onChange={event => setSubtitleSearch(event.target.value)}
                        placeholder={t('downloadOpt.subtitleSearch')}
                        className="h-7 text-xs"
                    />
                    <div className="max-h-28 overflow-y-auto rounded-md border">
                        {videoInfo.subtitles
                            .filter(subtitle => {
                                if (!subtitleSearch.trim()) return true
                                const query = subtitleSearch.toLowerCase()
                                return (subtitle.name || '').toLowerCase().includes(query) || subtitle.code.toLowerCase().includes(query)
                            })
                            .map(subtitle => (
                                <label key={getSubtitleSelectionKey(subtitle)} className="flex items-center gap-2 px-2.5 py-1 hover:bg-muted/50 cursor-pointer text-xs border-b last:border-b-0">
                                    <Checkbox checked={selectedSubtitleLangs.has(getSubtitleSelectionKey(subtitle))}
                                        onCheckedChange={(checked: boolean) => {
                                            setSelectedSubtitleLangs(current => {
                                                const next = new Set(current)
                                                const key = getSubtitleSelectionKey(subtitle)
                                                if (checked) next.add(key)
                                                else next.delete(key)
                                                return next
                                            })
                                        }} />
                                    <span className="flex-1 truncate">{subtitle.name || subtitle.code}</span>
                                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0 shrink-0">
                                        {subtitle.auto ? t('downloadOpt.subtitleAuto') : t('downloadOpt.subtitleManual')}
                                    </Badge>
                                </label>
                            ))}
                    </div>
                </div>
            )}
        </div>
    )
}
