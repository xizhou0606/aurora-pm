import type { WorkItem, WorkItemTreeRow, MutationState } from '../../../model/types'
import { RowActions } from '../RowActions'

export function renderActionsCell({
  item,
  updateState,
  deleteState,
  onOpen,
  onDelete,
}: {
  item: WorkItemTreeRow
  updateState: MutationState
  deleteState: MutationState
  onOpen?: (row: WorkItem, trigger: HTMLElement, mode?: 'view' | 'edit') => void
  onDelete: (row: WorkItem, options?: { cascade?: boolean }) => Promise<boolean>
}) {
  return (
    <RowActions
      row={item}
      action={updateState.status === 'pending' || deleteState.status === 'pending'
        ? { status: 'pending' }
        : { status: 'idle' }}
      onEdit={onOpen ? (item, trigger) => onOpen(item, trigger, 'edit') : undefined}
      onDelete={onDelete}
    />
  )
}