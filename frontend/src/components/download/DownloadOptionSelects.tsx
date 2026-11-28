import {useId} from 'react'
import {useI18n} from '../../i18n/context'
import {AUDIO_FORMATS, DOWNLOAD_QUALITIES, VIDEO_CONTAINERS} from '../../lib/downloadProfiles'
import {Label} from '@/components/ui/label'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'

export function PresetQualitySelect({value, onChange}: {value: string; onChange: (quality: string) => void}) {
    const {t} = useI18n()
    const id = useId()
    return (
        <div className="space-y-1">
            <Label htmlFor={id} className="text-xs text-muted-foreground">{t('quality.label')}</Label>
            <Select name="quality" value={value} onValueChange={onChange}>
                <SelectTrigger id={id}><SelectValue /></SelectTrigger>
                <SelectContent>
                    {DOWNLOAD_QUALITIES.map(quality => (
                        <SelectItem key={quality} value={quality}>{t(quality === 'audio' ? 'format.mode.audioOnly' : `quality.${quality}`)}</SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    )
}

interface OutputFormatProps {
    audio: boolean
    value?: string
    allowAppDefault?: boolean
    onChange: (format: string | undefined) => void
}

export function OutputFormatSelect({audio, value, allowAppDefault, onChange}: OutputFormatProps) {
    const {t} = useI18n()
    const id = useId()
    const autoValue = audio ? 'default' : 'auto'
    const formats = audio ? AUDIO_FORMATS : VIDEO_CONTAINERS
    return (
        <div className="space-y-1">
            <Label htmlFor={id} className="text-xs text-muted-foreground">{t(audio ? 'settings.audioFormat' : 'settings.mergeOutputFormat')}</Label>
            <Select name={audio ? 'audioFormat' : 'videoContainer'}
                value={value || (allowAppDefault ? 'inherit' : autoValue)}
                onValueChange={next => onChange(next === 'inherit' ? undefined : next)}>
                <SelectTrigger id={id}><SelectValue /></SelectTrigger>
                <SelectContent>
                    {allowAppDefault && <SelectItem value="inherit">{t('profile.defaults')}</SelectItem>}
                    <SelectItem value={autoValue}>{t(audio ? 'settings.audioFormatDefault' : 'settings.mergeOutputFormatAuto')}</SelectItem>
                    {formats.map(([format, label]) => <SelectItem key={format} value={format}>{label}</SelectItem>)}
                </SelectContent>
            </Select>
        </div>
    )
}
