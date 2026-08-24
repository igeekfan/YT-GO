import {MediaDownloadController} from '../../hooks/useMediaDownload'
import {useI18n} from '../../i18n/context'
import {formatDuration} from '../../lib/formatUtils'
import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'

export default function MediaSummary({media}: {media: MediaDownloadController}) {
    const {t} = useI18n()
    const {videoInfo, playlistInfo, collectionKind, selectedPlaylistItems, setSelectedPlaylistItems} = media

    return (
        <div className="space-y-2.5 animate-fade-in-up delay-1">
            {videoInfo && (
                <div className="flex gap-3.5 rounded-xl border bg-card/70 backdrop-blur-sm p-3.5 shadow-sm">
                    {videoInfo.thumbnail && (
                        <img src={videoInfo.thumbnail} alt={videoInfo.title} className="w-36 h-[81px] object-cover rounded-lg shrink-0 shadow-sm" onError={event => { (event.target as HTMLImageElement).style.display = 'none' }} />
                    )}
                    <div className="flex-1 min-w-0 space-y-1">
                        <div className="font-semibold text-sm line-clamp-2 leading-snug">{videoInfo.title}</div>
                        <div className="flex gap-x-3 gap-y-0.5 text-xs text-muted-foreground flex-wrap">
                            {videoInfo.duration > 0 && <span>{t('video.duration')}: {formatDuration(videoInfo.duration)}</span>}
                            {videoInfo.uploader && <span>{t('video.uploader')}: {videoInfo.uploader}</span>}
                            {videoInfo.platform && <span>{t('video.platform')}: {videoInfo.platform}</span>}
                        </div>
                    </div>
                </div>
            )}

            {playlistInfo && (
                <>
                    <div className="flex gap-3.5 rounded-xl border bg-card/70 backdrop-blur-sm p-3.5 shadow-sm">
                        <div className="w-12 h-12 flex items-center justify-center rounded-lg bg-gradient-to-br from-primary/15 to-primary/5 text-primary shrink-0">
                            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
                        </div>
                        <div className="flex-1 min-w-0 space-y-1">
                            <div className="font-semibold text-sm leading-snug">
                                {t(`collection.${collectionKind}.detected` as any)}{playlistInfo.title ? `: ${playlistInfo.title}` : ''}
                            </div>
                            <div className="flex gap-x-3 gap-y-0.5 text-xs text-muted-foreground flex-wrap">
                                <span>{t(`collection.${collectionKind}.count` as any, {count: String(playlistInfo.count)})}</span>
                                {playlistInfo.uploader && <span>{t('playlist.uploader')}: {playlistInfo.uploader}</span>}
                                <span>{t('collection.selected', {count: String(selectedPlaylistItems.size)})}</span>
                            </div>
                        </div>
                    </div>
                    <div className="rounded-xl border overflow-hidden shadow-sm">
                        <div className="flex gap-1.5 px-2.5 py-1.5 border-b bg-muted/40">
                            <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setSelectedPlaylistItems(new Set(playlistInfo.videos.map((_, index) => index)))}>
                                {t('collection.selectAll')}
                            </Button>
                            <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setSelectedPlaylistItems(new Set())}>
                                {t('collection.selectNone')}
                            </Button>
                        </div>
                        <div className="max-h-60 overflow-y-auto">
                            {playlistInfo.videos.map((video, index) => (
                                <label key={index} className="flex items-center gap-2.5 px-2.5 py-1.5 hover:bg-muted/50 cursor-pointer text-sm border-b last:border-b-0">
                                    <Checkbox
                                        checked={selectedPlaylistItems.has(index)}
                                        onCheckedChange={(checked: boolean) => {
                                            setSelectedPlaylistItems(current => {
                                                const next = new Set(current)
                                                if (checked) next.add(index)
                                                else next.delete(index)
                                                return next
                                            })
                                        }}
                                    />
                                    <span className="text-xs text-muted-foreground w-6 text-right shrink-0">{index + 1}</span>
                                    <span className="flex-1 min-w-0 truncate text-xs">{video.title || video.url || video.id}</span>
                                    {video.duration > 0 && <span className="text-[11px] text-muted-foreground shrink-0">{formatDuration(video.duration)}</span>}
                                </label>
                            ))}
                        </div>
                    </div>
                </>
            )}
        </div>
    )
}
