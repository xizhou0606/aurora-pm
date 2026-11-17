import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'

import {
  Button,
  DataTable,
  type DataTableState,
} from '@/shared/ui'
import {
  applyTableColumnPreference,
  type TableColumnOption,
  type TableColumnPreference,
} from '@/shared/model/table-column-preferences'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useListGroupDisclosure } from '../../hooks'
import type {
  CollectionState,
  WorkItemListGroup,
  WorkItemListPresentationRow,
  MutationState,
  UserOptionsState,
  WorkItemWorkflowCatalog,
  WorkItemClientOption,
  WorkItem,
  WorkItemDetailPatch,
} from '../../model/types'
import { useListColumns } from './ListColumns'

const WORK_ITEMS_TABLE_LAYOUT = 'min-w-max table-fixed'

export type WorkItemsListColumnId =
  | 'selection' | 'task_name' | 'task_number' | 'status' | 'assignee'
  | 'priority' | 'task_client_id' | 'estimated_minutes' | 'starts_at' | 'due_at' | 'actions'

export const WORK_ITEMS_LIST_COLUMN_OPTIONS: readonly TableColumnOption<WorkItemsListColumnId>[] = [
  { id: 'selection', label: WORK_ITEMS_COPY.list.selection, required: true },
  { id: 'task_name', label: WORK_ITEMS_COPY.list.taskName, required: true },
  { id: 'task_number', label: WORK_ITEMS_COPY.list.taskNumber },
  { id: 'status', label: WORK_ITEMS_COPY.list.status },
  { id: 'assignee', label: WORK_ITEMS_COPY.list.assignee },
  { id: 'priority', label: WORK_ITEMS_COPY.list.priority },
  { id: 'task_client_id', label: WORK_ITEMS_COPY.list.client, defaultHidden: true },
  { id: 'estimated_minutes', label: WORK_ITEMS_COPY.list.estimatedTime },
  { id: 'starts_at', label: WORK_ITEMS_COPY.list.startsAt },
  { id: 'due_at', label: WORK_ITEMS_COPY.list.due },
  { id: 'actions', label: WORK_ITEMS_COPY.list.actions, required: true },
]

type ListProps = {
  state: CollectionState<WorkItem>
  groups: readonly WorkItemListGroup[]
  selectedRowIds: readonly string[]
  onToggleSelected: (row: WorkItem) => void
  expandedRowIds: ReadonlySet<string>
  onToggleExpanded: (row: WorkItem, rows: readonly WorkItem[]) => void
  onOpen?: (row: WorkItem, trigger: HTMLElement, mode?: 'view' | 'edit') => void
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
  onDelete: (row: WorkItem, options?: { cascade?: boolean }) => Promise<boolean>
  updateState: MutationState
  deleteState: MutationState
  users: UserOptionsState
  workflow?: WorkItemWorkflowCatalog
  clients?: readonly WorkItemClientOption[]
  columnPreference?: TableColumnPreference<WorkItemsListColumnId>
}

function tableState(
  state: CollectionState<WorkItem>,
): DataTableState<WorkItemListPresentationRow> {
  if (state.status === 'loading') {
    return { status: 'loading', label: WORK_ITEMS_COPY.list.loading, skeletonRows: 8 }
  }
  if (state.status === 'error') {
    return {
      status: 'error',
      title: WORK_ITEMS_COPY.list.error,
      onRetry: state.retry,
      retryLabel: WORK_ITEMS_COPY.actions.retry,
    }
  }
  if (state.status === 'empty') {
    return {
      status: 'empty',
      title: WORK_ITEMS_COPY.list.empty
    }
  }
  return { status: 'ready', rows: [], refresh: state.refresh }
}

export function List({
  state,
  groups,
  selectedRowIds,
  onToggleSelected,
  expandedRowIds,
  onToggleExpanded,
  onOpen,
  onUpdate,
  onDelete,
  updateState,
  deleteState,
  users,
  workflow,
  clients = [],
  columnPreference = {
    order: WORK_ITEMS_LIST_COLUMN_OPTIONS.map(({ id }) => id),
    hidden: [],
  },
}: ListProps) {
  const { rows: presentationRows, toggleGroup } = useListGroupDisclosure(groups, expandedRowIds)
  const allColumns = useListColumns({
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
  })
  const columns = useMemo(
    () => applyTableColumnPreference(allColumns, columnPreference),
    [allColumns, columnPreference],
  )

  if (state.status !== 'ready') {
    return (
      <section className="grid min-w-0 gap-3 overflow-x-auto" aria-label={WORK_ITEMS_COPY.list.label}>
        {state.status === 'empty' && state.refresh === 'refreshing' ? (
          <p role="status">{WORK_ITEMS_COPY.page.updating}</p>
        ) : null}
        {state.status === 'empty' && state.refresh === 'stale_error' ? (
          <p role="alert">
            {WORK_ITEMS_COPY.page.staleError}{' '}
            <Button type="button" size="sm" variant="outline" onClick={state.retry}>
              {WORK_ITEMS_COPY.actions.retry}
            </Button>
          </p>
        ) : null}
        <DataTable
          columns={columns}
          state={tableState(state)}
          className="overflow-x-auto"
          tableClassName={WORK_ITEMS_TABLE_LAYOUT}
          scrollAreaClassName="max-h-[calc(100vh-100px)]"
          aria-label={WORK_ITEMS_COPY.list.label}
        />
      </section>
    )
  }

  return (
    <section className="grid min-w-0 gap-3 overflow-x-auto" aria-label={WORK_ITEMS_COPY.list.label}>
      {updateState.status === 'error' || deleteState.status === 'error' ? (
        <p role="alert">{WORK_ITEMS_COPY.mutation.failed}</p>
      ) : null}
      <DataTable
        columns={columns}
        state={{
          status: 'ready',
          rows: presentationRows,
          refresh: state.refresh,
          refreshing: state.refresh === 'refreshing',
          refreshingLabel: WORK_ITEMS_COPY.page.updating,
          staleErrorLabel: WORK_ITEMS_COPY.page.staleError,
        }}
        getRowId={(row) => row.rowId}
        // eslint-disable-next-line shadcn/require-static-classes -- 按行回调返回静态类名；规则无法窥入函数体
        getRowClassName={(row) => row.kind === 'group'
          ? 'bg-muted/60 font-medium hover:bg-muted/70'
          : undefined}
        className="overflow-x-auto"
        tableClassName={WORK_ITEMS_TABLE_LAYOUT}
        scrollAreaClassName="max-h-[calc(100vh-100px)]"
        aria-label={WORK_ITEMS_COPY.list.label}
      />
    </section>
  )
}
