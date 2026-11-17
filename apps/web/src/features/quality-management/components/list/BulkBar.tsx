import { useState } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { BulkActionBar, Button, ConfirmDialog } from '@/shared/ui'
import type { QualityMutationState } from '../../model/types'

export function BulkBar({
  count,
  mutation,
  assignmentMutation,
  onAssign,
  onDelete,
  onSelectAll,
  onInvertSelection,
  canDelete,
  canAssign,
}: {
  count: number
  mutation: QualityMutationState
  assignmentMutation: QualityMutationState
  onAssign: () => void
  onDelete: () => Promise<void>
  onSelectAll: () => void
  onInvertSelection: () => void
  canDelete: boolean
  canAssign: boolean
}) {
  const [deleteOpen, setDeleteOpen] = useState(false)
  if (!count || (!canAssign && !canDelete)) return null
  const pending = mutation.status === 'pending'
  const runDelete = async () => {
    try {
      await onDelete()
      setDeleteOpen(false)
    } catch {
      // Keep the confirmation open so the mutation feedback can be retried.
    }
  }
  return (
    <BulkActionBar
      data-testid="quality-bulk"
      selectedLabel={QUALITY_COPY.bulk.selected(count)}
      selectAllLabel={QUALITY_COPY.bulk.selectAll}
      invertSelectionLabel={QUALITY_COPY.bulk.invertSelection}
      onSelectAll={onSelectAll}
      onInvertSelection={onInvertSelection}
      actions={<>
      {canAssign ? (
        <Button
          type="button"
          size="sm"
          disabled={pending || assignmentMutation.status === 'pending'}
          onClick={onAssign}
        >
          {QUALITY_COPY.bulk.assign}
        </Button>
      ) : null}
      {canDelete ? (
        <Button
          type="button"
          size="sm"
          variant="destructive"
          disabled={pending || assignmentMutation.status === 'pending'}
          onClick={() => setDeleteOpen(true)}
        >
          {QUALITY_COPY.bulk.delete}
        </Button>
      ) : null}
      </>}
    >
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onConfirm={() => void runDelete()}
        title={QUALITY_COPY.bulk.confirmDeleteTitle}
        description={QUALITY_COPY.bulk.confirmDeleteDescription(count)}
        cancelLabel={QUALITY_COPY.confirm.cancel}
        confirmLabel={QUALITY_COPY.bulk.delete}
        pending={pending}
        error={mutation.status === 'error' ? QUALITY_COPY.mutation.deleteFailed : undefined}
      />
    </BulkActionBar>
  )
}
