import { useCallback, useEffect, useMemo, useState } from 'react'

import type { QualityBug } from '../model/types'

export function useQualitySelection(
  rows: readonly QualityBug[] | undefined,
  isSelectable: (row: QualityBug) => boolean,
) {
  const [selectedIds, setSelectedIds] = useState<readonly number[]>([])

  useEffect(() => {
    if (!rows) {
      setSelectedIds([])
      return
    }
    const available = new Set(rows.filter(isSelectable).map((row) => row.id))
    setSelectedIds((current) => {
      const next = current.filter((id) => available.has(id))
      return next.length === current.length ? current : next
    })
  }, [isSelectable, rows])

  const selectableIds = useMemo(
    () => rows?.filter(isSelectable).map((row) => row.id) || [],
    [isSelectable, rows],
  )
  const selectedRows = useMemo(
    () => rows?.filter((row) => selectedIds.includes(row.id)) || [],
    [rows, selectedIds],
  )
  const toggleSelected = useCallback((row: QualityBug) => {
    if (!isSelectable(row)) return
    setSelectedIds((current) => current.includes(row.id)
      ? current.filter((id) => id !== row.id)
      : [...current, row.id])
  }, [isSelectable])
  const selectAll = useCallback(() => setSelectedIds(selectableIds), [selectableIds])
  const invertSelection = useCallback(() => {
    setSelectedIds((current) => {
      const selected = new Set(current)
      return selectableIds.filter((id) => !selected.has(id))
    })
  }, [selectableIds])
  const clearSelection = useCallback(() => setSelectedIds([]), [])

  return {
    selectedIds,
    selectedRows,
    toggleSelected,
    selectAll,
    invertSelection,
    clearSelection,
  }
}
