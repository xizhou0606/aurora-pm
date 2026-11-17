import type {
  CreateProjectWorkTaskPayload,
  PatchProjectWorkTaskPayload,
  PMContentPriority,
  ProjectWorkTask,
  ProjectWorkTaskStatus,
} from '../api'
import {
  addDraftDateErrors,
  dateTimePayload,
  draftDateValue,
  hasValidationErrors,
  isOneOf,
  positiveDraftId,
} from './form-utils'
import type { PayloadResult } from './write-forms-types'
import type {
  TaskCreateDraft,
  TaskDraft,
  TaskUpdateDraft,
  TaskValidationErrors,
} from './content-forms-types'
import { CONTENT_PRIORITIES, TASK_STATUSES } from './content-forms-types'

function optionalUserId(value: unknown): number | null {
  return value == null ? null : positiveDraftId(value)
}

function normalizedTaskEstimate(value: string): number | null {
  const hours = Number(value)
  return Number.isFinite(hours) && hours >= 0 && hours <= 1000
    ? Math.round(hours * 60)
    : null
}

export function taskDraftFromCreate(input: {
  projectId: number
  planUnitId: number
  requirementId: number
}): TaskCreateDraft {
  return {
    mode: 'create',
    ...input,
    title: '',
    description: '',
    assigneeId: null,
    priority: 'normal',
    startsAt: '',
    dueAt: '',
    estimatedHours: '0',
  }
}

export function taskDraftFromTask(task: ProjectWorkTask): TaskUpdateDraft {
  return {
    mode: 'update',
    projectId: task.project_set_id,
    planUnitId: task.project_id,
    requirementId: task.requirement_id,
    taskId: task.id,
    title: task.title || '',
    description: task.description || '',
    assigneeId: optionalUserId(task.assignee_id),
    priority: String(task.priority || 'normal'),
    startsAt: draftDateValue(task.starts_at, true),
    dueAt: draftDateValue(task.due_at, true),
    estimatedHours: String(Math.round((Number(task.estimated_minutes || 0) / 60) * 10) / 10),
    status: String(task.status || 'draft'),
  }
}

export function validateTaskDraft(draft: TaskDraft): TaskValidationErrors {
  const errors: TaskValidationErrors = {}
  if (!positiveDraftId(draft.projectId)) errors.projectId = 'INVALID_ID'
  if (!positiveDraftId(draft.planUnitId)) errors.planUnitId = 'INVALID_ID'
  if (!positiveDraftId(draft.requirementId)) errors.requirementId = 'INVALID_ID'
  if (draft.mode === 'update' && !positiveDraftId(draft.taskId)) errors.taskId = 'INVALID_ID'
  if (!draft.title.trim()) errors.title = 'REQUIRED'
  if (draft.assigneeId !== null && !positiveDraftId(draft.assigneeId)) errors.assigneeId = 'INVALID_ID'
  if (!isOneOf(draft.priority, CONTENT_PRIORITIES)) errors.priority = 'INVALID_ENUM'
  if (draft.mode === 'update' && !isOneOf(draft.status, TASK_STATUSES)) {
    errors.status = 'INVALID_ENUM'
  }
  addDraftDateErrors(errors, 'startsAt', draft.startsAt, 'dueAt', draft.dueAt)
  if (normalizedTaskEstimate(draft.estimatedHours) === null) {
    errors.estimatedHours = 'INVALID_NUMBER'
  }
  return errors
}

export function mapTaskCreateDraft(
  draft: TaskCreateDraft,
): PayloadResult<CreateProjectWorkTaskPayload, TaskValidationErrors> {
  const errors = validateTaskDraft(draft)
  if (hasValidationErrors(errors)) return { ok: false, errors }
  return {
    ok: true,
    value: {
      requirement_id: draft.requirementId,
      kind: 'task',
      title: draft.title.trim(),
      description: draft.description.trim(),
      assignee_id: draft.assigneeId,
      priority: draft.priority as PMContentPriority,
      starts_at: dateTimePayload(draft.startsAt),
      due_at: dateTimePayload(draft.dueAt),
      estimated_minutes: normalizedTaskEstimate(draft.estimatedHours) as number,
    },
  }
}

export function buildTaskPatch(
  draft: TaskUpdateDraft,
  source: ProjectWorkTask,
): PayloadResult<PatchProjectWorkTaskPayload, TaskValidationErrors> {
  const errors = validateTaskDraft(draft)
  if (hasValidationErrors(errors)) return { ok: false, errors }
  const patch: PatchProjectWorkTaskPayload = {}
  if (draft.title.trim() !== source.title.trim()) patch.title = draft.title.trim()
  if (draft.description.trim() !== String(source.description || '').trim()) patch.description = draft.description.trim()
  if (draft.assigneeId !== optionalUserId(source.assignee_id)) patch.assignee_id = draft.assigneeId
  if (draft.priority !== String(source.priority || 'normal')) patch.priority = draft.priority as PMContentPriority
  if (draft.status !== String(source.status || 'draft')) patch.status = draft.status as ProjectWorkTaskStatus
  const startsAt = dateTimePayload(draft.startsAt)
  const dueAt = dateTimePayload(draft.dueAt)
  if (startsAt !== (source.starts_at ? new Date(source.starts_at).toISOString() : null)) patch.starts_at = startsAt
  if (dueAt !== (source.due_at ? new Date(source.due_at).toISOString() : null)) patch.due_at = dueAt
  const estimate = normalizedTaskEstimate(draft.estimatedHours) as number
  if (estimate !== Number(source.estimated_minutes || 0)) patch.estimated_minutes = estimate
  return { ok: true, value: patch }
}