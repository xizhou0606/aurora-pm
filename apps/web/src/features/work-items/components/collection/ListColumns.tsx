import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemListGroup,
  WorkItemListPresentationRow,
  WorkItemTreeRow,
  MutationState,
  UserOptionsState,
  WorkItemWorkflowCatalog,
  WorkItemClientOption,
} from '../../model/types'
import { itemRow, renderSelectionCell, renderTaskNumberCell, renderTaskNameCell, renderActionsCell } from './ListColumnCells'
import { buildFieldColumns } from './ListColumnsFields'

export function useListColumns({
  groups,
  expandedRowIds,
  toggleGroup,
  onToggleExpanded,
  onToggleSelected,
  onOpen,
  onUpdate,
  onDelete,
  updateState,
  deleteState,
  users,
  workflow,
  clients,
  selectedRowIds,
}: {
  groups: readonly WorkItemListGroup[]
  expandedRowIds: ReadonlySet<string>
  toggleGroup: (groupId: string) => void
  onToggleExpanded: (row: WorkItemTreeRow, rows: readonly WorkItemTreeRow[]) => void
  onToggleSelected: (row: WorkItem) => void
  onOpen?: (row: WorkItem, trigger: HTMLElement, mode?: 'view' | 'edit') => void
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
  onDelete: (row: WorkItem, options?: { cascade?: boolean }) => Promise<boolean>
  updateState: MutationState
  deleteState: MutationState
  users: UserOptionsState
  workflow?: WorkItemWorkflowCatalog
  clients: readonly WorkItemClientOption[]
  selectedRowIds: readonly string[]
}): ColumnDef<WorkItemListPresentationRow>[] {
  return useMemo<ColumnDef<WorkItemListPresentationRow>[]>(() => [
    {
      id: 'selection',
      size: 48,
      header: WORK_ITEMS_COPY.list.selection,
      cell: ({ row }) => renderSelectionCell({ row: itemRow(row.original), selectedRowIds, onToggleSelected }),
    },
    {
      id: 'task_number',
      size: 240,
      header: WORK_ITEMS_COPY.list.taskNumber,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        return item ? renderTaskNumberCell({ item, onOpen }) : null
      },
    },
    {
      id: 'task_name',
      size: 320,
      header: WORK_ITEMS_COPY.list.taskName,
      cell: ({ row }) => renderTaskNameCell({
        presentation: row.original, expandedRowIds, groups, users, updateState,
        onToggleExpanded, onUpdate, toggleGroup,
      }),
    },
    ...buildFieldColumns({ updateState, users, workflow, onUpdate }),
    {
      id: 'task_client_id',
      size: 140,
      header: WORK_ITEMS_COPY.list.client,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item?.taskClientId) return item ? WORK_ITEMS_COPY.list.noClient : null
        return clients.find(({ id }) => id === item.taskClientId)?.name || WORK_ITEMS_COPY.list.noClient
      },
    },
    {
      id: 'actions',
      size: 120,
      header: WORK_ITEMS_COPY.list.actions,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        return item ? renderActionsCell({ item, updateState, deleteState, onOpen, onDelete }) : null
      },
    },
  ], [clients, deleteState.status, expandedRowIds, groups, onDelete, onOpen, onToggleExpanded, onToggleSelected, onUpdate, selectedRowIds, toggleGroup, updateState, users, workflow])
}
