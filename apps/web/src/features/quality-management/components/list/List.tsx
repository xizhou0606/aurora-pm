import { useMemo } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { DataTable } from '@/shared/ui'
import { applyTableColumnPreference } from '@/shared/model/table-column-preferences'
import type { TableColumnOption, TableColumnPreference } from '@/shared/model/table-column-preferences'
import type {
  QualityBug,
  QualitySceneOptionCatalogs,
  QualityFilters,
  QualityListState,
  QualityUsersState,
} from '../../model/types'
import { QUALITY_LIST_COLUMN_OPTIONS } from './ListColumns'
import { useListColumns } from './ListColumns'
import type { QualityListColumnId } from './ListColumns'

export type { QualityListColumnId }
export { QUALITY_LIST_COLUMN_OPTIONS }

export function List({
  state,
  users,
  sceneOptions,
  onAssign,
  onOpen,
  onDelete,
  selectedIds,
  onToggleSelected,
  onOpenRequirement,
  onSort,
  onInlineSave,
  onStatusChange,
  canUpdateBug,
  canAssignBug,
  canSelectBug,
  canTransitionBug,
  canDeleteBug,
  columnPreference = {
    order: QUALITY_LIST_COLUMN_OPTIONS.map(({ id }) => id),
    hidden: ['review-solution'],
  },
}: {
  state: Exclude<QualityListState, { status: 'unavailable' }>
  users: QualityUsersState
  sceneOptions: QualitySceneOptionCatalogs
  onAssign: (bug: QualityBug) => void
  onOpen: (bug: QualityBug, mode?: 'view' | 'edit') => void
  onDelete: (bug: QualityBug) => void
  selectedIds: readonly number[]
  onToggleSelected: (bug: QualityBug) => void
  onOpenRequirement: (requirementId: number) => void
  onSort: (field: QualityFilters['sortBy']) => void
  onInlineSave: (bug: QualityBug, payload: Record<string, unknown>) => Promise<void>
  onStatusChange: (bug: QualityBug, status: string, resolutionReason?: string) => Promise<void>
  canUpdateBug: (bug: QualityBug) => boolean
  canAssignBug: (bug: QualityBug) => boolean
  canSelectBug: (bug: QualityBug) => boolean
  canTransitionBug: (bug: QualityBug) => boolean
  canDeleteBug: (bug: QualityBug) => boolean
  columnPreference?: TableColumnPreference<QualityListColumnId>
}) {
  const directory = useMemo(
    () => new Map(users.rows.map((user) => [user.id, user])),
    [users.rows],
  )

  const allColumns = useListColumns({
    directory,
    canAssignBug,
    canDeleteBug,
    canSelectBug,
    canTransitionBug,
    canUpdateBug,
    onAssign,
    onDelete,
    onInlineSave,
    onOpen,
    onOpenRequirement,
    onSort,
    onStatusChange,
    onToggleSelected,
    sceneOptions,
    selectedIds,
    users,
  })

  const columns = useMemo(
    () => applyTableColumnPreference(allColumns, columnPreference),
    [allColumns, columnPreference],
  )

  const tableState = state.status === 'loading'
    || (state.status === 'ready' && state.refresh === 'refreshing')
    ? { status: 'loading' as const, label: QUALITY_COPY.list.loading }
    : state.status === 'error'
      ? {
          status: 'error' as const,
          title: QUALITY_COPY.list.error,
          description: QUALITY_COPY.list.errorDescription,
          onRetry: state.retry,
        }
      : state.status === 'empty'
        ? {
            status: 'empty' as const,
            title: QUALITY_COPY.list.empty,
            description: QUALITY_COPY.list.emptyDescription,
          }
        : {
            status: 'ready' as const,
            rows: state.rows,
            refresh: state.refresh,
          }

  return (
    <DataTable
      aria-label={QUALITY_COPY.list.label}
      columns={columns}
      state={tableState}
      getRowId={(row) => String(row.id)}
      tableClassName="min-w-max table-fixed"
      scrollAreaClassName="max-h-[calc(100vh-100px)]"
    />
  )
}
