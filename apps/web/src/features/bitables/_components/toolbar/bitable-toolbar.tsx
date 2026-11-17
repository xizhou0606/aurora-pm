import { useState } from 'react'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button, Popover, PopoverContent, PopoverTrigger } from '@/shared/ui'
import { PhKanban, PhPencil, PhPlus, PhTable, PhX } from '@/shared/ui/icons'
import { displayCellValue } from '../../model/fields'
import type { ViewToolbarProps } from '../../model/grid-port'
import { SchemaPanel, type SchemaPanelTarget } from './schema-panel'
import { SearchControl } from './search-control'
import { ViewSettingPopovers } from './view-setting-popovers'
import { FieldConfigPopover } from './field-config-popover'

export function BitableToolbar(props: ViewToolbarProps & { hideViewTabs?: boolean }) {
  const [panel, setPanel] = useState<SchemaPanelTarget | null>(null)
  const active = props.views.find((view) => view.id === props.activeViewId)
  return <div className="shrink-0 border-b border-border bg-card" data-slot="bitable-toolbar">
    {!props.hideViewTabs ? <div role="tablist" aria-label={text.viewType} className="flex min-w-0 items-center gap-1 overflow-x-auto px-3 pb-1 pt-2">
      {props.views.map((view) => {
        const Icon = view.kind === 'grid' ? PhTable : PhKanban
        return <Button key={view.id} role="tab" aria-selected={view.id === props.activeViewId} variant={view.id === props.activeViewId ? 'secondary' : 'ghost'} size="sm" className={`h-7 max-w-48 shrink-0 gap-1.5 rounded-full px-3 text-xs ${view.id === props.activeViewId ? 'border border-border/70 shadow-sm' : 'text-muted-foreground'}`} title={view.name} disabled={props.pending} onClick={() => props.onViewSelect(view.id)}><Icon aria-hidden="true" className="size-3.5 shrink-0" /><span className="truncate">{view.name}</span></Button>
      })}
      {active && props.capabilities.canManageViews ? <Popover open={panel?.kind === 'rename-view'} onOpenChange={open => setPanel(open ? { kind: 'rename-view', view: active } : null)}><PopoverTrigger render={<Button variant="ghost" size="icon-xs" />} title={text.renameView} aria-label={text.renameView} disabled={props.pending}><PhPencil className="size-3.5 text-muted-foreground" aria-hidden="true" /></PopoverTrigger><PopoverContent className="w-72">{panel?.kind === 'rename-view' ? <SchemaPanel target={panel} presentation="popover" onClose={() => setPanel(null)} onSave={name => props.onViewRename(panel.view.id, name)} /> : null}</PopoverContent></Popover> : null}
      {props.capabilities.canManageViews ? <Button variant="ghost" size="icon-sm" aria-label={text.createView} disabled={props.pending} onClick={() => setPanel({ kind: 'create-view' })}><PhPlus aria-hidden="true" /></Button> : null}
    </div> : null}
    <div className="flex flex-wrap items-center gap-1 px-3 py-2">
      {props.hideViewTabs && props.capabilities.canManageViews ? <Button variant="ghost" size="icon-xs" aria-label={text.createView} title={text.createView} disabled={props.pending} onClick={() => setPanel({ kind: 'create-view' })}><PhPlus aria-hidden="true" /></Button> : null}
      {props.hideViewTabs && active && props.capabilities.canManageViews ? <Popover open={panel?.kind === 'rename-view'} onOpenChange={open => setPanel(open ? { kind: 'rename-view', view: active } : null)}><PopoverTrigger render={<Button variant="ghost" size="icon-xs" />} aria-label={text.renameView} disabled={props.pending}><PhPencil className="size-3.5" /></PopoverTrigger><PopoverContent className="w-72">{panel?.kind === 'rename-view' ? <SchemaPanel target={panel} presentation="popover" onClose={() => setPanel(null)} onSave={name => props.onViewRename(panel.view.id, name)} /> : null}</PopoverContent></Popover> : null}
      <FieldConfigPopover fields={props.fields} view={active} capabilities={props.capabilities} pending={props.pending} createFieldRequest={props.createFieldRequest} onCreateField={props.onCreateField} onRenameField={props.onRenameField} onUpdateField={props.onUpdateField} onViewConfigChange={props.onViewConfigChange} />
      {active ? <ViewSettingPopovers fields={props.fields} view={active} readOnly={!props.capabilities.canManageViews} pending={props.pending} onSave={props.onViewConfigChange} /> : null}
      <div className="min-w-0 flex-1 basis-2" />
      <SearchControl control={props.search} />
    </div>
    {active && (active.config.filters.length > 0 || active.config.sorts.length > 0 || active.config.groupFieldId) ? <div className="flex flex-wrap items-center gap-1.5 px-3 pb-2" data-slot="bitable-active-settings">
      {active.config.filters.map((filter, index) => {
        const field = props.fields.find((item) => item.id === filter.fieldId)
        const value = field ? displayCellValue(field, filter.value ?? null) : String(filter.value ?? '')
        return <span key={`${filter.fieldId}-${index}`} className="inline-flex max-w-full items-center gap-1 rounded-md border border-border/70 bg-muted/50 px-2 py-0.5 text-xs" title={text.filterChipTitle(field?.name ?? filter.fieldId, text.operators[filter.operator], value)}>
          <span className="max-w-64 truncate"><span className="font-medium">{field?.name ?? filter.fieldId}</span> <span className="text-muted-foreground">{text.operators[filter.operator]}</span> {value}</span>
          {props.capabilities.canManageViews ? <Button size="icon-xs" variant="ghost" className="size-5 shrink-0" aria-label={text.removeFilter} disabled={props.pending} onClick={() => void props.onViewConfigChange(active.id, { ...active.config, filters: active.config.filters.filter((_, at) => at !== index) })}><PhX className="size-3" aria-hidden="true" /></Button> : null}
        </span>
      })}
      {active.config.sorts.map((sort) => <span key={sort.fieldId} className="max-w-64 truncate rounded-md border border-border/70 px-2 py-1 text-xs text-muted-foreground">{text.sortLabel(props.fields.find((field) => field.id === sort.fieldId)?.name ?? sort.fieldId)} {sort.direction === 'asc' ? text.ascending : text.descending}</span>)}
      {active.config.groupFieldId ? <span className="inline-flex max-w-64 items-center gap-1 rounded-md border border-border/70 px-2 py-0.5 text-xs text-muted-foreground"><span className="truncate">{text.groupLabel(props.fields.find((field) => field.id === active.config.groupFieldId)?.name ?? active.config.groupFieldId)}</span>{props.capabilities.canManageViews ? <Button size="icon-xs" variant="ghost" className="size-5 shrink-0" aria-label={text.clearGroup} disabled={props.pending} onClick={() => void props.onViewConfigChange(active.id, { ...active.config, groupFieldId: undefined })}><PhX className="size-3" aria-hidden="true" /></Button> : null}</span> : null}
    </div> : null}
    {panel && panel.kind !== 'rename-view' ? <SchemaPanel target={panel} onClose={() => setPanel(null)} onSave={(name, type, kind, choices, options) => {
      switch (panel.kind) {
        case 'create-view': return props.onViewCreate(name, kind)
        case 'create-field': return props.onCreateField?.(name, type, choices, options) ?? Promise.resolve(false)
        case 'rename-field': return props.onRenameField?.(panel.field.id, name) ?? Promise.resolve(false)
      }
    }} /> : null}
  </div>
}
