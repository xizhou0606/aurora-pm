import { useEffect, useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { ConfirmDialog, DataTable } from '@/shared/ui'
import type {
  UserOptionsState,
  MutationState,
  WorkItemChildResource,
  WorkItemRelated,
  WorkItemStatusOption,
} from '../../model/types'
import { BulkBar } from '../collection/BulkBar'
import { useChildListColumns } from './ChildListColumns'

export { statusOptions } from './ChildListColumns'

export function ChildList({
  resource,
  users,
  workflow,
}: {
  resource: WorkItemChildResource
  users: UserOptionsState
  workflow: readonly WorkItemStatusOption[]
}) {
  const [deleteRow, setDeleteRow] = useState<WorkItemRelated | null>(null)
  const [deletingId, setDeletingId] = useState<number>()
  const [selectedIds, setSelectedIds] = useState<readonly number[]>([])
  const [bulkMutation, setBulkMutation] = useState<MutationState>({ status: 'idle' })
  const pending = resource.mutation.status === 'pending'
  const rows = resource.state.status === 'ready' ? resource.state.data : []
  const selectedRows = rows.filter((row) => selectedIds.includes(row.id))
  const selectableRows = rows.filter((row) => row.capabilities.canUpdate
    || row.capabilities.canDelete || row.capabilities.allowedTransitions.length > 0)
  const commonTransitions = selectedRows.length
    ? selectedRows[0].capabilities.allowedTransitions.filter((status) => (
        selectedRows.every((row) => row.capabilities.allowedTransitions.includes(status))
      ))
    : []
  const canBatchDelete = selectedRows.length > 0
    && selectedRows.every((row) => row.capabilities.canDelete)

  useEffect(() => {
    const available = new Set(rows.map((row) => row.id))
    setSelectedIds((current) => {
      const next = current.filter((id) => available.has(id))
      return next.length === current.length ? current : next
    })
  }, [rows])

  const columns = useChildListColumns({
    resource,
    users,
    workflow,
    selectedIds,
    setSelectedIds,
    deletingId,
    pending,
    setDeleteRow,
  })

  const tableState = resource.state.status === 'idle' || resource.state.status === 'loading'
    ? { status: 'loading' as const, label: WORK_ITEMS_COPY.detail.children }
    : resource.state.status === 'error'
      ? {
          status: 'error' as const,
          description: WORK_ITEMS_COPY.detail.resourceError,
          onRetry: resource.state.retry,
          retryLabel: WORK_ITEMS_COPY.actions.retry,
        }
      : resource.state.data.length
        ? {
            status: 'ready' as const,
            rows: resource.state.data,
            refresh: resource.state.refresh,
            staleErrorLabel: WORK_ITEMS_COPY.detail.refreshWarning,
          }
        : { status: 'empty' as const, title: WORK_ITEMS_COPY.detail.noChildren }
  const remove = async () => {
    if (!deleteRow || deletingId) return
    setDeletingId(deleteRow.id)
    try {
      if (await resource.remove(deleteRow.id)) setDeleteRow(null)
    } finally {
      setDeletingId(undefined)
    }
  }
  const updateSelectedStatus = async (status: string) => {
    if (!selectedRows.length || !status) return false
    setBulkMutation({ status: 'pending', action: 'child-batch-update' })
    const results = await Promise.all(selectedRows.map((row) => resource.update(row, { status })))
    if (results.every(Boolean)) {
      setSelectedIds([])
      setBulkMutation({ status: 'success', action: 'child-batch-update' })
      return true
    }
    setBulkMutation({
      status: 'error', action: 'child-batch-update', message: WORK_ITEMS_COPY.mutation.failed,
    })
    return false
  }
  const deleteSelected = async () => {
    if (!canBatchDelete) return false
    setBulkMutation({ status: 'pending', action: 'child-batch-delete' })
    const results = await Promise.all(selectedRows.map((row) => resource.remove(row.id)))
    if (results.every(Boolean)) {
      setSelectedIds([])
      setBulkMutation({ status: 'success', action: 'child-batch-delete' })
      return true
    }
    setBulkMutation({
      status: 'error', action: 'child-batch-delete', message: WORK_ITEMS_COPY.mutation.failed,
    })
    return false
  }
  return (
    <>
      {resource.mutation.status === 'error' ? (
        <p role="alert" className="text-sm text-destructive">{WORK_ITEMS_COPY.mutation.failed}</p>
      ) : null}
      <DataTable
        aria-label={WORK_ITEMS_COPY.detail.children}
        columns={columns}
        state={tableState}
        className="overflow-x-auto"
        tableClassName="min-w-max table-fixed"
        getRowId={(row) => String(row.id)}
      />
      <BulkBar
        rows={selectedRows}
        mutation={bulkMutation}
        transitions={commonTransitions}
        canDelete={canBatchDelete}
        onUpdateStatus={updateSelectedStatus}
        onDelete={deleteSelected}
        onSelectAll={() => setSelectedIds(selectableRows.map((row) => row.id))}
        onInvertSelection={() => setSelectedIds((current) => {
          const selected = new Set(current)
          return selectableRows.filter((row) => !selected.has(row.id)).map((row) => row.id)
        })}
        deleteDescription={WORK_ITEMS_COPY.detail.confirmChildBatchDeleteDescription}
      />
      <ConfirmDialog
        open={Boolean(deleteRow)}
        onOpenChange={(open) => { if (!open) setDeleteRow(null) }}
        onConfirm={() => void remove()}
        title={WORK_ITEMS_COPY.detail.removeResourceTitle}
        description={WORK_ITEMS_COPY.detail.removeResourceDescription(deleteRow?.title || '')}
        cancelLabel={WORK_ITEMS_COPY.mutation.cancel}
        confirmLabel={WORK_ITEMS_COPY.mutation.delete}
        pending={Boolean(deletingId)}
      />
    </>
  )
}
