import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  normalizeWorkItemStatus,
  workItemStatusLabel,
} from '@/shared/model'
import { dateInputValue } from '@/shared/lib/date-boundary'
import type {
  WorkItem,
  WorkItemStatusOption,
  WorkItemUserOption,
  WorkItemWorkflowCatalog,
} from './types'

export {
  buildBoardGroups,
  buildBoardLanes,
  buildListGroups,
  groupWorkItems,
  hasWorkItemsBoardScope,
  isValidWorkItemsBoardScope,
  isValidWorkItemsListScope,
  normalizeWorkItemsBoardScope,
  visibleTreeRows,
} from './board-presenters'

export function resolveWorkItemAssignee(
  item: Pick<WorkItem, 'assigneeId' | 'assigneeName'>,
  users: readonly WorkItemUserOption[],
): { name: string; user?: WorkItemUserOption } {
  const user = item.assigneeId
    ? users.find((candidate) => candidate.id === item.assigneeId)
    : undefined
  return {
    name: user?.label || item.assigneeName?.trim() || WORK_ITEMS_COPY.list.unassigned,
    user,
  }
}

export function workItemAssigneeDirectoryIds(
  items: readonly Pick<WorkItem, 'assigneeId'>[],
  selectedIds: readonly number[] = [],
): number[] {
  const ids = new Set<number>()
  for (const item of items) {
    if (Number.isInteger(item.assigneeId) && Number(item.assigneeId) > 0) {
      ids.add(Number(item.assigneeId))
    }
  }
  for (const id of selectedIds) {
    if (Number.isInteger(id) && id > 0) ids.add(id)
  }
  return [...ids].sort((left, right) => left - right)
}

export function normalizeWorkItemPriority(value: unknown): string {
  const key = String(value || '').trim().toLowerCase()
  return key === 'medium' ? 'normal' : key
}

export function enumLabel(value: string | undefined, labels: Readonly<Record<string, string>>): string {
  const key = String(value || '').trim().toLowerCase()
  return labels[key] || value || WORK_ITEMS_COPY.enums.fallback
}

export function statusLabel(value: string | undefined): string {
  return workItemStatusLabel(value, WORK_ITEMS_COPY.enums.fallback)
}

export function priorityLabel(value: string | undefined): string {
  return enumLabel(normalizeWorkItemPriority(value), WORK_ITEMS_COPY.enums.priorities)
}

export function kindLabel(value: string | undefined): string {
  return enumLabel(value, WORK_ITEMS_COPY.enums.kinds)
}

export function workflowStatusOptions(
  item: WorkItem,
  workflow: WorkItemWorkflowCatalog,
): WorkItemStatusOption[] {
  const current = normalizeWorkItemStatus(item.status)
  const labels = new Map(workflow.statuses.map((status) => [status.key, status.label]))
  const permittedEdges = new Set(workflow.transitions
    .filter((transition) => transition.from === current)
    .map((transition) => transition.to))
  const seen = new Set<string>()
  const options: WorkItemStatusOption[] = []
  const append = (value: string, requireCatalog: boolean) => {
    const key = normalizeWorkItemStatus(value)
    if (!key || seen.has(key) || (requireCatalog && !labels.has(key))) return
    seen.add(key)
    options.push({ value: key, label: labels.get(key) || statusLabel(key) })
  }
  append(current, false)
  for (const transition of item.capabilities.allowedTransitions) {
    const target = normalizeWorkItemStatus(transition)
    if (workflow.strict && !permittedEdges.has(target)) continue
    append(target, true)
  }
  return options
}

export function formatDate(value: string | undefined): string {
  if (!value) return WORK_ITEMS_COPY.list.noDueDate
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return WORK_ITEMS_COPY.list.noDueDate
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
    .format(parsed)
}

export function formatEstimatedMinutes(value: number | undefined): string {
  if (!value || value < 0) return WORK_ITEMS_COPY.list.noEstimatedTime
  const hours = Number((value / 60).toFixed(1))
  return WORK_ITEMS_COPY.list.estimatedHours(hours)
}

export function isWorkItemOverdue(
  item: Pick<WorkItem, 'dueAt' | 'status'>,
  now = Date.now(),
): boolean {
  if (!item.dueAt) return false
  const status = normalizeWorkItemStatus(item.status)
  if (status === 'completed' || status === 'cancelled') return false
  const current = new Date(now)
  if (Number.isNaN(current.getTime())) return false
  const dueDate = dateInputValue(item.dueAt)
  const currentDate = dateInputValue(current.toISOString())
  return Boolean(dueDate && currentDate && dueDate < currentDate)
}

export function formatDateTime(value: string | undefined): string {
  if (!value) return ''
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(parsed)
}

export function formatActivityDateTime(value: string | undefined): string {
  if (!value) return WORK_ITEMS_COPY.enums.fallback
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  const parts = Object.fromEntries(new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(parsed).map((part) => [part.type, part.value]))
  return `${parts.year}/${parts.month}/${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`
}

export function safeAttachmentURL(value: string | undefined): string | undefined {
  if (!value) return undefined
  try {
    const url = new URL(value, window.location.origin)
    return ['http:', 'https:'].includes(url.protocol) ? url.href : undefined
  } catch {
    return undefined
  }
}

export function isSafePMDetailPath(value: string | undefined): value is string {
  return Boolean(value && /^\/workspace\/projects\/[1-9]\d*\/items\/[1-9]\d*\?tab=tasks&task_requirement_id=[1-9]\d*&task_id=[1-9]\d*$/.test(value))
}
