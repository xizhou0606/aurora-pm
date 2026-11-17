import { useEffect, useRef, useState } from 'react'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { PhDotsThree, PhKanban, PhTable } from '@/shared/ui/icons'
import type { GridCommands, ViewToolbarProps } from '../../model/grid-port'
import { BitableToolbar } from './bitable-toolbar'

type Props = {
  resourceName: string; tables: readonly { id: string; name: string }[]; tableId: string; canSwitchTable: boolean
  onTableSelect: (id: string) => void; toolbar: ViewToolbarProps; commands: GridCommands; onTransfer: () => void
}
export function EmbeddedToolbar(props: Props) {
  const [expanded, setExpanded] = useState(false)
  const request = useRef(props.toolbar.createFieldRequest ?? 0)
  const activeView = props.toolbar.views.find(view => view.id === props.toolbar.activeViewId)
  useEffect(() => {
    const next = props.toolbar.createFieldRequest ?? 0
    if (next !== request.current && next > 0) setExpanded(true)
    request.current = next
  }, [props.toolbar.createFieldRequest])
  const Icon = activeView?.kind === 'kanban' ? PhKanban : PhTable
  return <div className="shrink-0" data-slot="bitable-embedded-controls">
    <div className="flex h-10 min-w-0 items-center gap-1.5 border-b border-border/60 pl-3 pr-10" data-slot="bitable-embedded-header">
      <PhTable className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <span className="min-w-0 flex-1 truncate text-xs font-medium" title={props.resourceName}>{props.resourceName}</span>
      {props.tables.length > 1 ? <Select value={props.tableId} items={props.tables.map(table => ({ value: table.id, label: table.name }))} disabled={props.toolbar.pending || !props.canSwitchTable} onValueChange={id => { if (id) props.onTableSelect(id) }}><SelectTrigger aria-label={copy.chooseTable} className="h-7 w-auto max-w-24 border-transparent bg-transparent px-1.5 text-xs shadow-none"><SelectValue /></SelectTrigger><SelectContent>{props.tables.map(table => <SelectItem key={table.id} value={table.id}>{table.name}</SelectItem>)}</SelectContent></Select> : null}
      <span className="h-4 w-px shrink-0 bg-border" aria-hidden="true" />
      <Select value={props.toolbar.activeViewId} items={props.toolbar.views.map(view => ({ value: view.id, label: view.name }))} disabled={props.toolbar.pending} onValueChange={id => { if (id) props.toolbar.onViewSelect(id) }}><SelectTrigger aria-label={text.viewType} className="h-7 w-auto max-w-32 gap-1.5 border-transparent bg-transparent px-1.5 text-xs shadow-none"><Icon className="size-3.5 shrink-0" aria-hidden="true" /><SelectValue /></SelectTrigger><SelectContent>{props.toolbar.views.map(view => <SelectItem key={view.id} value={view.id}>{view.name}</SelectItem>)}</SelectContent></Select>
      <Button size="icon-xs" variant="ghost" aria-label={expanded ? text.collapseTools : text.expandTools} title={text.embeddedTools} aria-expanded={expanded} onClick={() => setExpanded(value => !value)}><PhDotsThree className="size-4" /></Button>
    </div>
    <div hidden={!expanded} data-slot="bitable-embedded-tools">
      <BitableToolbar {...props.toolbar} hideViewTabs />
      <div className="flex flex-wrap items-center gap-1 border-b border-border/60 px-2 py-1">
        {props.toolbar.capabilities.canCreateRecords ? <Button size="xs" variant="ghost" disabled={props.toolbar.pending} onClick={() => void props.commands.onCreateEmpty()}>{text.addRow}</Button> : null}
        {(props.toolbar.capabilities.canImport || props.toolbar.capabilities.canExport) ? <Button size="xs" variant="ghost" disabled={props.toolbar.pending} onClick={props.onTransfer}>{copy.transfer}</Button> : null}
      </div>
    </div>
  </div>
}
