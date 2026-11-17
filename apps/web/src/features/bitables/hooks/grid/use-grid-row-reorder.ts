import { useEffect, useRef, useState, type DragEvent } from 'react'
import type { Row, ViewConfig } from '../../model/types'
import { createDragPreview, type DragPreview } from '@/shared/lib/drag-preview'

export function useGridRowReorder({ instanceKey, rows, config, canUpdate, disabled, onMove }: {
  instanceKey: string; rows: readonly Row[]; config: ViewConfig; canUpdate: boolean; disabled: boolean
  onMove?: (row: Row, target: Row, direction: 'before' | 'after') => Promise<boolean>
}) {
  const source = useRef<Row | null>(null), lock = useRef(false), generation = useRef(0)
  const preview = useRef<DragPreview | null>(null)
  const [target, setTarget] = useState<{ id: string; direction: 'before' | 'after' } | null>(null)
  const canMove = canUpdate && Boolean(onMove) && !config.filters.length && !config.sorts.length && !config.groupFieldId
  const latest = useRef({ instanceKey, canMove, disabled }); latest.current = { instanceKey, canMove, disabled }
  const valid = () => latest.current.instanceKey === instanceKey && latest.current.canMove && !latest.current.disabled
  const clear = () => { preview.current?.finish(); preview.current = null; source.current = null; setTarget(null) }
  useEffect(() => () => { preview.current?.finish() }, [])
  useEffect(() => { generation.current++; lock.current = false; clear() }, [instanceKey, canMove])
  const move = async (row: Row, anchor: Row, direction: 'before' | 'after') => {
    if (!valid() || lock.current || row.id === anchor.id || !rows.some(item => item.id === row.id) || !rows.some(item => item.id === anchor.id)) return
    const ticket = generation.current; lock.current = true; clear()
    try { await onMove?.(row, anchor, direction) }
    catch { /* Workflow owns persistence errors; no local reorder is applied. */ }
    finally { if (ticket === generation.current) lock.current = false }
  }
  return {
    canMove, target,
    start: (row: Row, event: DragEvent<HTMLButtonElement>) => {
      if (!valid() || lock.current) { event.preventDefault(); return }
      source.current = row; event.stopPropagation(); event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('application/x-pm-bitable-row', row.id)
      const element = event.currentTarget.closest<HTMLElement>('tr')
      // Keep virtualized table rows in their original hit-test positions. Displacing
      // TRs disables their pointer events and can make native drops miss the tbody.
      if (element) preview.current = createDragPreview(element, event.nativeEvent, () => Array.from(element.parentElement?.querySelectorAll<HTMLElement>('tr[data-record-id]') ?? []))
    },
    over: (row: Row | undefined, event: DragEvent<HTMLElement>) => {
      if (!valid() || !source.current || lock.current) return
      event.preventDefault(); event.stopPropagation(); event.dataTransfer.dropEffect = 'move'
      const bounds = event.currentTarget.getBoundingClientRect()
      const hit = preview.current?.update(event.clientX, event.clientY)
      const id = hit?.element.dataset.recordId ?? row?.id
      setTarget(id ? { id, direction: hit?.direction ?? (event.clientY < bounds.top + bounds.height / 2 ? 'before' : 'after') } : null)
    },
    drop: (row: Row | undefined, event: DragEvent<HTMLElement>) => {
      const moving = source.current
      if (!valid() || !moving || event.dataTransfer.getData('application/x-pm-bitable-row') !== moving.id) return
      event.preventDefault(); event.stopPropagation()
      const bounds = event.currentTarget.getBoundingClientRect()
      const hit = preview.current?.update(event.clientX, event.clientY)
      const anchor = rows.find(item => item.id === hit?.element.dataset.recordId) ?? row
      if (!anchor || anchor.id === moving.id) { clear(); return }
      void move(moving, anchor, hit?.direction ?? (event.clientY < bounds.top + bounds.height / 2 ? 'before' : 'after'))
    },
    end: clear,
    keyboard: (row: Row, direction: -1 | 1) => { const anchor = rows[rows.findIndex(item => item.id === row.id) + direction]; if (anchor) void move(row, anchor, direction === -1 ? 'before' : 'after') },
  }
}
