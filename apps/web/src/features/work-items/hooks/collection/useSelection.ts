import { useCallback, useEffect, useMemo, useState } from 'react'
import type { WorkItem, WorkItemListGroup, WorkItemTreeRow } from '../../model/types'

function canSelect(row: WorkItem): boolean {
  return row.capabilities.canUpdate
    || row.capabilities.canDelete
    || row.capabilities.allowedTransitions.length > 0
}

export function useWorkItemSelection(
  rows: readonly WorkItem[] | undefined,
  listGroups: readonly WorkItemListGroup[],
  selectionEnabled = true,
) {
  const [selectedRowIds, setSelectedRowIds] = useState<readonly string[]>([])
  const [collapsedTreeRowIds, setCollapsedTreeRowIds] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    if (!selectionEnabled) {
      setSelectedRowIds((current) => current.length ? [] : current)
      return
    }
    if (!rows) {
      setSelectedRowIds([])
      return
    }
    const available = new Set(rows.map((row) => row.rowId))
    setSelectedRowIds((current) => {
      const next = current.filter((id) => available.has(id))
      return next.length === current.length ? current : next
    })
  }, [rows, selectionEnabled])

  const selectedRows = useMemo(
    () => rows?.filter((row) => selectedRowIds.includes(row.rowId)) || [],
    [rows, selectedRowIds],
  )
  const selectableRowIds = useMemo(
    () => rows?.filter(canSelect).map((row) => row.rowId) || [],
    [rows],
  )
  const commonTransitions = selectedRows.length
    ? selectedRows[0].capabilities.allowedTransitions.filter((status) => (
      selectedRows.every((row) => row.capabilities.canUpdate
        && row.capabilities.allowedTransitions.includes(status))
    ))
    : []
  const canBatchDelete = selectedRows.length > 0
    && selectedRows.every((row) => row.capabilities.canDelete)
  const expandedTreeRowIds = useMemo(() => new Set(listGroups.flatMap((group) => (
    group.rows.filter((row) => row.hasChildren && !collapsedTreeRowIds.has(row.rowId)).map((row) => row.rowId)
  ))), [collapsedTreeRowIds, listGroups])

  const toggleSelected = useCallback((row: WorkItem) => {
    if (!canSelect(row)) return
    setSelectedRowIds((current) => current.includes(row.rowId)
      ? current.filter((id) => id !== row.rowId)
      : [...current, row.rowId])
  }, [])
  const selectAll = useCallback(() => setSelectedRowIds(selectableRowIds), [selectableRowIds])
  const invertSelection = useCallback(() => {
    setSelectedRowIds((current) => {
      const selected = new Set(current)
      return selectableRowIds.filter((id) => !selected.has(id))
    })
  }, [selectableRowIds])

  const toggleTreeExpanded = useCallback((row: WorkItemTreeRow, treeRows: readonly WorkItemTreeRow[]) => {
    if (!row.hasChildren) return
    const wasExpanded = !collapsedTreeRowIds.has(row.rowId)
    setCollapsedTreeRowIds((current) => {
      const collapsed = new Set(current)
      if (collapsed.has(row.rowId)) collapsed.delete(row.rowId)
      else collapsed.add(row.rowId)
      return collapsed
    })
    if (!wasExpanded) return
    const start = treeRows.findIndex((entry) => entry.rowId === row.rowId)
    if (start < 0) return
    const hidden = new Set<string>()
    for (let index = start + 1; index < treeRows.length; index += 1) {
      if (treeRows[index].depth <= row.depth) break
      hidden.add(treeRows[index].rowId)
    }
    setSelectedRowIds((current) => current.filter((id) => !hidden.has(id)))
  }, [collapsedTreeRowIds])

  return {
    selectedRowIds,
    selectedRows,
    commonTransitions,
    canBatchDelete,
    expandedTreeRowIds,
    toggleSelected,
    selectAll,
    invertSelection,
    toggleTreeExpanded,
    clearSelection: useCallback(() => setSelectedRowIds([]), []),
  }
}
