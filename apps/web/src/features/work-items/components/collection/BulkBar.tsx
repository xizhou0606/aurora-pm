import { BulkActionBar, BulkResult, Button, ConfirmDialog } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useState } from 'react'
import type { MutationState, WorkItem } from '../../model/types'
import { statusLabel } from '../../model/utils'
import { WorkItemsSelect } from './SelectField'

export function BulkBar({
  rows,
  mutation,
  transitions,
  canDelete,
  onUpdateStatus,
  onDelete,
  onSelectAll,
  onInvertSelection,
  deleteDescription = WORK_ITEMS_COPY.bulk.confirmDeleteDescription,
}: {
  rows: readonly WorkItem[]
  mutation: MutationState
  transitions: readonly string[]
  canDelete: boolean
  onUpdateStatus: (status: string) => Promise<boolean>
  onDelete: () => Promise<boolean>
  onSelectAll: () => void
  onInvertSelection: () => void
  deleteDescription?: string
}) {
  const [status, setStatus] = useState('')
  const [deleteOpen, setDeleteOpen] = useState(false)
  if (!rows.length) return null
  const pending = mutation.status === 'pending'
  const runUpdate = async () => {
    try {
      await onUpdateStatus(status)
    } catch {
      // Mutation state owns the retryable error.
    }
  }
  const runDelete = async () => {
    try {
      if (await onDelete()) setDeleteOpen(false)
    } catch {
      // Keep confirmation open for retry.
    }
  }
  return (
    <BulkActionBar
      data-testid="work-items-bulk"
      selectedLabel={WORK_ITEMS_COPY.bulk.selected(rows.length)}
      selectAllLabel={WORK_ITEMS_COPY.bulk.selectAll}
      invertSelectionLabel={WORK_ITEMS_COPY.bulk.invertSelection}
      onSelectAll={onSelectAll}
      onInvertSelection={onInvertSelection}
      controls={
        transitions.length ? (
          <div className="w-52 min-w-0 max-[720px]:flex-1">
            <WorkItemsSelect
              id="work-items-bulk-status"
              label={WORK_ITEMS_COPY.bulk.status}
              value={status}
              disabled={pending}
              placeholder={WORK_ITEMS_COPY.bulk.selectStatus}
              options={transitions.map((value) => ({ value, label: statusLabel(value) }))}
              onValueChange={setStatus}
            />
          </div>
        ) : null
      }
      actions={<>
        {transitions.length ? (
          <Button type="button" size="sm" disabled={pending || !status} onClick={() => void runUpdate()}>
            {WORK_ITEMS_COPY.bulk.update}
          </Button>
        ) : null}
        {canDelete ? (
          <Button type="button" size="sm" variant="destructive" disabled={pending} onClick={() => setDeleteOpen(true)}>
            {WORK_ITEMS_COPY.bulk.delete}
          </Button>
        ) : null}
      </>}
    >
      {!transitions.length && !canDelete ? (
        <span className="basis-full text-xs text-muted-foreground" role="status" aria-live="polite">{WORK_ITEMS_COPY.bulk.unavailable}</span>
      ) : null}
      {mutation.status === 'error' ? <span className="basis-full" role="alert">{mutation.message}</span> : null}
      {mutation.status === 'partial' ? (
        <div className="basis-full" role="status" aria-live="polite">
          <span>{WORK_ITEMS_COPY.mutation.partial}</span>
          <BulkResult succeeded={mutation.succeeded} failures={mutation.failures} />
        </div>
      ) : null}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onConfirm={() => void runDelete()}
        title={WORK_ITEMS_COPY.bulk.confirmDeleteTitle}
        description={deleteDescription}
        cancelLabel={WORK_ITEMS_COPY.mutation.cancel}
        confirmLabel={WORK_ITEMS_COPY.bulk.delete}
        pending={pending}
      />
    </BulkActionBar>
  )
}
