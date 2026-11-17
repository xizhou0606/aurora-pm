import type { WorkItem, WorkItemTreeRow } from '../../../model/types'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { Checkbox } from '@/shared/ui'

export function canSelect(row: WorkItem): boolean {
  return (
    row.capabilities.canUpdate
    || row.capabilities.canDelete
    || row.capabilities.allowedTransitions.length > 0
  )
}

export function itemRow(row: { kind: string; item?: WorkItemTreeRow }): WorkItemTreeRow | undefined {
  return row.kind === 'item' ? row.item : undefined
}

export function renderSelectionCell({
  row,
  selectedRowIds,
  onToggleSelected,
}: {
  row: WorkItemTreeRow | undefined
  selectedRowIds: readonly string[]
  onToggleSelected: (row: WorkItem) => void
}) {
  return row ? (
    <Checkbox
      checked={selectedRowIds.includes(row.rowId)}
      disabled={!canSelect(row)}
      aria-label={WORK_ITEMS_COPY.list.selectRow(row.title || WORK_ITEMS_COPY.list.noTitle)}
      onCheckedChange={() => onToggleSelected(row)}
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => event.stopPropagation()}
    />
  ) : null
}
