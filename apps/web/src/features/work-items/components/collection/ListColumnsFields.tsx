import type { ColumnDef } from '@tanstack/react-table'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemPriorityTone, workItemStatusTone } from '@/shared/model'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemListPresentationRow,
  MutationState,
  UserOptionsState,
  WorkItemWorkflowCatalog,
} from '../../model/types'
import {
  WORK_ITEM_PRIORITY_OPTIONS,
  workflowStatusOptions,
} from '../../model/utils'
import { renderStatusCell, renderAssigneeCell, renderPriorityCell, renderEstimatedCell, renderStartsAtCell, renderDueAtCell, itemRow } from './ListColumnCells'

export function buildFieldColumns({
  updateState,
  users,
  workflow,
  onUpdate,
}: {
  updateState: MutationState
  users: UserOptionsState
  workflow?: WorkItemWorkflowCatalog
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}): ColumnDef<WorkItemListPresentationRow>[] {
  return [
    {
      id: 'status', size: 128, header: WORK_ITEMS_COPY.list.status,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item) return null
        return renderStatusCell({ item, workflow, updateState, onUpdate })
      },
    },
    {
      id: 'assignee', size: 144, header: WORK_ITEMS_COPY.list.assignee,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item) return null
        return renderAssigneeCell({ item, users, updateState, onUpdate })
      },
    },
    {
      id: 'priority', size: 112, header: WORK_ITEMS_COPY.list.priority,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item) return null
        return renderPriorityCell({ item, updateState, onUpdate })
      },
    },
    {
      id: 'estimated_minutes', size: 112, header: WORK_ITEMS_COPY.list.estimatedTime,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item) return null
        return renderEstimatedCell({ item, updateState, onUpdate })
      },
    },
    {
      id: 'starts_at', size: 160, header: WORK_ITEMS_COPY.list.startsAt,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item) return null
        return renderStartsAtCell({ item, updateState, onUpdate })
      },
    },
    {
      id: 'due_at', size: 160, header: WORK_ITEMS_COPY.list.due,
      cell: ({ row }) => {
        const item = itemRow(row.original)
        if (!item) return null
        return renderDueAtCell({ item, updateState, onUpdate })
      },
    },
  ]
}