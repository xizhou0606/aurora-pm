import type { WorkItemStatusOption, WorkItemUserOption } from './collection'
import type { WorkItemChildCreatePayload, WorkItemChildDraft } from './resource'

const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/
const PRIORITIES = new Set(['low', 'normal', 'high', 'urgent'])

export type WorkItemChildDraftError =
  | 'title'
  | 'reporter'
  | 'assignee'
  | 'priority'
  | 'status'
  | 'estimated-hours'
  | 'date-range'

function estimatedMinutes(value: string): number | undefined {
  if (!value.trim()) return undefined
  const hours = Number(value)
  if (!Number.isFinite(hours) || hours < 0 || hours > 16_666) return Number.NaN
  return Math.round(hours * 60)
}

export function createEmptyChildDraft(clientId: string): WorkItemChildDraft {
  return {
    clientId,
    title: '',
    priority: '',
    startsAt: '',
    dueAt: '',
    estimatedHours: '',
    status: '',
  }
}

export function validateChildDraft(input: {
  draft: WorkItemChildDraft
  users: readonly WorkItemUserOption[]
  statuses: readonly WorkItemStatusOption[]
}): WorkItemChildDraftError | undefined {
  const { draft } = input
  if (!draft.title.trim() || draft.title.trim().length > 200) return 'title'
  if (draft.reporterUserId && !input.users.some(({ id }) => id === draft.reporterUserId)) {
    return 'reporter'
  }
  if (draft.assigneeId && !input.users.some(({ id }) => id === draft.assigneeId)) {
    return 'assignee'
  }
  if (draft.priority && !PRIORITIES.has(draft.priority)) return 'priority'
  if (draft.status && !input.statuses.some(({ value }) => value === draft.status)) return 'status'
  const minutes = estimatedMinutes(draft.estimatedHours)
  if (Number.isNaN(minutes)) return 'estimated-hours'
  if ((draft.startsAt && !DATE_PATTERN.test(draft.startsAt))
    || (draft.dueAt && !DATE_PATTERN.test(draft.dueAt))
    || (draft.startsAt && draft.dueAt && draft.startsAt > draft.dueAt)) {
    return 'date-range'
  }
  return undefined
}

export function childCreatePayload(draft: WorkItemChildDraft): WorkItemChildCreatePayload {
  const minutes = estimatedMinutes(draft.estimatedHours)
  return {
    clientId: draft.clientId,
    title: draft.title.trim(),
    ...(draft.reporterUserId ? { reporterUserId: draft.reporterUserId } : {}),
    ...(draft.assigneeId ? { assigneeId: draft.assigneeId } : {}),
    ...(draft.priority ? { priority: draft.priority } : {}),
    ...(draft.startsAt ? { startsAt: draft.startsAt } : {}),
    ...(draft.dueAt ? { dueAt: draft.dueAt } : {}),
    ...(minutes !== undefined && !Number.isNaN(minutes) ? { estimatedMinutes: minutes } : {}),
    ...(draft.status ? { status: draft.status } : {}),
  }
}
