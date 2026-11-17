import { useEffect, useImperativeHandle, useMemo, useRef, useState, type Ref } from 'react'
import { getCoreRowModel, useReactTable, type ColumnDef } from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { BITABLE_COLLABORATION_COPY as collaborationCopy } from '@/shared/copy/bitables-collaboration'
import { cn } from '@/shared/lib/utils'
import { PhPlus } from '@/shared/ui/icons'
import { AlertBanner, Button, ConfirmDialog, EmptyState, LoadingSkeleton, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, clipboardImageFiles } from '@/shared/ui'
import { displayCellValue } from '../../model/fields'
import type { CellAddress, Row } from '../../model/types'
import type { GridPort, GridProps } from '../../model/grid-port'
import { sameCell, useGridInteraction } from '../../hooks/grid/use-grid-interaction'
import { useGridRows } from '../../hooks/grid/use-grid-rows'
import { CellEditor } from './cell-editor'
import { GridCreateRow, GridFooter } from './grid-footer'
import { GridRowMenu } from './grid-row-menu'
import { GridCellDisplay } from './grid-cell-display'
import { GridColumnHeader } from './grid-column-header'
import { GridRowNumber } from './grid-row-number'
import { useGridRowReorder } from '../../hooks/grid/use-grid-row-reorder'

export function TanStackGrid({ ref, ...props }: GridProps & { ref?: Ref<GridPort> }) {
  'use no memo'
  const { fields, config, state, commands, capabilities } = props
  const rows = state.status === 'ready' ? state.rows : []
  const visibleFields = useMemo(() => {
    const order = new Map(config.columnOrder.map((id, index) => [id, index]))
    return fields.filter((field) => !config.hiddenFieldIds.includes(field.id))
      .sort((left, right) => (order.get(left.id) ?? fields.indexOf(left) + fields.length) - (order.get(right.id) ?? fields.indexOf(right) + fields.length))
  }, [config.columnOrder, config.hiddenFieldIds, fields])
  const columns = useMemo<ColumnDef<Row>[]>(() => visibleFields.map((field) => ({ id: field.id, accessorFn: (row) => row.values[field.id], header: field.name })), [visibleFields])
  const table = useReactTable({ data: rows as Row[], columns, getRowId: (row) => row.id, getCoreRowModel: getCoreRowModel() })
  const scrollRef = useRef<HTMLDivElement>(null)
  const cellRefs = useRef(new Map<string, HTMLTableCellElement>())
  const rowHeight = Math.min(144, Math.max(32, config.rowHeight))
  const grouped = useGridRows(rows, fields.find((field) => field.id === config.groupFieldId), props.instanceKey)
  const virtual = useVirtualizer({ count: grouped.items.length, getScrollElement: () => scrollRef.current, estimateSize: (index) => grouped.items[index]?.kind === 'group' ? 32 : rowHeight, overscan: 8, initialRect: { height: 480, width: 900 } })
  const interaction = useGridInteraction({ instanceKey: props.instanceKey, rows, fields: visibleFields, writableFieldIds: capabilities.canUpdateRecords ? capabilities.writableFieldIds : [], commands })
  const [deleting, setDeleting] = useState<Row | null>(null)
  const [pendingFocus, setPendingFocus] = useState<CellAddress | null>(null)
  const currentInstance = useRef(props.instanceKey); currentInstance.current = props.instanceKey
  const key = (address: CellAddress) => JSON.stringify([address.recordId, address.fieldId])
  const reveal = (address: CellAddress, focus = false) => {
    if (interaction.isEditing()) return
    const owner = props.instanceKey
    grouped.reveal(address.recordId)
    const index = grouped.items.findIndex((row) => row.kind === 'record' && row.id === address.recordId)
    if (index < 0) return
    virtual.scrollToIndex(index, { align: 'auto' })
    requestAnimationFrame(() => {
      if (interaction.isEditing() || currentInstance.current !== owner) return
      const cell = cellRefs.current.get(key(address))
      const parent = scrollRef.current
      if (!cell || !parent) return
      const cellRect = cell.getBoundingClientRect()
      const viewport = parent.getBoundingClientRect()
      // Virtual indexes exclude the sticky header. Correct against the actual visible
      // body rectangle without scrollIntoView, which would move document ancestors.
      const headerBottom = parent.querySelector('thead')?.getBoundingClientRect().bottom ?? viewport.top
      if (cellRect.top < headerBottom) parent.scrollTop -= headerBottom - cellRect.top
      else if (cellRect.bottom > viewport.bottom) parent.scrollTop += cellRect.bottom - viewport.bottom
      if (cellRect.left < viewport.left) parent.scrollLeft -= viewport.left - cellRect.left
      else if (cellRect.right > viewport.right) parent.scrollLeft += cellRect.right - viewport.right
      if (focus) { interaction.select(address); cell.focus({ preventScroll: true }) }
    })
  }
  useImperativeHandle(ref, () => ({ focusCell: (address) => reveal(address, true), revealCell: reveal, getSelection: () => interaction.selection }))
  const currentMatch = props.search?.matches[props.search.current]
  useEffect(() => { if (currentMatch) reveal(currentMatch) }, [currentMatch?.recordId, currentMatch?.fieldId, rows, grouped.items])
  useEffect(() => { setDeleting(null); setPendingFocus(null); if (scrollRef.current) { scrollRef.current.scrollTop = 0; scrollRef.current.scrollLeft = 0 } }, [props.instanceKey])
  const items = virtual.getVirtualItems()
  const top = items[0]?.start ?? 0
  const bottom = virtual.getTotalSize() - (items.at(-1)?.end ?? 0)
  const active = interaction.selection?.focus
  const activeEditor = interaction.editor
  const reorder = useGridRowReorder({ instanceKey: props.instanceKey, rows, config, canUpdate: capabilities.canUpdateRecords, disabled: commands.pending || activeEditor.phase !== 'view', onMove: props.onMoveRow })
  const previousPhase = useRef(activeEditor.phase)
  useEffect(() => {
    if (previousPhase.current !== 'view' && activeEditor.phase === 'view' && active) setPendingFocus(active)
    previousPhase.current = activeEditor.phase
  }, [activeEditor.phase, active])
  useEffect(() => {
    if (activeEditor.phase !== 'view') { setPendingFocus(null); return }
    if (!pendingFocus || !rows.some((row) => row.id === pendingFocus.recordId)) return
    reveal(pendingFocus, true)
    setPendingFocus(null)
  }, [pendingFocus, rows, activeEditor.phase])
  useEffect(() => { props.onEditingChange?.(activeEditor.phase !== 'view') }, [activeEditor.phase, props.onEditingChange])
  const move = (dr: number, dc: number, extend = false) => {
    const next = interaction.move(dr, dc, extend)
    if (next) reveal(next, !extend)
  }
  const handleKey = (event: React.KeyboardEvent) => {
    if ((event.target as HTMLElement).closest('[data-bitable-editor]')) return
    if ((event.target as HTMLElement).closest('button, input, [role="menu"]')) return
    event.stopPropagation()
    if (event.nativeEvent.isComposing || event.keyCode === 229 || commands.pending) return
    const modifier = event.metaKey || event.ctrlKey
    if (modifier && event.key.toLowerCase() === 'z') {
      event.preventDefault()
      if (event.shiftKey ? commands.canRedo : commands.canUndo) void (event.shiftKey ? commands.onRedo() : commands.onUndo())
      return
    }
    if (activeEditor.phase !== 'view') return
    if (event.key === 'ArrowDown') { event.preventDefault(); move(1, 0, event.shiftKey) }
    else if (event.key === 'ArrowUp') { event.preventDefault(); move(-1, 0, event.shiftKey) }
    else if (event.key === 'ArrowLeft') { event.preventDefault(); move(0, -1, event.shiftKey) }
    else if (event.key === 'ArrowRight') { event.preventDefault(); move(0, 1, event.shiftKey) }
    else if ((event.key === 'Enter' || event.key === 'F2') && active) { event.preventDefault(); interaction.begin(active) }
    else if (event.key === 'Tab' && active) {
      const column = visibleFields.findIndex((field) => field.id === active.fieldId)
      const row = rows.findIndex((entry) => entry.id === active.recordId)
      const last = event.shiftKey ? column === 0 && row === 0 : column === visibleFields.length - 1 && row === rows.length - 1
      if (last) return
      event.preventDefault()
      if (!event.shiftKey && column === visibleFields.length - 1) move(1, -column)
      else if (event.shiftKey && column === 0) move(-1, visibleFields.length - 1)
      else move(0, event.shiftKey ? -1 : 1)
    } else if (active && !modifier && !event.altKey && event.key.length === 1) { event.preventDefault(); interaction.begin(active, event.key) }
  }
  return (
    <section className="flex size-full min-h-0 min-w-0 flex-col overflow-hidden bg-card" data-slot="bitable-grid" onKeyDown={handleKey}
      onCopy={(event) => { if ((event.target as HTMLElement).closest('input, textarea, [role="menu"]')) return; if (activeEditor.phase === 'view' && interaction.selection) { event.preventDefault(); event.stopPropagation(); event.clipboardData.setData('text/plain', commands.onCopy(interaction.selection)) } }}
      onPaste={(event) => {
        if ((event.target as HTMLElement).closest('input, textarea, [role="menu"], [contenteditable="true"]') || activeEditor.phase !== 'view' || !active || commands.pending) return
        const field = visibleFields.find(item => item.id === active.fieldId)
        if (field?.type === 'attachment') {
          const row = rows.find(item => item.id === active.recordId)
          const file = clipboardImageFiles(event.clipboardData, item => item.type.startsWith('image/'))[0]
          if (!file || !row || field.readOnly || !capabilities.canUpdateRecords || !capabilities.canUploadAttachments || !capabilities.writableFieldIds.includes(field.id) || !props.onUploadAttachment) return
          event.preventDefault(); event.stopPropagation()
          void props.onUploadAttachment(active, row.fieldRevisions[field.id], file)
          return
        }
        const value = event.clipboardData.getData('text/plain')
        if (!value || !capabilities.canUpdateRecords) return
        event.preventDefault(); event.stopPropagation(); void commands.onPaste(active, value)
      }}>
      {state.status === 'loading' ? <LoadingSkeleton label={BITABLE_COPY.loading} rows={8} className="p-4" /> : state.status === 'error' ? (
        <div className="p-4"><EmptyState title={BITABLE_COPY.error} /><AlertBanner tone="error" action={<Button variant="outline" onClick={state.onRetry}>{BITABLE_COPY.retry}</Button>}>{state.message}</AlertBanner></div>
      ) : <>
        {state.refresh === 'stale_error' ? <AlertBanner tone="warning">{text.staleError}</AlertBanner> : null}
        <div ref={scrollRef} className="relative min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain" data-slot="bitable-grid-scroll" onDragOver={event => reorder.over(undefined, event)} onDrop={event => reorder.drop(undefined, event)}>
          <Table role="grid" aria-label={text.grid} aria-rowcount={state.total + 2} aria-colcount={visibleFields.length + 2} containerClassName="overflow-visible" className="w-full min-w-(--table-min-w) table-fixed border-collapse" style={{ '--table-min-w': `${104 + visibleFields.reduce((width, field) => width + (config.columnWidths[field.id] ?? 180), 0)}px` }}>
            <TableHeader className="sticky top-0 z-20 bg-card"><TableRow><TableHead className="sticky left-0 z-30 h-9 w-16 min-w-16 max-w-16 border-r border-border/50 bg-muted px-1 text-center text-xs font-normal">{text.rowNumber}</TableHead>
              {visibleFields.map((field, index) => <GridColumnHeader key={field.id} field={field} width={config.columnWidths[field.id] ?? 180} stretch={index === visibleFields.length - 1} disabled={!capabilities.canManageViews || commands.pending} onWidthChange={props.onColumnWidthChange} />)}
              <TableHead className="sticky right-0 z-30 h-9 w-10 border-l border-border/50 bg-muted px-0 text-center" />
            </TableRow></TableHeader>
            <TableBody onDragOver={event => reorder.over(undefined, event)} onDrop={event => reorder.drop(undefined, event)}>
              {top > 0 ? <TableRow aria-hidden="true"><TableCell colSpan={visibleFields.length + 2} className="h-(--spacer-top-h) p-0" style={{ '--spacer-top-h': `${top}px` }} /></TableRow> : null}
              {items.map((item) => {
                const display = grouped.items[item.index]
                if (display.kind === 'group') return <TableRow key={`group:${display.id}`} ref={virtual.measureElement} data-index={item.index} className="bg-muted/50"><TableCell colSpan={visibleFields.length + 2} className="p-0"><Button variant="ghost" size="sm" className="w-full justify-start" aria-expanded={display.expanded} onClick={() => grouped.toggle(display.id)}>{text.groupCount(display.name, display.count)}</Button></TableCell></TableRow>
                const row = table.getRowModel().rows[display.index]?.original
                if (!row) return null
                return <GridRowMenu key={row.id} row={row} number={state.offset + display.index + 1} canCreate={capabilities.canCreateRecords} canDelete={capabilities.canDeleteRecords} canComment={capabilities.canComment} disabled={commands.pending || activeEditor.phase !== 'view'} onDelete={() => setDeleting(row)} onInsertRows={props.onInsertRows} onRecordOpen={props.onRecordOpen} onRecordLink={props.onRecordLink}
                  rowProps={{ ref: virtual.measureElement, 'data-index': item.index, 'data-record-id': row.id, 'aria-rowindex': state.offset + display.index + 2, 'data-drop-position': reorder.target?.id === row.id ? reorder.target.direction : undefined, style: { height: rowHeight }, className: 'group/record border-border/50 hover:bg-muted/40', onDragOver: (event: React.DragEvent<HTMLElement>) => reorder.over(row, event), onDrop: (event: React.DragEvent<HTMLElement>) => reorder.drop(row, event) } as React.ComponentProps<typeof TableRow>}>
                  <GridRowNumber number={state.offset + display.index + 1} selected={interaction.isRowSelected(row.id)} disabled={commands.pending || activeEditor.phase !== 'view'} canMove={reorder.canMove} onSelect={checked => checked ? interaction.selectRow(row.id) : interaction.clearSelection()} onDragStart={event => reorder.start(row, event)} onDragEnd={reorder.end} onKeyboardMove={direction => reorder.keyboard(row, direction)} />
                  {visibleFields.map((field) => {
                    const address = { recordId: row.id, fieldId: field.id }
                    const editing = activeEditor.phase !== 'view' && sameCell(activeEditor.address, address)
                    const hit = props.search?.matches.some((match) => sameCell(match, address))
                    const occupants = props.presence?.occupiedCells.find((entry) => sameCell(entry.address, address))?.people ?? []
                    const occupantColor = (occupants.find(person => person.editing) ?? occupants[0])?.color
                    return <TableCell key={field.id} ref={(element) => { if (element) cellRefs.current.set(key(address), element); else cellRefs.current.delete(key(address)) }} role="gridcell" aria-label={text.cell(field.name, state.offset + display.index + 1)} aria-selected={interaction.isSelected(address)} tabIndex={sameCell(active, address) || (!active && display.index === 0 && field.id === visibleFields[0]?.id) ? 0 : -1}
                      data-record-id={row.id} data-field-id={field.id} data-search-match={hit || undefined} data-presence-occupied={occupants.length ? 'true' : undefined}
                      // eslint-disable-next-line shadcn/require-static-classes -- 协作占位色来自运行时按用户分配的类名映射，无法静态化
                      className={cn('relative w-(--col-w) max-w-(--col-w) border-r border-border/50 p-0 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary', occupantColor && ['ring-2 ring-inset', occupantColor.ring, occupantColor.selection], interaction.isSelected(address) && 'bg-primary/10', hit && !interaction.isSelected(address) && 'bg-amber-100/70 dark:bg-amber-900/40', sameCell(currentMatch, address) && 'ring-2 ring-inset ring-amber-500', (sameCell(active, address) || editing) && 'z-10 ring-2 ring-inset ring-primary')}
                      style={{ '--col-w': `${config.columnWidths[field.id] ?? 180}px` }}
                      onFocus={() => { if (!editing && !interaction.selection) interaction.select(address) }}
                      onPointerDown={(event) => { if (event.button === 0 && !editing) { event.preventDefault(); event.currentTarget.focus({ preventScroll: true }); interaction.pointerDown(address, event.shiftKey) } }}
                      onPointerEnter={() => interaction.pointerEnter(address)}
                      onClick={event => { if (field.type === 'date' && !editing && !event.shiftKey) interaction.begin(address); else if (field.type === 'attachment' && !editing) interaction.select(address) }}
                      onDoubleClick={() => interaction.begin(address)}>
                      {editing ? <CellEditor field={field} state={activeEditor} people={props.people ?? []} height={rowHeight - 1} onChange={interaction.change} onCommit={interaction.commit} onCancel={interaction.cancel} onComposition={(value) => { interaction.composing.current = value }} onTab={(direction) => { const column = visibleFields.indexOf(field); if (direction > 0 && column === visibleFields.length - 1) move(1, -column); else if (direction < 0 && column === 0) move(-1, visibleFields.length - 1); else move(0, direction) }} />
                        : <GridCellDisplay field={field} row={row} value={row.values[field.id] ?? null} height={rowHeight} text={displayCellValue(field, row.values[field.id] ?? null)} capabilities={capabilities} pending={commands.pending} onUpload={props.onUploadAttachment} onPreview={props.onPreviewAttachment} onDownload={props.onDownloadAttachment} onLoadPreview={props.onLoadAttachmentPreview} />}
                      {occupants.length ? <span className="pointer-events-none absolute top-full right-0 z-20 flex max-w-full gap-1 text-xs leading-4" title={collaborationCopy.occupiedBy(occupants.map((person) => person.name))} aria-label={collaborationCopy.occupiedBy(occupants.map((person) => person.name))}>
                        {occupants.map(person => <span key={person.id} className={cn('truncate rounded-b px-1', person.color?.label ?? 'bg-emerald-100 text-emerald-900')}>{collaborationCopy.cellParticipant(person.name, Boolean(person.editing))}</span>)}
                      </span> : null}
                    </TableCell>
                  })}
                </GridRowMenu>
              })}
              {bottom > 0 ? <TableRow aria-hidden="true"><TableCell colSpan={visibleFields.length + 2} className="h-(--spacer-bottom-h) p-0" style={{ '--spacer-bottom-h': `${bottom}px` }} /></TableRow> : null}
              <TableRow><TableCell colSpan={visibleFields.length + 2} className="p-0"><GridCreateRow commands={commands} canCreate={capabilities.canCreateRecords} editing={activeEditor.phase !== 'view'} onCreated={(row) => { if (interaction.isEditing()) return; const field = visibleFields.find((entry) => capabilities.writableFieldIds.includes(entry.id)); if (field) setPendingFocus({ recordId: row.id, fieldId: field.id }) }} /></TableCell></TableRow>
            </TableBody>
          </Table>
          {!rows.length ? <EmptyState title={BITABLE_COPY.empty} /> : null}
          {state.refresh === 'refreshing' ? <span role="status" className="sr-only">{BITABLE_COPY.loading}</span> : null}
        </div>
        {!props.hideFooter ? <GridFooter total={state.total} commands={commands} editing={activeEditor.phase !== 'view'} /> : null}
      </>}
      <ConfirmDialog open={Boolean(deleting)} onOpenChange={(open) => { if (!open) setDeleting(null) }} title={text.deleteTitle} description={text.deleteDescription} cancelLabel={BITABLE_COPY.cancel} confirmLabel={text.deleteRow} pending={commands.pending} onConfirm={() => { if (deleting) void commands.onDelete(deleting).then((ok) => { if (ok) setDeleting(null) }) }} />
    </section>
  )
}
