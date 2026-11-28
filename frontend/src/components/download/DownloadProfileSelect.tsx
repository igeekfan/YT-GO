import {useId} from 'react'
import {useI18n} from '../../i18n/context'
import {DEFAULT_PROFILE_VALUE} from '../../lib/downloadProfiles'
import type {DownloadProfiles} from '../../types'
import {Label} from '@/components/ui/label'
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select'

interface Props {
    profiles: DownloadProfiles
    disabled?: boolean
    onSelect: (id: string) => void
}

export default function DownloadProfileSelect({profiles, disabled, onSelect}: Props) {
    const {t} = useI18n()
    const id = useId()
    return (
        <div className="min-w-0 space-y-1">
            <Label htmlFor={id} className="text-xs text-muted-foreground">{t('profile.active')}</Label>
            <Select value={profiles.selectedId || DEFAULT_PROFILE_VALUE} disabled={disabled}
                onValueChange={value => onSelect(value === DEFAULT_PROFILE_VALUE ? '' : value)}>
                <SelectTrigger id={id}><SelectValue /></SelectTrigger>
                <SelectContent>
                    <SelectItem value={DEFAULT_PROFILE_VALUE}>{t('profile.defaults')}</SelectItem>
                    {profiles.items?.map(profile => (
                        <SelectItem key={profile.id} value={profile.id}>{profile.name}</SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    )
}
