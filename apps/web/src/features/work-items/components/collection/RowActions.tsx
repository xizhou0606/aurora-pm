import { useState } from 'react'
import { ConfirmDialog, TableActionButton } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemTreeRow } from '../../model/types'

export function RowActions({
  row,
  action,
  onEdit,
  onDelete,
}: {
  row: WorkItemTreeRow
  action: { status: 'idle' } | { status: 'pending' }
  onEdit?: (row: WorkItemTreeRow, trigger: HTMLElement) => void
  onDelete: (row: WorkItemTreeRow, options?: { cascade?: boolean }) => Promise<boolean>
}) {
  const pending = action.status === 'pending'
  const [deleteOpen, setDeleteOpen] = useState(false)
  if ((!row.capabilities.canUpdate || !onEdit) && !row.capabilities.canDelete) {
    return null
  }
  const remove = async () => {
    try {
      if (await onDelete(row, { cascade: row.hasChildren })) setDeleteOpen(false)
    } catch {
      // Keep the confirmation open so the user can retry.
    }
  }
  return (
    <div className="flex min-w-max flex-nowrap items-center justify-center gap-1">
      {row.capabilities.canUpdate && onEdit ? (
        <TableActionButton
          id={`work-items-edit-${row.rowId.replace(':', '-')}`}
          label={WORK_ITEMS_COPY.mutation.edit}
          text={WORK_ITEMS_COPY.mutation.edit}
          disabled={pending}
          onClick={(event) => onEdit(row, event.currentTarget)}
        />
      ) : null}
      {row.capabilities.canDelete ? (
        <TableActionButton
          label={WORK_ITEMS_COPY.mutation.delete}
          text={WORK_ITEMS_COPY.mutation.delete}
          disabled={pending}
          onClick={() => setDeleteOpen(true)}
        />
      ) : null}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onConfirm={() => void remove()}
        title={WORK_ITEMS_COPY.mutation.confirmDeleteTitle}
        description={row.hasChildren
          ? WORK_ITEMS_COPY.detail.confirmCascadeDeleteDescription(row.title)
          : WORK_ITEMS_COPY.mutation.confirmDeleteDescription(row.title)}
        cancelLabel={WORK_ITEMS_COPY.mutation.cancel}
        confirmLabel={WORK_ITEMS_COPY.mutation.delete}
        pending={pending}
      />
    </div>
  )
}
