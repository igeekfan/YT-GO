import {MediaDownloadController} from '../../hooks/useMediaDownload'
import {FormatMode, parseResolutionHeight, sortFormats} from '../../lib/downloadComposer'
import {useI18n} from '../../i18n/context'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'
import {Collapsible, CollapsibleContent, CollapsibleTrigger} from '@/components/ui/collapsible'
import {ChevronDown, ChevronRight} from 'lucide-react'

export default function FormatSelector({media}: {media: MediaDownloadController}) {
    const {t} = useI18n()
    const {
        formatInfo, formatMode, setFormatMode,
        selectedFormat, setSelectedFormat,
        selectedVideoFormat, setSelectedVideoFormat,
        selectedAudioFormat, setSelectedAudioFormat,
        isGettingFormats, formatExpanded, setFormatExpanded,
        videoOnlyFormats, audioOnlyFormats, combineVideoFormats, combineAudioFormats,
        hasSeparateTrackFormats, hasCustomFormatSelection,
        getFormatOptionLabel, selectBestQuality, getFormats,
    } = media

    return (
        <Collapsible open={formatExpanded} onOpenChange={setFormatExpanded}>
            <CollapsibleTrigger className="flex items-center gap-1.5 w-full rounded-lg border px-3 py-2 text-sm font-medium hover:bg-muted/40 transition-all duration-150">
                {formatExpanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                <span>{t('format.label')}</span>
                {!formatExpanded && hasCustomFormatSelection && (
                    <span className="text-xs text-primary truncate ml-1.5">
                        {selectedFormat && formatInfo
                            ? getFormatOptionLabel(formatInfo.formats.find(format => format.formatId === selectedFormat)!)
                            : (selectedVideoFormat || selectedAudioFormat)
                                ? `${selectedVideoFormat ? t('format.video') : ''}${selectedVideoFormat && selectedAudioFormat ? ' + ' : ''}${selectedAudioFormat ? t('format.audio') : ''}`
                                : ''}
                    </span>
                )}
            </CollapsibleTrigger>
            <CollapsibleContent className="mt-2 space-y-2.5">
                {formatInfo ? (
                    <div className="space-y-3">
                        <div className="flex items-center gap-2">
                            <Select value={formatMode} onValueChange={(value: string) => {
                                setFormatMode(value as FormatMode)
                                setSelectedFormat(''); setSelectedVideoFormat(''); setSelectedAudioFormat('')
                            }}>
                                <SelectTrigger className="w-48"><SelectValue /></SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="single">{t('format.mode.single')}</SelectItem>
                                    <SelectItem value="combine">{t('format.mode.combine')}</SelectItem>
                                    <SelectItem value="audio-only">{t('format.mode.audioOnly')}</SelectItem>
                                    <SelectItem value="video-only">{t('format.mode.videoOnly')}</SelectItem>
                                </SelectContent>
                            </Select>
                            <Button variant="outline" size="sm" onClick={selectBestQuality}>{t('format.bestQuality')}</Button>
                        </div>

                        {formatMode === 'single' && (
                            <div className="max-h-52 overflow-y-auto rounded-lg border">
                                {sortFormats(formatInfo.formats.filter(format => format.hasVideo || format.hasAudio)).map(format => (
                                    <label key={format.formatId} className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedFormat === format.formatId ? 'bg-primary/5' : ''}`}>
                                        <input type="radio" name="format-single" checked={selectedFormat === format.formatId}
                                            onChange={() => { setSelectedFormat(format.formatId); setSelectedVideoFormat(''); setSelectedAudioFormat('') }}
                                            className="mt-0.5 accent-primary" />
                                        <span className="flex-1 break-words leading-relaxed">{getFormatOptionLabel(format)}</span>
                                    </label>
                                ))}
                            </div>
                        )}

                        {formatMode === 'audio-only' && (
                            <div className="max-h-52 overflow-y-auto rounded-md border">
                                <label className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedAudioFormat ? 'bg-primary/5' : ''}`}>
                                    <input type="radio" name="format-audio-only" checked={!selectedAudioFormat} onChange={() => setSelectedAudioFormat('')} className="mt-0.5 accent-primary" />
                                    <span>{t('format.auto')}</span>
                                </label>
                                {audioOnlyFormats.sort((a, b) => (b.tbr || b.filesize || 0) - (a.tbr || a.filesize || 0)).map(format => (
                                    <label key={format.formatId} className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedAudioFormat === format.formatId ? 'bg-primary/5' : ''}`}>
                                        <input type="radio" name="format-audio-only" checked={selectedAudioFormat === format.formatId}
                                            onChange={() => { setSelectedAudioFormat(format.formatId); setSelectedVideoFormat(''); setSelectedFormat('') }}
                                            className="mt-0.5 accent-primary" />
                                        <span className="flex-1 break-words leading-relaxed">{getFormatOptionLabel(format)}</span>
                                    </label>
                                ))}
                            </div>
                        )}

                        {formatMode === 'video-only' && (
                            <div className="max-h-52 overflow-y-auto rounded-md border">
                                <label className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedVideoFormat ? 'bg-primary/5' : ''}`}>
                                    <input type="radio" name="format-video-only" checked={!selectedVideoFormat} onChange={() => setSelectedVideoFormat('')} className="mt-0.5 accent-primary" />
                                    <span>{t('format.auto')}</span>
                                </label>
                                {videoOnlyFormats.sort((a, b) => {
                                    const heightDifference = parseResolutionHeight(b.resolution || '') - parseResolutionHeight(a.resolution || '')
                                    return heightDifference || (b.filesize || 0) - (a.filesize || 0)
                                }).map(format => (
                                    <label key={format.formatId} className={`flex items-start gap-2 px-2.5 py-2 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedVideoFormat === format.formatId ? 'bg-primary/5' : ''}`}>
                                        <input type="radio" name="format-video-only" checked={selectedVideoFormat === format.formatId}
                                            onChange={() => { setSelectedVideoFormat(format.formatId); setSelectedAudioFormat(''); setSelectedFormat('') }}
                                            className="mt-0.5 accent-primary" />
                                        <span className="flex-1 break-words leading-relaxed">{getFormatOptionLabel(format)}</span>
                                    </label>
                                ))}
                            </div>
                        )}

                        {formatMode === 'combine' && hasSeparateTrackFormats ? (
                            <div className="grid grid-cols-2 gap-3">
                                <div className="space-y-1.5">
                                    <Label className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium">{t('format.video')}</Label>
                                    <div className="max-h-52 overflow-y-auto rounded-md border">
                                        <label className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedVideoFormat ? 'bg-primary/5' : ''}`}>
                                            <input type="radio" name="format-video" checked={!selectedVideoFormat} onChange={() => setSelectedVideoFormat('')} className="mt-0.5 accent-primary" />
                                            <span>{t('format.selectVideo')}</span>
                                        </label>
                                        {combineVideoFormats.map(format => (
                                            <label key={format.formatId} className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedVideoFormat === format.formatId ? 'bg-primary/5' : ''}`}>
                                                <input type="radio" name="format-video" checked={selectedVideoFormat === format.formatId} onChange={() => setSelectedVideoFormat(format.formatId)} className="mt-0.5 accent-primary" />
                                                <span className="flex-1 break-words leading-relaxed">{getFormatOptionLabel(format)}</span>
                                            </label>
                                        ))}
                                    </div>
                                </div>
                                <div className="space-y-1.5">
                                    <Label className="text-[11px] text-muted-foreground uppercase tracking-wider font-medium">{t('format.audio')}</Label>
                                    <div className="max-h-52 overflow-y-auto rounded-md border">
                                        <label className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b ${!selectedAudioFormat ? 'bg-primary/5' : ''}`}>
                                            <input type="radio" name="format-audio" checked={!selectedAudioFormat} onChange={() => setSelectedAudioFormat('')} className="mt-0.5 accent-primary" />
                                            <span>{t('format.selectAudio')}</span>
                                        </label>
                                        {combineAudioFormats.map(format => (
                                            <label key={format.formatId} className={`flex items-start gap-2 px-2.5 py-1.5 text-xs cursor-pointer hover:bg-muted/50 border-b last:border-b-0 ${selectedAudioFormat === format.formatId ? 'bg-primary/5' : ''}`}>
                                                <input type="radio" name="format-audio" checked={selectedAudioFormat === format.formatId} onChange={() => setSelectedAudioFormat(format.formatId)} className="mt-0.5 accent-primary" />
                                                <span className="flex-1 break-words leading-relaxed">{getFormatOptionLabel(format)}</span>
                                            </label>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        ) : formatMode === 'combine' && !hasSeparateTrackFormats ? (
                            <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">{t('format.combineUnavailable')}</div>
                        ) : null}
                    </div>
                ) : (
                    <div className="flex items-center gap-2">
                        {isGettingFormats ? (
                            <span className="text-sm text-muted-foreground">{t('format.loading')}</span>
                        ) : (
                            <Button variant="outline" size="sm" onClick={getFormats} disabled={isGettingFormats}>
                                {isGettingFormats ? t('format.loading') : t('format.detect')}
                            </Button>
                        )}
                    </div>
                )}
            </CollapsibleContent>
        </Collapsible>
    )
}
