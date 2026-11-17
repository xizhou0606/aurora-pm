import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  CollectionState,
  ReadyRefresh,
  ResourceState,
  WorkItem,
  WorkItemUserOption,
  WorkItemsPage,
} from './types'

export { isValidEstimatedMinutes } from './mutation'
export { normalizeWorkItemStatus } from '@/shared/model'

export {
  DEFAULT_PAGE_SIZE,
  PAGE_SIZES,
  emptyScope,
  parseWorkItemsRoute,
  requestScope,
  resetPage,
  scopeSummary,
  workItemsPath,
} from './route-utils'

export {
  buildBoardGroups,
  buildBoardLanes,
  buildListGroups,
  enumLabel,
  formatDate,
  formatDateTime,
  formatEstimatedMinutes,
  isSafePMDetailPath,
  isWorkItemOverdue,
  hasWorkItemsBoardScope,
  isValidWorkItemsBoardScope,
  isValidWorkItemsListScope,
  kindLabel,
  normalizeWorkItemPriority,
  normalizeWorkItemsBoardScope,
  priorityLabel,
  resolveWorkItemAssignee,
  safeAttachmentURL,
  statusLabel,
  workItemAssigneeDirectoryIds,
  workflowStatusOptions,
} from './presenters'

export const WORK_ITEM_PRIORITY_OPTIONS = Object.entries(WORK_ITEMS_COPY.enums.priorities)
  .map(([value, label]) => ({ value, label }))

export function workItemUserSelectOption(option: WorkItemUserOption) {
  return {
    id: option.id,
    name: option.label,
    username: option.username,
    avatarUrl: option.avatarUrl,
  }
}

export function workItemDetailTriggerId(row: Pick<WorkItem, 'id'>): string {
  return `work-items-detail-trigger-${row.id}`
}

export function workItemRequirementPath(
  item: {
    projectSetId?: number
    projectId?: number
    requirementId?: number
  },
): string | undefined {
  if (!item.projectSetId || !item.projectId || !item.requirementId) return undefined
  return `/workspace/projects/${item.projectSetId}/items/${item.projectId}?tab=requirements`
}

export function readyRefresh(fetching: boolean, failed: boolean): ReadyRefresh {
  if (fetching) return 'refreshing'
  if (failed) return 'stale_error'
  return 'idle'
}

export function resolveCollectionState(input: {
  data?: WorkItemsPage
  pending: boolean
  fetching: boolean
  failed: boolean
  retry: () => void
}): CollectionState<WorkItem> {
  if (!input.data) {
    if (input.pending) return { status: 'loading' }
    return { status: 'error', retry: input.retry }
  }
  if (!input.data.items.length) {
    return {
      status: 'empty',
      refresh: readyRefresh(input.fetching, input.failed),
      retry: input.retry,
    }
  }
  return {
    status: 'ready',
    rows: input.data.items,
    total: input.data.total,
    refresh: readyRefresh(input.fetching, input.failed),
    truncated: input.data.truncated === true,
  }
}

export function resolveResourceState<T>(input: {
  data?: T
  pending: boolean
  fetching: boolean
  failed: boolean
  retry: () => void
  enabled: boolean
}): ResourceState<T> {
  if (!input.enabled) return { status: 'idle' }
  if (input.data === undefined) {
    if (input.pending) return { status: 'loading' }
    return { status: 'error', retry: input.retry }
  }
  return {
    status: 'ready',
    data: input.data,
    refresh: readyRefresh(input.fetching, input.failed),
  }
}
