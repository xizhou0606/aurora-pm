import type { WorkItem } from '../model/types'
import { dateInputValue } from '@/shared/lib/date-boundary'
import { normalizeWorkItemStatus } from '@/shared/model'
import { normalizeWorkItemPriority } from '../model/presenters'

type RecordValue = Record<string, unknown>

export function record(value: unknown): RecordValue {
  return value && typeof value === 'object' ? value as RecordValue : {}
}

export function array(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

export function optionalNumber(value: unknown): number | undefined {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

export function optionalString(value: unknown): string | undefined {
  const parsed = typeof value === 'string' ? value.trim() : ''
  return parsed || undefined
}

function optionalDate(value: unknown): string | undefined {
  const parsed = optionalString(value)
  return parsed && /^\d{4}-\d{2}-\d{2}(?:$|T)/.test(parsed)
    ? dateInputValue(parsed) || undefined
    : undefined
}

function optionalNonNegativeInteger(value: unknown): number | undefined {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : undefined
}

function stringArray(value: unknown): string[] {
  return array(value).flatMap((entry) => {
    const parsed = optionalString(entry)
    return parsed ? [parsed] : []
  })
}

function normalizedStatuses(value: unknown): string[] {
  const allowed = new Set(['draft', 'in_dev', 'pending_test', 'completed', 'cancelled'])
  return [...new Set(stringArray(value).map(normalizeWorkItemStatus))]
    .filter((status) => allowed.has(status))
}

export function normalizeWorkItem(value: unknown): WorkItem | null {
  const source = record(value)
  const id = optionalNumber(source.id)
  const title = optionalString(source.title)
  if (!id || !title) return null
  const statusTransitions = stringArray(source.allowed_status_transitions)
  const transitions = statusTransitions.length
    ? normalizedStatuses(statusTransitions)
    : normalizedStatuses(source.allowed_transitions)
  const contextOnly = source.context_only === true
  return {
    id,
    rowId: `task:${id}`,
    taskKey: optionalString(source.task_key),
    title,
    description: optionalString(source.description),
    kind: optionalString(source.kind),
    status: normalizeWorkItemStatus(optionalString(source.status)) || undefined,
    priority: normalizeWorkItemPriority(optionalString(source.priority)) || undefined,
    assigneeId: optionalNumber(source.assignee_id),
    assigneeName: optionalString(source.assignee_display_name)
      || optionalString(source.assignee_name)
      || optionalString(source.assignee_username),
    reporterId: optionalNumber(source.reporter_user_id),
    reporterName: optionalString(source.reporter_display_name)
      || optionalString(source.reporter_name)
      || optionalString(source.reporter_username),
    createdByUserId: optionalNumber(source.created_by_user_id),
    teamId: optionalNumber(source.team_id),
    projectSetId: optionalNumber(source.project_set_id),
    projectId: optionalNumber(source.project_id),
    requirementId: optionalNumber(source.requirement_id),
    taskClientId: optionalNumber(source.task_client_id),
    requirementTitle: optionalString(source.requirement_title),
    parentId: optionalNumber(source.parent_id),
    startsAt: optionalDate(source.starts_at),
    dueAt: optionalDate(source.due_at),
    estimatedMinutes: optionalNonNegativeInteger(source.estimated_minutes),
    createdAt: optionalString(source.created_at),
    updatedAt: optionalString(source.updated_at),
    externalSource: optionalString(source.source),
    contextOnly,
    watched: source.watched === true,
    capabilities: {
      canUpdate: !contextOnly,
      canDelete: !contextOnly,
      canComment: true,
      canAttach: true,
      canWatch: true,
      canClone: false,
      canLogTime: false,
      allowedTransitions: contextOnly ? [] : transitions,
    },
  }
}
