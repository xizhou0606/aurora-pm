import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'

import type { TableColumnOption } from '@/shared/model/table-column-preferences'
import { QUALITY_COPY } from '@/shared/copy'
import type {
  QualityBug,
  QualitySceneOptionCatalogs,
  QualityFilters,
  QualityUsersState,
} from '../../model/types'
import { dataColumns } from './ListDataColumns'
import { linkColumns } from './ListLinkColumns'

export type QualityListColumnId =
  | 'selection' | 'title' | 'key' | 'severity' | 'status' | 'review-solution'
  | 'requirement' | 'owner' | 'reporter' | 'environment' | 'created'
  | 'status-wait' | 'actions'

export const QUALITY_LIST_COLUMN_OPTIONS: readonly TableColumnOption<QualityListColumnId>[] = [
  { id: 'selection', label: QUALITY_COPY.bulk.selection, required: true },
  { id: 'title', label: QUALITY_COPY.list.title, required: true },
  { id: 'key', label: QUALITY_COPY.list.key },
  { id: 'severity', label: QUALITY_COPY.list.severity },
  { id: 'status', label: QUALITY_COPY.list.status },
  { id: 'review-solution', label: QUALITY_COPY.list.solution, defaultHidden: true },
  { id: 'requirement', label: QUALITY_COPY.list.requirement },
  { id: 'owner', label: QUALITY_COPY.list.owner },
  { id: 'reporter', label: QUALITY_COPY.list.reporter },
  { id: 'status-wait', label: QUALITY_COPY.list.statusWait },
  { id: 'created', label: QUALITY_COPY.list.createdAt },
  { id: 'environment', label: QUALITY_COPY.list.environment },
  { id: 'actions', label: QUALITY_COPY.list.actions, required: true },
]

export type ListColumnsDeps = {
  directory: Map<number, { id: number; name: string; username?: string; avatarUrl?: string }>
  canAssignBug: (bug: QualityBug) => boolean
  canDeleteBug: (bug: QualityBug) => boolean
  canSelectBug: (bug: QualityBug) => boolean
  canTransitionBug: (bug: QualityBug) => boolean
  canUpdateBug: (bug: QualityBug) => boolean
  onAssign: (bug: QualityBug) => void
  onDelete: (bug: QualityBug) => void
  onInlineSave: (bug: QualityBug, payload: Record<string, unknown>) => Promise<void>
  onOpen: (bug: QualityBug, mode?: 'view' | 'edit') => void
  onOpenRequirement: (requirementId: number) => void
  onSort: (field: QualityFilters['sortBy']) => void
  onStatusChange: (bug: QualityBug, status: string) => Promise<void>
  onToggleSelected: (bug: QualityBug) => void
  sceneOptions: QualitySceneOptionCatalogs
  selectedIds: readonly number[]
  users: QualityUsersState
}

export function useListColumns(deps: ListColumnsDeps): ColumnDef<QualityBug>[] {
  const {
    canSelectBug,
    onOpen,
    onOpenRequirement,
    onToggleSelected,
    selectedIds,
  } = deps

  const links = linkColumns({ canSelectBug, onOpen, onOpenRequirement, onToggleSelected, selectedIds })
  const data = dataColumns(deps)

  return useMemo(() => [...links, ...data], [links, data])
}
