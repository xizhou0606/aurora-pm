import { useState } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { AlertBanner, Button, Popover, PopoverContent, PopoverTitle, PopoverTrigger, Select, SelectItem, SelectValue } from '@/shared/ui'
import { BitableSelectTrigger as SelectTrigger, BitableSelectContent as SelectContent } from '../controls'
import { PhCheck } from '@/shared/ui/icons'
import type { Field, View, ViewConfig } from '../../model/types'
import { FilterSortEditor } from './filter-sort-editor'
import { useViewSettingDraft } from '../../hooks/useViewSettingDraft'

const settings = [
  { key: 'rowHeight', label: text.rowHeight },
  { key: 'groupFieldId', label: text.group },
  { key: 'filters', label: text.filter },
  { key: 'sorts', label: text.sort },
] as const
const rowHeights = [
  { value: 36, label: text.rowHeightPresets.low },
  { value: 56, label: text.rowHeightPresets.medium },
  { value: 88, label: text.rowHeightPresets.high },
  { value: 120, label: text.rowHeightPresets.extraHigh },
] as const
type Setting = typeof settings[number]
type Props = { fields: readonly Field[]; view: View; readOnly: boolean; pending: boolean; onSave: (id: string, config: ViewConfig) => Promise<boolean> }

export function ViewSettingPopovers(props: Props) {
  return settings.map(setting => <SettingPopover key={`${props.view.id}:${setting.key}`} {...props} setting={setting} />)
}

function SettingPopover({ setting, ...props }: Props & { setting: Setting }) {
  const [open, setOpen] = useState(false)
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger render={<Button variant="ghost" size="sm" />} disabled={props.pending} className="h-7 px-2 text-xs text-muted-foreground">{setting.label}</PopoverTrigger>
    <PopoverContent className={`${setting.key === 'rowHeight' ? 'w-36' : setting.key === 'filters' ? 'w-[32rem]' : 'w-96'} max-w-[calc(100vw-2rem)] p-3`}>
      <PopoverTitle className="mb-3 text-sm">{setting.label}</PopoverTitle>
      {open ? <SettingEditor {...props} setting={setting} onClose={() => setOpen(false)} /> : null}
    </PopoverContent>
  </Popover>
}

function SettingEditor({ fields, view, readOnly, pending, onSave, setting, onClose }: Props & { setting: Setting; onClose: () => void }) {
  const { draft, setDraft, phase, disabled, save } = useViewSettingDraft({ view, setting: setting.key, pending, readOnly, onSave, onClose })
  return <div className="grid gap-3">
    {setting.key === 'rowHeight' ? <div className="grid gap-0.5">{rowHeights.map(preset => <Button key={preset.value} variant="ghost" size="sm" aria-pressed={draft.rowHeight === preset.value} disabled={disabled} className={`h-8 w-full justify-start gap-2 px-2 text-xs ${draft.rowHeight === preset.value ? 'text-primary' : ''}`} onClick={() => { const next = { ...draft, rowHeight: preset.value }; setDraft(next); void save(next) }}><span aria-hidden="true" className="flex size-4 items-center justify-center"><span className="h-(--preset-h) w-2 rounded-sm border border-current" style={{ '--preset-h': `${4 + preset.value / 12}px` }} /></span>{preset.label}{draft.rowHeight === preset.value ? <PhCheck className="ml-auto size-3.5" aria-hidden="true" /> : null}</Button>)}</div> : null}
    {setting.key === 'groupFieldId' ? <Select value={draft.groupFieldId ?? ''} disabled={disabled} items={[{ value: '', label: text.noGroup }, ...fields.map(field => ({ value: field.id, label: field.name }))]} onValueChange={id => setDraft({ ...draft, groupFieldId: id || undefined })}><SelectTrigger aria-label={text.group}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="">{text.noGroup}</SelectItem>{fields.map(field => <SelectItem key={field.id} value={field.id}>{field.name}</SelectItem>)}</SelectContent></Select> : null}
    {setting.key === 'filters' || setting.key === 'sorts' ? <div className="max-h-72 overflow-y-auto"><FilterSortEditor fields={fields} config={draft} disabled={disabled} onChange={setDraft} section={setting.key} /></div> : null}
    {phase === 'error' ? <AlertBanner tone="error">{BITABLE_COPY.saveError}</AlertBanner> : null}
    {!readOnly && setting.key !== 'rowHeight' ? <div className="flex justify-end"><Button variant="ghost" size="xs" className="text-primary" disabled={disabled} onClick={() => void save()}>{text.apply}</Button></div> : null}
  </div>
}
