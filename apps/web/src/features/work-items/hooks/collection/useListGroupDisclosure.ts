import { useCallback, useEffect, useMemo, useState } from 'react'

import { visibleTreeRows } from '../../model/presenters'
import type {
  WorkItemListGroup,
  WorkItemListPresentationRow,
} from '../../model/types'

export function useListGroupDisclosure(
  groups: readonly WorkItemListGroup[],
  expandedTreeRowIds: ReadonlySet<string>,
) {
  const [collapsedGroupIds, setCollapsedGroupIds] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    const available = new Set(groups.filter((group) => group.label).map((group) => group.id))
    setCollapsedGroupIds((current) => {
      const next = new Set([...current].filter((id) => available.has(id)))
      const unchanged = next.size === current.size && [...next].every((id) => current.has(id))
      return unchanged ? current : next
    })
  }, [groups])

  const toggleGroup = useCallback((groupId: string) => {
    setCollapsedGroupIds((current) => {
      const next = new Set(current)
      if (next.has(groupId)) next.delete(groupId)
      else next.add(groupId)
      return next
    })
  }, [])

  const rows = useMemo<WorkItemListPresentationRow[]>(() => groups.flatMap((group) => {
    const grouped = Boolean(group.label)
    const expanded = !collapsedGroupIds.has(group.id)
    const header: WorkItemListPresentationRow[] = grouped
      ? [{
          kind: 'group',
          rowId: `group:${group.id}`,
          groupId: group.id,
          label: group.label,
          count: group.rows.filter((row) => !row.contextOnly).length,
          expanded,
        }]
      : []
    if (grouped && !expanded) return header
    return [
      ...header,
      ...visibleTreeRows(group.rows, expandedTreeRowIds).map((item): WorkItemListPresentationRow => ({
        kind: 'item',
        rowId: item.rowId,
        item,
      })),
    ]
  }), [collapsedGroupIds, expandedTreeRowIds, groups])

  return { rows, toggleGroup }
}
