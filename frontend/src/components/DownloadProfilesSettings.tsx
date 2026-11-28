import {useId, useState, type FormEvent} from 'react'
import {useI18n} from '../i18n/context'
import {createDownloadProfile, DEFAULT_FILENAME_TEMPLATE} from '../lib/downloadProfiles'
import type {DownloadProfile, DownloadProfiles, Settings} from '../types'
import DownloadProfileSelect from './download/DownloadProfileSelect'
import {OutputFormatSelect, PresetQualitySelect} from './download/DownloadOptionSelects'
import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'

interface Props {
    profiles: DownloadProfiles
    settings: Settings
    hasFixedDir: boolean
    saving: boolean
    onSave: (profiles: DownloadProfiles) => Promise<void>
}

export default function DownloadProfilesSettings({profiles, settings, hasFixedDir, saving, onSave}: Props) {
    const {t} = useI18n()
    const id = useId()
    const [draft, setDraft] = useState<DownloadProfile | null>(null)
    const [error, setError] = useState('')
    const items = profiles.items ?? []
    const selected = items.find(profile => profile.id === profiles.selectedId)
    const editing = items.some(profile => profile.id === draft?.id)

    const persist = async (next: DownloadProfiles) => {
        setError('')
        try {
            await onSave(next)
            setDraft(null)
        } catch (error) {
            const detail = error instanceof Error ? error.message : String(error)
            setError(detail || t('settings.saveFailed'))
        }
    }

    const submit = (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        if (!draft || saving) return
        const profile = {
            ...draft,
            name: draft.name.trim(),
            outputDir: draft.outputDir.trim(),
            filenameTemplate: draft.filenameTemplate.trim(),
            subtitleLangs: draft.subtitleLangs.trim(),
        }
        if (!profile.name) {
            setError(t('profile.nameRequired'))
            return
        }
        void persist({
            items: editing ? items.map(item => item.id === profile.id ? profile : item) : [...items, profile],
            selectedId: editing ? profiles.selectedId : profile.id,
        })
    }

    const update = (patch: Partial<DownloadProfile>) => {
        setDraft(current => current && {...current, ...patch})
        setError('')
    }

    return (
        <div className="space-y-3 border-b border-border px-4 py-4 sm:px-5">
            <h2 className="text-sm font-semibold">{t('profile.title')}</h2>
            <div className="grid items-end gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                <DownloadProfileSelect profiles={profiles} disabled={saving || !!draft}
                    onSelect={selectedId => void persist({...profiles, selectedId})} />
                <div className="flex flex-wrap gap-2">
                    <Button type="button" variant="outline" disabled={saving || !!draft} onClick={() => {
                        setDraft(createDownloadProfile(settings))
                        setError('')
                    }}>{t('profile.new')}</Button>
                    {selected && <>
                        <Button type="button" variant="outline" disabled={saving || !!draft} onClick={() => {
                            setDraft(selected)
                            setError('')
                        }}>{t('profile.edit')}</Button>
                        <Button type="button" variant="ghost" disabled={saving || !!draft}
                            onClick={() => void persist({items: items.filter(profile => profile.id !== selected.id), selectedId: ''})}>
                            {t('profile.delete')}
                        </Button>
                    </>}
                </div>
            </div>
            {draft && (
                <form aria-labelledby={`${id}-heading`} onSubmit={submit} className="space-y-3 rounded-lg border bg-muted/20 p-4">
                    <h3 id={`${id}-heading`} className="text-sm font-semibold">{t(editing ? 'profile.edit' : 'profile.new')}</h3>
                    <div className="space-y-1">
                        <Label htmlFor={`${id}-name`} className="text-xs text-muted-foreground">{t('profile.name')}</Label>
                        <Input id={`${id}-name`} name="name" autoFocus required maxLength={80} value={draft.name}
                            onChange={event => update({name: event.target.value})} />
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                        <PresetQualitySelect value={draft.quality} onChange={quality => update({quality})} />
                        <OutputFormatSelect audio={draft.quality === 'audio'}
                            value={draft.quality === 'audio' ? draft.audioFormat : draft.videoContainer}
                            onChange={format => update(draft.quality === 'audio' ? {audioFormat: format ?? 'default'} : {videoContainer: format ?? 'auto'})} />
                    </div>
                    {!hasFixedDir && (
                        <div className="space-y-1">
                            <Label htmlFor={`${id}-directory`} className="text-xs text-muted-foreground">{t('outputDir.label')}</Label>
                            <Input id={`${id}-directory`} name="outputDir" value={draft.outputDir}
                                placeholder={settings.outputDir} spellCheck={false}
                                onChange={event => update({outputDir: event.target.value})} />
                            <p className="text-xs text-muted-foreground">{t('profile.directoryHint')}</p>
                        </div>
                    )}
                    <div className="space-y-1">
                        <Label htmlFor={`${id}-filename`} className="text-xs text-muted-foreground">{t('settings.filenameTemplate')}</Label>
                        <Input id={`${id}-filename`} name="filenameTemplate" value={draft.filenameTemplate}
                            placeholder={DEFAULT_FILENAME_TEMPLATE} maxLength={512} spellCheck={false}
                            onChange={event => update({filenameTemplate: event.target.value})} />
                    </div>
                    <label className="flex w-fit min-h-8 items-center gap-2 text-sm">
                        <Checkbox name="writeSubtitles" checked={draft.writeSubtitles}
                            onCheckedChange={checked => update({writeSubtitles: checked === true})} />
                        {t('settings.writeSubtitles')}
                    </label>
                    {draft.writeSubtitles && (
                        <>
                            <div className="space-y-1">
                                <Label htmlFor={`${id}-subtitles`} className="text-xs text-muted-foreground">{t('settings.subtitleLangs')}</Label>
                                <Input id={`${id}-subtitles`} name="subtitleLangs" value={draft.subtitleLangs}
                                    placeholder={t('settings.subtitleLangsPlaceholder')} maxLength={512} spellCheck={false}
                                    onChange={event => update({subtitleLangs: event.target.value})} />
                            </div>
                            <label className="flex w-fit min-h-8 items-center gap-2 text-sm">
                                <Checkbox name="embedSubtitles" checked={draft.embedSubtitles}
                                    onCheckedChange={checked => update({embedSubtitles: checked === true})} />
                                {t('settings.embedSubtitles')}
                            </label>
                        </>
                    )}
                    <div className="flex gap-2">
                        <Button type="submit" disabled={saving}>{t('settings.save')}</Button>
                        <Button type="button" variant="outline" disabled={saving}
                            onClick={() => { setDraft(null); setError('') }}>{t('action.cancel')}</Button>
                    </div>
                </form>
            )}
            {error && <p role="alert" className="break-words rounded-md border border-destructive/40 px-3 py-2 text-sm">{error}</p>}
        </div>
    )
}
