import { useEffect, useRef, useState } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { copy } from '@/shared/copy'
import { normalizeTableColumnPreference } from '@/shared/model/table-column-preferences'
import { createDragPreview, type DragPreview } from '@/shared/lib/drag-preview'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { AlertBanner, Button, EmptyState, Popover, PopoverContent, PopoverTitle, PopoverTrigger, SearchInput } from '@/shared/ui'
import { PhDotsSixVertical, PhDotsThree, PhEye, PhEyeSlash, PhPlus, PhSlidersHorizontal } from '@/shared/ui/icons'
import { fieldIcons } from '../../model/field-icons'
import type { ViewToolbarProps } from '../../model/grid-port'
import type { Field, View, ViewConfig } from '../../model/types'
import { SchemaPanel } from './schema-panel'

type Props = Pick<ViewToolbarProps, 'fields' | 'capabilities' | 'pending' | 'createFieldRequest' | 'onCreateField' | 'onRenameField' | 'onUpdateField' | 'onViewConfigChange'> & { view?: View }
export function FieldConfigPopover(props: Props) {
  const [open, setOpen] = useState(false), [query, setQuery] = useState('')
  const [target, setTarget] = useState<string | null>(null)
  const [dragging, setDragging] = useState<string | null>(null)
  const preview = useRef<DragPreview | null>(null)
  useEffect(() => () => { preview.current?.finish() }, [])
  const [busy, setBusy] = useState(false), [error, setError] = useState('')
  const lock = useRef(false), request = useRef(props.createFieldRequest ?? 0)
  const disabled = props.pending || busy
  const editable = props.capabilities.canManageFields
  useEffect(() => {
    const next = props.createFieldRequest ?? 0
    if (next !== request.current && next > 0 && editable && !props.pending) { setOpen(true); setTarget('create'); setQuery('') }
    request.current = next
  }, [props.createFieldRequest, editable, props.pending])
  useEffect(() => { if (!editable) setTarget(null) }, [editable])
  const preference = normalizeTableColumnPreference(props.fields.map(field => ({ id: field.id, label: field.name })), { order: [...(props.view?.config.columnOrder ?? [])], hidden: [...(props.view?.config.hiddenFieldIds ?? [])] })
  const ordered = preference.order.flatMap(id => props.fields.filter(field => field.id === id))
  const matches = ordered.filter(field => field.name.toLocaleLowerCase().includes(query.toLocaleLowerCase()))
  const hidden = props.view?.config.hiddenFieldIds ?? []
  const visibleCount = props.fields.filter(field => !hidden.includes(field.id)).length
  const updateView = async (patch: Partial<ViewConfig>) => {
    if (disabled || lock.current || !props.capabilities.canManageViews || !props.view) return
    lock.current = true; setBusy(true); setError('')
    try {
      if (!await props.onViewConfigChange(props.view.id, { ...props.view.config, ...patch })) setError(BITABLE_COPY.saveError)
    } catch { setError(BITABLE_COPY.saveError) }
    finally { lock.current = false; setBusy(false) }
  }
  const toggle = (field: Field) => {
    if (!hidden.includes(field.id) && visibleCount <= 1) return
    return updateView({ hiddenFieldIds: hidden.includes(field.id) ? hidden.filter(id => id !== field.id) : [...hidden, field.id] })
  }
  const reorder = (id: string, targetId: string) => {
    if (id === targetId || !preference.order.includes(id) || !preference.order.includes(targetId)) return
    const next = preference.order.filter(entry => entry !== id)
    next.splice(preference.order.indexOf(targetId), 0, id)
    void updateView({ columnOrder: next })
  }
  const content = (field?: Field) => <PopoverContent side="right" sideOffset={8} className="w-80 p-0"><SchemaPanel key={field?.id ?? 'create'} presentation="popover" target={field ? { kind: 'rename-field', field } : { kind: 'create-field' }} onClose={() => setTarget(null)} onSave={(name, type, _, choices, options) => field ? props.onUpdateField?.(field.id, name, options) ?? props.onRenameField?.(field.id, name) ?? Promise.resolve(false) : props.onCreateField?.(name, type, choices, options) ?? Promise.resolve(false)} /></PopoverContent>
  return <Popover open={open} onOpenChange={next => { if (disabled) return; setOpen(next); if (!next) { setTarget(null); setQuery(''); setError('') } }}>
    <PopoverTrigger render={<Button variant="ghost" size="sm" />} disabled={props.pending} className="h-7 gap-1.5 px-2 text-xs text-muted-foreground"><PhSlidersHorizontal className="size-3.5" aria-hidden="true" />{text.fieldsConfig}</PopoverTrigger>
    <PopoverContent className="w-64 p-0">
      <PopoverTitle className="sr-only">{text.fieldsConfig}</PopoverTitle>
      <div className="border-b border-border p-2"><SearchInput aria-label={text.fieldSearch} placeholder={text.fieldSearch} value={query} onChange={event => setQuery(event.currentTarget.value)} /></div>
      <div className="max-h-72 overflow-y-auto p-1"
        onDragOver={event => { if (dragging && !disabled) { event.preventDefault(); preview.current?.update(event.clientX, event.clientY) } }}
        onDrop={event => {
          event.preventDefault()
          const hit = preview.current?.update(event.clientX, event.clientY)
          const anchor = hit?.element.dataset.dragField
          if (dragging && anchor && anchor !== dragging && !disabled) {
            const next = preference.order.filter(id => id !== dragging)
            const index = next.indexOf(anchor)
            if (index >= 0) {
              next.splice(index + (hit.direction === 'after' ? 1 : 0), 0, dragging)
              if (next.some((id, i) => id !== preference.order[i])) void updateView({ columnOrder: next })
            }
          }
          preview.current?.finish(); setDragging(null)
        }}>
        {matches.length ? matches.map(field => {
          const Icon = fieldIcons[field.type], isHidden = hidden.includes(field.id)
          return <div key={field.id} data-drag-field={field.id} className="group flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/60">
            <Button size="icon-xs" variant="ghost" className="shrink-0 cursor-grab text-muted-foreground" disabled={disabled || !props.view || !props.capabilities.canManageViews} draggable={!disabled && props.capabilities.canManageViews} aria-label={copy.ui.tableColumns.drag(field.name)} title={copy.ui.tableColumns.drag(field.name)} onDragStart={event => { setDragging(field.id); event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', field.id); const row = event.currentTarget.closest<HTMLElement>('[data-drag-field]'); if (row) preview.current = createDragPreview(row, event.nativeEvent, () => Array.from(row.parentElement?.querySelectorAll<HTMLElement>('[data-drag-field]') ?? []), true) }} onDragEnd={() => { preview.current?.finish(); setDragging(null) }} onKeyDown={event => { if (event.altKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) { event.preventDefault(); const targetId = preference.order[preference.order.indexOf(field.id) + (event.key === 'ArrowUp' ? -1 : 1)]; if (targetId) reorder(field.id, targetId) } }}><PhDotsSixVertical className="size-3.5" aria-hidden="true" /></Button>
            <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" /><span className={`min-w-0 flex-1 truncate text-sm ${isHidden ? 'text-muted-foreground' : ''}`} title={field.name}>{field.name}</span>
            <Button size="icon-xs" variant="ghost" aria-label={isHidden ? text.showField(field.name) : text.hideField(field.name)} title={isHidden ? text.showField(field.name) : text.hideField(field.name)} disabled={disabled || !props.view || !props.capabilities.canManageViews || (!isHidden && visibleCount <= 1)} onClick={() => void toggle(field)}>{isHidden ? <PhEyeSlash className="size-3.5 text-muted-foreground" /> : <PhEye className="size-3.5 text-muted-foreground" />}</Button>
            {editable ? <Popover open={target === field.id} onOpenChange={next => { if (!disabled) setTarget(next ? field.id : null) }}><PopoverTrigger render={<Button size="icon-xs" variant="ghost" />} aria-label={text.fieldMore(field.name)} title={text.fieldMore(field.name)} disabled={disabled}><PhDotsThree className="size-4 text-muted-foreground" /></PopoverTrigger>{content(field)}</Popover> : null}
          </div>
        }) : <EmptyState title={text.fieldsEmpty} />}
      </div>
      {error ? <AlertBanner tone="error">{error}</AlertBanner> : null}
      {editable && props.onCreateField ? <div className="border-t border-border p-1"><Popover open={target === 'create'} onOpenChange={next => { if (!disabled) setTarget(next ? 'create' : null) }}><PopoverTrigger render={<Button variant="ghost" size="sm" />} className="w-full justify-start gap-2 px-2" disabled={disabled}><PhPlus className="size-4" />{text.addField}</PopoverTrigger>{content()}</Popover></div> : null}
    </PopoverContent>
  </Popover>
}
