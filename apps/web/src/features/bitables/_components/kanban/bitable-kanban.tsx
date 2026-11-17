import { useEffect, useRef } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { AlertBanner, Button, EmptyState, LoadingOverlay, LoadingSkeleton, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { displayCellValue } from '../../model/fields'
import type { GridProps } from '../../model/grid-port'
import type { Row } from '../../model/types'
import { GridFooter } from '../grid/grid-footer'
import { CellEditor } from '../grid/cell-editor'
import { sameCell, useGridInteraction } from '../../hooks/grid/use-grid-interaction'

export function BitableKanban(props: GridProps) {
  const { state, config, fields, capabilities, commands } = props
  const rows = state.status === 'ready' ? state.rows : []
  const group = fields.find((field) => field.id === config.groupFieldId && field.type === 'single_select')
  const interaction = useGridInteraction({ instanceKey: props.instanceKey, rows, fields, writableFieldIds: capabilities.canUpdateRecords ? capabilities.writableFieldIds : [], commands })
  const dragging = useRef<Row | null>(null)
  useEffect(() => { props.onEditingChange?.(interaction.editor.phase !== 'view') }, [interaction.editor.phase, props.onEditingChange])
  const groups = [{ id: '', name: text.ungrouped }, ...(group?.options?.choices ?? [])]
  const visibleFields = fields.filter((field) => field.id !== group?.id && !config.hiddenFieldIds.includes(field.id))
  const writableGroup = Boolean(group && !group.readOnly && capabilities.canUpdateRecords && capabilities.writableFieldIds.includes(group.id))
  const move = (row: Row, value: string) => {
    if (!group || !writableGroup || commands.pending || interaction.editor.phase !== 'view' || row.values[group.id] === (value || null)) return
    const baseRevision = row.fieldRevisions[group.id]
    if (!Number.isSafeInteger(baseRevision)) return
    void commands.onEdit({ address: { recordId: row.id, fieldId: group.id }, value: value || null, baseRevision })
  }
  if (state.status === 'loading') return <LoadingSkeleton label={BITABLE_COPY.loading} rows={6} />
  if (state.status === 'error') return <AlertBanner tone="error" action={<Button onClick={state.onRetry}>{BITABLE_COPY.retry}</Button>}>{state.message}</AlertBanner>
  if (!group) return <EmptyState title={text.chooseBoardGroup} />
  return <section className="flex size-full min-h-0 min-w-0 flex-col overflow-hidden" aria-label={text.kanban} data-slot="bitable-kanban" onKeyDown={(event) => {
    if ((event.target as HTMLElement).closest('[data-bitable-editor]')) return
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
      event.stopPropagation(); event.preventDefault()
      if (!commands.pending && interaction.editor.phase === 'view' && (event.shiftKey ? commands.canRedo : commands.canUndo)) void (event.shiftKey ? commands.onRedo() : commands.onUndo())
    }
  }}>
    {state.refresh === 'stale_error' ? <AlertBanner tone="warning">{text.staleError}</AlertBanner> : null}
    <div className="relative flex min-h-0 flex-1 gap-3 overflow-auto overscroll-contain bg-muted/20 p-3">
      {groups.map((choice) => {
        const cards = rows.filter((row) => {
          const value = row.values[group.id] ?? ''
          // A removed/unknown choice must not hide a persisted record.
          const known = groups.some((entry) => entry.id === value)
          return (known ? value : '') === choice.id
        })
        return <section key={choice.id} aria-label={choice.name} className="flex w-64 shrink-0 flex-col rounded-lg border border-border bg-muted/30" onDragOver={(event) => { if (dragging.current && writableGroup) event.preventDefault() }} onDrop={(event) => { event.preventDefault(); const row = dragging.current; dragging.current = null; if (row) move(row, choice.id) }}>
          <header className="flex items-center justify-between border-b border-border p-3 text-sm font-medium"><span>{choice.name}</span><span className="text-xs text-muted-foreground">{cards.length}</span></header>
          <div className="grid content-start gap-2 p-2">
            {!cards.length ? <EmptyState title={BITABLE_COPY.empty} /> : cards.map((row) => <article key={row.id} className="grid gap-2 rounded-lg border border-border bg-card p-3 shadow-sm" draggable={writableGroup && !commands.pending && interaction.editor.phase === 'view'} onDragStart={(event) => { dragging.current = row; event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', row.id) }} onDragEnd={() => { dragging.current = null }}>
              {visibleFields.map((field) => {
                const address = { recordId: row.id, fieldId: field.id }
                const editor = interaction.editor
                return <div key={field.id} className="min-w-0"><span className="block text-xs text-muted-foreground">{field.name}</span>
                  {editor.phase !== 'view' && sameCell(editor.address, address) ? <CellEditor field={field} state={editor} people={props.people ?? []} onChange={interaction.change} onCommit={interaction.commit} onCancel={interaction.cancel} onComposition={(value) => { interaction.composing.current = value }} /> : <Button variant="ghost" size="sm" className="h-auto min-h-8 w-full justify-start whitespace-pre-wrap break-words px-0 text-left" disabled={!capabilities.canUpdateRecords || !capabilities.writableFieldIds.includes(field.id) || field.readOnly || commands.pending} onClick={() => interaction.begin(address)}>{displayCellValue(field, row.values[field.id] ?? null) || text.emptyValue}</Button>}
                </div>
              })}
              {writableGroup ? <Select value={choice.id} items={groups.map((item) => ({ value: item.id, label: item.name }))} disabled={commands.pending || interaction.editor.phase !== 'view'} onValueChange={(value) => { if (value !== null) move(row, value) }}><SelectTrigger aria-label={text.moveRecord}><SelectValue /></SelectTrigger><SelectContent>{groups.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent></Select> : null}
            </article>)}
          </div>
        </section>
      })}
      {state.refresh === 'refreshing' ? <LoadingOverlay label={BITABLE_COPY.loading} /> : null}
    </div>
    {!props.hideFooter ? <GridFooter total={state.total} commands={commands} canCreate={capabilities.canCreateRecords} editing={interaction.editor.phase !== 'view'} onCreated={() => undefined} /> : null}
  </section>
}
