import { useEffect, useMemo, useState } from 'react'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { displayCellValue } from '../../model/fields'
import type { Field, Row } from '../../model/types'

export type GridDisplayRow = { kind: 'record'; index: number; id: string } | { kind: 'group'; id: string; name: string; count: number; expanded: boolean }

export function useGridRows(rows: readonly Row[], groupField: Field | undefined, instanceKey: string) {
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())
  useEffect(() => { setCollapsed(new Set()) }, [instanceKey, groupField?.id])
  const groups = useMemo(() => {
    if (!groupField) return []
    const result = new Map<string, { id: string; name: string; records: GridDisplayRow[] }>()
    rows.forEach((row, index) => {
      const value = row.values[groupField.id] ?? null
      const id = JSON.stringify(value)
      if (!result.has(id)) result.set(id, { id, name: displayCellValue(groupField, value) || text.ungrouped, records: [] })
      result.get(id)!.records.push({ kind: 'record', id: row.id, index })
    })
    return [...result.values()]
  }, [groupField, rows])
  const items = useMemo<GridDisplayRow[]>(() => groupField ? groups.flatMap((group) => [
    { kind: 'group' as const, id: group.id, name: group.name, count: group.records.length, expanded: !collapsed.has(group.id) },
    ...(!collapsed.has(group.id) ? group.records : []),
  ]) : rows.map((row, index) => ({ kind: 'record', id: row.id, index })), [collapsed, groupField, groups, rows])
  return { items, toggle: (id: string) => setCollapsed((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next }),
    reveal: (recordId: string) => { const group = groups.find((entry) => entry.records.some((row) => row.id === recordId)); if (group && collapsed.has(group.id)) setCollapsed((current) => new Set([...current].filter((id) => id !== group.id))) } }
}
