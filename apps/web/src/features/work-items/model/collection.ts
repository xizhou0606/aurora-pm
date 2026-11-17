import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'

import type { WorkItem } from './item'

export type WorkItemsView = 'board' | 'list'
export type WorkItemsLayout = 'table' | 'gantt'
export type WorkItemsGroupBy = 'none' | 'requirement_id' | 'assignee_id' | 'priority'
export type WorkItemsSort = 'created_at_desc' | 'created_at_asc' | 'updated_at_desc' | 'updated_at_asc'
export type WorkItemsScope = PMScopeSelection

export type WorkItemsRouteState = {
  scope: WorkItemsScope
  view: WorkItemsView
  layout: WorkItemsLayout
  search: string
  overdue: boolean
  assigneeIds: readonly number[]
  statuses: readonly string[]
  priorities: readonly string[]
  taskClientIds: readonly number[]
  filterKinds?: readonly string[]
  groupBy: WorkItemsGroupBy
  sort: WorkItemsSort
  page: number
  pageSize: 10 | 20 | 50 | 100
  detailId?: number
  mode: 'view' | 'edit'
  from?: 'inbox'
  invalidLegacyScope: boolean
  needsRouteNormalization: boolean
}

export type WorkItemsPage = {
  items: WorkItem[]
  total: number
  truncated?: boolean
}

export type ReadyRefresh = 'idle' | 'refreshing' | 'stale_error'

export type CollectionState<T> =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | {
      status: 'empty'
      refresh: ReadyRefresh
      retry: () => void
    }
  | {
      status: 'ready'
      rows: T[]
      total: number
      refresh: ReadyRefresh
      truncated: boolean
    }

export type ScopeOptionsState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; data: PMScopeOptions }

export type WorkItemUserOption = {
  id: number
  label: string
  username?: string
  avatarUrl?: string
}

// The workspace user directory loads once and filters locally inside the
// selects; no server-side search pagination on this state.
export type UserOptionsState =
  | { status: 'hidden' }
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'ready'; data: readonly WorkItemUserOption[] }

export type WorkItemGroup = { id: string; label: string; rows: WorkItem[] }
export type WorkItemTreeRow = WorkItem & { depth: number; hasChildren: boolean; treeGroupId: string }
export type WorkItemListGroup = { id: string; label: string; rows: WorkItemTreeRow[] }
export type WorkItemListPresentationRow =
  | {
      kind: 'group'
      rowId: string
      groupId: string
      label: string
      count: number
      expanded: boolean
    }
  | { kind: 'item'; rowId: string; item: WorkItemTreeRow }
export type WorkItemBoardLane = { id: string; label: string; rows: WorkItem[] }
export type WorkItemBoardGroup = {
  id: string
  label: string
  lanes: WorkItemBoardLane[]
}
export type WorkItemBoardTransitionState =
  | { status: 'idle' }
  | { status: 'dragging'; rowId: string; targetStatus?: string; targetLaneId?: string }
  | { status: 'pending'; rowId: string; targetStatus: string }
  | {
      status: 'error'
      rowId: string
      targetStatus: string
    }

export type WorkItemBoardTransition = {
  state: WorkItemBoardTransitionState
  canDrag: (row: WorkItem) => boolean
  canDrop: (status: string) => boolean
  start: (row: WorkItem) => boolean
  over: (status: string, laneId?: string) => void
  leave: (status: string, laneId?: string) => void
  drop: (status: string) => void
  end: () => void
  retry: () => void
  dismiss: () => void
  moveByKeyboard: (row: WorkItem, direction: -1 | 1) => void
}
export type WorkItemWorkflowStatus = { key: string; label: string; sortOrder: number }
export type WorkItemWorkflowCatalog = {
  statuses: readonly WorkItemWorkflowStatus[]
  transitions: readonly { from: string; to: string }[]
  strict: boolean
}
export type WorkItemStatusOption = { value: string; label: string }
