import {useId} from 'react'
import type {MediaDownloadController} from '../../hooks/useMediaDownload'
import {getSubtitleSelectionKey, isAudioDownload, type DownloadOptionKey} from '../../lib/downloadComposer'
import {DEFAULT_FILENAME_TEMPLATE} from '../../lib/downloadProfiles'
import {useI18n} from '../../i18n/context'
import {backendMode} from '../../lib/backend'
import {OutputFormatSelect, PresetQualitySelect} from './DownloadOptionSelects'
import {Badge} from '@/components/ui/badge'
import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip'
import {FolderOpen, Info} from 'lucide-react'

interface Props {
    media: MediaDownloadController
    hasFixedDir: boolean
    onBrowseDirectory: () => void
}

export default function DownloadOptionsPanel({media, hasFixedDir, onBrowseDirectory}: Props) {
    const {t} = useI18n()
    const id = useId()
    const {
        videoInfo, task, setTaskOption,
        hasAvailableSubtitles, isWechatChannelsVideo,
        subtitleSearch, setSubtitleSearch,
        selectedSubtitleLangs, setSelectedSubtitleLangs,
    } = media
    const audio = isAudioDownload(media.resolveQuality())

    const options: Array<{key: DownloadOptionKey; label: string}> = [
        {key: 'saveThumbnail', label: t('downloadOpt.saveThumbnail')},
        {key: 'saveDescription', label: t('downloadOpt.saveDescription')},
        ...(!isWechatChannelsVideo ? [{key: 'embedChapters' as const, label: t('downloadOpt.embedChapters')}] : []),
        ...(!isWechatChannelsVideo ? [
            {key: 'writeSubtitles' as const, label: t('downloadOpt.writeSubtitles')},
            ...(task.options.writeSubtitles ? [{key: 'embedSubtitles' as const, label: t('downloadOpt.embedSubtitles')}] : []),
        ] : []),
        ...(!isWechatChannelsVideo ? [{key: 'sponsorBlock' as const, label: t('downloadOpt.sponsorBlock')}] : []),
    ]

    return (
        <div className="space-y-3 rounded-lg border bg-muted/20 p-3.5">
            <div className="grid gap-2.5 sm:grid-cols-2">
                {!media.hasCustomFormatSelection && (media.formatMode === 'single' || media.formatMode === 'combine') && (
                    <PresetQualitySelect value={task.quality} onChange={value => setTaskOption('quality', value)} />
                )}
                {!isWechatChannelsVideo && (
                    <OutputFormatSelect audio={audio} value={audio ? task.audioFormat : task.videoContainer}
                        allowAppDefault onChange={value => setTaskOption(audio ? 'audioFormat' : 'videoContainer', value)} />
                )}
            </div>
            {!hasFixedDir && (
                <div className="space-y-1">
                    <Label htmlFor={`${id}-directory`} className="text-xs text-muted-foreground">{t('outputDir.label')}</Label>
                    <div className="flex gap-2">
                        <Input id={`${id}-directory`} value={task.outputDir} spellCheck={false}
                            onChange={event => setTaskOption('outputDir', event.target.value)}
                            placeholder={backendMode === 'web' ? t('outputDir.serverPathPlaceholder') : t('outputDir.placeholder')} />
                        <Button type="button" variant="outline" onClick={backendMode === 'desktop' ? media.selectFolder : onBrowseDirectory}>
                            <FolderOpen className="mr-1 h-4 w-4" />{t('outputDir.browse')}
                        </Button>
                    </div>
                </div>
            )}
            <div className="space-y-1">
                <div className="flex items-center gap-1.5">
                    <Label htmlFor={`${id}-filename`} className="text-xs text-muted-foreground">{t('downloadOpt.filenameTemplate')}</Label>
                    <Tooltip>
                        <TooltipTrigger aria-label={t('downloadOpt.filenameTemplateHelp')}><Info className="h-3 w-3 text-muted-foreground" /></TooltipTrigger>
                        <TooltipContent className="max-w-xs text-xs space-y-1">
                            <div>{t('downloadOpt.filenameTemplateHelp')}</div>
                            <div className="font-mono text-[10px] break-all">%(title)s · %(uploader)s · %(upload_date)s · %(id)s · %(ext)s</div>
                        </TooltipContent>
                    </Tooltip>
                </div>
                <Input
                    id={`${id}-filename`}
                    value={task.filenameTemplate}
                    onChange={event => setTaskOption('filenameTemplate', event.target.value)}
                    placeholder={DEFAULT_FILENAME_TEMPLATE}
                    spellCheck={false}
                />
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-x-3 gap-y-1.5">
                {options.map(option => (
                    <label key={option.key} className="flex items-center gap-1.5 cursor-pointer">
                        <Checkbox checked={task.options[option.key]}
                            onCheckedChange={checked => media.setDownloadOption(option.key, checked === true)} />
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

            {task.options.writeSubtitles && !isWechatChannelsVideo && (
                <div className="space-y-1">
                    <Label htmlFor={`${id}-languages`} className="text-xs text-muted-foreground">{t('settings.subtitleLangs')}</Label>
                    <Input id={`${id}-languages`} value={task.subtitleLangs} spellCheck={false}
                        placeholder={t('settings.subtitleLangsPlaceholder')}
                        onChange={event => {
                            setSelectedSubtitleLangs(new Set())
                            setTaskOption('subtitleLangs', event.target.value)
                        }} />
                </div>
            )}

            {task.options.writeSubtitles && hasAvailableSubtitles && videoInfo && (
                <div className="space-y-1.5">
                    <Label htmlFor={`${id}-subtitle-search`} className="text-xs text-muted-foreground">{t('downloadOpt.subtitleSearch')}</Label>
                    <Input
                        id={`${id}-subtitle-search`}
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
            {media.hasTaskOverrides && (
                <Button type="button" variant="ghost" size="sm" onClick={media.resetTaskOptions}>
                    {t('download.resetTaskOptions')}
                </Button>
            )}
        </div>
    )
}
