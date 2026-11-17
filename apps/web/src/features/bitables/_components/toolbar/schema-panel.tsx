import { useId, useRef, useState } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { AlertBanner, Button, Checkbox, Label, PopoverTitle, ResponsivePanel, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { BitableInput as Input } from '../controls'
import type { Choice, FieldOptions, Field, View } from '../../model/types'
import { useBitableFieldChoices } from '../../hooks/useBitableFieldChoices'
import { FieldChoiceOptions } from './field-choice-options'

export type SchemaPanelTarget =
  | { kind: 'create-view' }
  | { kind: 'rename-view'; view: View }
  | { kind: 'create-field' }
  | { kind: 'rename-field'; field: Field }

export function SchemaPanel({ target, onClose, onSave, presentation = 'panel' }: {
  target: SchemaPanelTarget; onClose: () => void; presentation?: 'panel' | 'popover'
  onSave: (name: string, fieldType: Field['type'], viewKind: View['kind'], choices: readonly Choice[], options?: FieldOptions) => Promise<boolean>
}) {
  const nameId = useId()
  const [name, setName] = useState(target.kind === 'rename-view' ? target.view.name : target.kind === 'rename-field' ? target.field.name : '')
  const existing = target.kind === 'rename-field' ? target.field : undefined
  const [fieldType, setFieldType] = useState<Field['type']>(existing?.type ?? 'text')
  const [viewKind, setViewKind] = useState<View['kind']>('grid')
  const choices = useBitableFieldChoices(fieldType === 'multiple_select', existing?.options)
  const isField = target.kind === 'create-field' || target.kind === 'rename-field'
  const isChoice = isField && (fieldType === 'single_select' || fieldType === 'multiple_select')
  const [multiple, setMultiple] = useState(existing?.options?.multiple ?? false)
  const [phase, setPhase] = useState<'editing' | 'saving' | 'error'>('editing')
  const saving = useRef(false)
  const labels = { 'create-view': text.createView, 'rename-view': text.renameView, 'create-field': text.addField, 'rename-field': text.renameField }
  const types = Object.entries(BITABLE_COPY.fieldTypes).map(([value, label]) => ({ value: value as Field['type'], label }))
  const save = async () => {
    if (saving.current || !name.trim() || (isChoice && !choices.valid)) return
    saving.current = true
    setPhase('saving')
    try { if (await onSave(name.trim(), fieldType, viewKind, isChoice ? choices.normalized : [], fieldType === 'user' ? { ...existing?.options, multiple } : isChoice ? { ...existing?.options, ...choices.options, choices: choices.normalized, defaultValue: choices.options.defaultValue } : existing?.options)) { onClose(); return } } catch { /* The command owner owns safe API feedback. */ }
    saving.current = false
    setPhase('error')
  }
  const footer = <div className="flex justify-end gap-2"><Button variant="outline" disabled={phase === 'saving'} onClick={onClose}>{BITABLE_COPY.cancel}</Button><Button disabled={!name.trim() || phase === 'saving' || (isChoice && !choices.valid)} onClick={() => void save()}>{BITABLE_COPY.save}</Button></div>
  const content = <div className="grid gap-3">
      <div className="grid gap-1"><Label htmlFor={nameId}>{BITABLE_COPY.name}</Label><Input id={nameId} value={name} autoFocus maxLength={128} disabled={phase === 'saving'} onChange={(event) => setName(event.currentTarget.value)} /></div>
      {isField ? <>
        <Label>{text.fieldType}</Label><Select value={fieldType} items={types} disabled={phase === 'saving' || Boolean(existing)} onValueChange={(type) => { if (type) setFieldType(type) }}><SelectTrigger aria-label={text.fieldType}><SelectValue /></SelectTrigger><SelectContent>{types.map((type) => <SelectItem key={type.value} value={type.value}>{type.label}</SelectItem>)}</SelectContent></Select>
        {existing ? <p className="text-xs text-muted-foreground">{text.typeChangeUnavailable}</p> : null}
        {isChoice ? <FieldChoiceOptions choices={choices.choices} selected={choices.selected} multiple={fieldType === 'multiple_select'} disabled={phase === 'saving'} onAdd={choices.add} onUpdate={choices.update} onRemove={choices.remove} onMove={choices.move} onSelect={choices.select} onClear={choices.clear} /> : null}
        {fieldType === 'user' ? <label className="flex items-center gap-2 text-sm"><Checkbox checked={multiple} disabled={phase === 'saving'} onCheckedChange={(value) => setMultiple(value === true)} />{text.multiplePeople}</label> : null}
      </> : null}
      {target.kind === 'create-view' ? <Select value={viewKind} items={[{ value: 'grid', label: text.grid }, { value: 'kanban', label: text.kanban }]} disabled={phase === 'saving'} onValueChange={(kind) => { if (kind === 'grid' || kind === 'kanban') setViewKind(kind) }}><SelectTrigger aria-label={text.viewType}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="grid">{text.grid}</SelectItem><SelectItem value="kanban">{text.kanban}</SelectItem></SelectContent></Select> : null}
      {phase === 'error' ? <AlertBanner tone="error">{BITABLE_COPY.saveError}</AlertBanner> : null}
    </div>
  if (presentation === 'popover') return <div className="flex max-h-96 flex-col"><PopoverTitle className="border-b border-border px-4 py-3">{labels[target.kind]}</PopoverTitle><div className="min-h-0 overflow-y-auto p-4">{content}</div><div className="shrink-0 border-t border-border bg-card p-3">{footer}</div></div>
  return <ResponsivePanel open onOpenChange={(open) => { if (!open && !saving.current) onClose() }} title={labels[target.kind]} pending={phase === 'saving'} footer={footer}>{content}</ResponsivePanel>
}
