import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { endOfLocalDay, startOfLocalDay } from '@/shared/lib/date-boundary'

import type { WorkItemUserOption } from './collection'
import { buildRequirementOptions, selectedRequirementOption } from './create'

export type QuickTaskInheritedField = 'priority' | 'taskClientId' | 'assigneeId' | 'startsAt' | 'dueAt'

export type QuickTaskChildDraft = {
  clientId: string
  title: string
  priority: string
  taskClientId?: number
  assigneeId?: number
  startsAt: string
  dueAt: string
  estimatedHours: string
  touched: Readonly<Record<QuickTaskInheritedField, boolean>>
}

export type QuickTaskDraft = {
  clientId: string
  title: string
  priority: string
  taskClientId?: number
  requirementScope: PMScopeSelection
  assigneeId?: number
  startsAt: string
  dueAt: string
  estimatedHours: string
  children: QuickTaskChildDraft[]
}

export type QuickTaskField = Exclude<keyof QuickTaskDraft, 'clientId' | 'children'>
export type QuickTaskChildField = Exclude<keyof QuickTaskChildDraft, 'clientId' | 'touched'>
export type QuickTaskDraftError = 'title' | 'requirement' | 'priority' | 'assignee' | 'estimated-hours' | 'date-range'
export type QuickTaskDraftErrors = Readonly<Record<string, QuickTaskDraftError>>
export type QuickTaskCreateState =
  | { status: 'closed' }
  | {
      status: 'editing'
      tasks: QuickTaskDraft[]
      errors: QuickTaskDraftErrors
      submit: 'idle' | 'pending' | 'error'
    }

export type QuickTaskBatchPayload = {
  tasks: Array<{
    client_id: string
    title: string
    requirement_id: number
    priority: string
    task_client_id?: number
    assignee_id: number
    starts_at?: string
    due_at?: string
    estimated_minutes?: number
    children: Array<{
      client_id: string
      title: string
      priority: string
      task_client_id?: number | null
      assignee_id: number
      starts_at?: string | null
      due_at?: string | null
      estimated_minutes?: number
    }>
  }>
}

export type QuickTaskBatchResult = {
  created: Array<{
    clientId: string
    id: number
    taskKey?: string
    children: Array<{ clientId: string; id: number; taskKey?: string }>
  }>
  total: number
}

const PRIORITIES = new Set(['low', 'normal', 'high', 'urgent'])
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/
const EMPTY_SCOPE: PMScopeSelection = { projectSetIds: [], projectIds: [], requirementIds: [] }

function estimatedMinutes(value: string): number | undefined {
  if (!value.trim()) return undefined
  const hours = Number(value)
  if (!Number.isFinite(hours) || hours < 0 || hours > 16_666) return Number.NaN
  return Math.round(hours * 60)
}

function selectedScope(selected: PMScopeSelection, scope: PMScopeOptions): PMScopeSelection {
  const option = selected.requirementIds.length === 1
    ? buildRequirementOptions(scope).find(({ value }) => Number(value) === selected.requirementIds[0])
    : undefined
  return option?.selection ?? EMPTY_SCOPE
}

export function createQuickTaskDraft(input: {
  clientId: string
  selected: PMScopeSelection
  scope: PMScopeOptions
  assigneeId?: number
}): QuickTaskDraft {
  return {
    clientId: input.clientId,
    title: '',
    priority: 'normal',
    requirementScope: selectedScope(input.selected, input.scope),
    assigneeId: input.assigneeId,
    startsAt: '',
    dueAt: '',
    estimatedHours: '',
    children: [],
  }
}

export function applyQuickTaskFilledFields(
  tasks: readonly QuickTaskDraft[],
  sourceClientId: string,
  scope: PMScopeOptions,
): QuickTaskDraft[] {
  const source = tasks.find(({ clientId }) => clientId === sourceClientId)
  if (!source) return [...tasks]
  const requirementScope = selectedScope(source.requirementScope, scope)
  const shared = {
    ...(source.title.trim() ? { title: source.title } : {}),
    ...(source.priority.trim() ? { priority: source.priority } : {}),
    ...(source.taskClientId ? { taskClientId: source.taskClientId } : {}),
    ...(source.assigneeId ? { assigneeId: source.assigneeId } : {}),
    ...(source.startsAt ? { startsAt: source.startsAt } : {}),
    ...(source.dueAt ? { dueAt: source.dueAt } : {}),
    ...(source.estimatedHours.trim() ? { estimatedHours: source.estimatedHours } : {}),
  }
  return tasks.map((task) => ({
    ...task,
    ...shared,
    ...(requirementScope.requirementIds.length === 1 ? {
      requirementScope: {
        projectSetIds: [...requirementScope.projectSetIds],
        projectIds: [...requirementScope.projectIds],
        requirementIds: [...requirementScope.requirementIds],
      },
    } : {}),
    children: task.children.map((child) => ({
      ...child,
      ...shared,
      touched: {
        ...child.touched,
        ...(source.priority.trim() ? { priority: true } : {}),
        ...(source.taskClientId ? { taskClientId: true } : {}),
        ...(source.assigneeId ? { assigneeId: true } : {}),
        ...(source.startsAt ? { startsAt: true } : {}),
        ...(source.dueAt ? { dueAt: true } : {}),
      },
    })),
  }))
}

export function createQuickTaskChildDraft(
  clientId: string,
  parent: QuickTaskDraft,
): QuickTaskChildDraft {
  return {
    clientId,
    title: '',
    priority: parent.priority,
    taskClientId: parent.taskClientId,
    assigneeId: parent.assigneeId,
    startsAt: parent.startsAt,
    dueAt: parent.dueAt,
    estimatedHours: '',
    touched: { priority: false, taskClientId: false, assigneeId: false, startsAt: false, dueAt: false },
  }
}

export function updateQuickTaskParent(
  parent: QuickTaskDraft,
  field: QuickTaskField,
  value: QuickTaskDraft[QuickTaskField],
): QuickTaskDraft {
  if (field === 'requirementScope' || field === 'title' || field === 'estimatedHours') {
    return { ...parent, [field]: value }
  }
  return {
    ...parent,
    [field]: value,
    children: parent.children.map((child) => child.touched[field]
      ? child
      : { ...child, [field]: value }),
  }
}

export function updateQuickTaskChild(
  child: QuickTaskChildDraft,
  field: QuickTaskChildField,
  value: QuickTaskChildDraft[QuickTaskChildField],
): QuickTaskChildDraft {
  const inherited = field === 'priority' || field === 'taskClientId' || field === 'assigneeId'
    || field === 'startsAt' || field === 'dueAt'
  return {
    ...child,
    [field]: value,
    ...(inherited ? { touched: { ...child.touched, [field]: true } } : {}),
  }
}

function validateShared(input: {
  id: string
  title: string
  priority: string
  assigneeId?: number
  startsAt: string
  dueAt: string
  estimatedHours: string
  users: readonly WorkItemUserOption[]
}, errors: Record<string, QuickTaskDraftError>) {
  if (!input.title.trim() || input.title.trim().length > 220) errors[`${input.id}:title`] = 'title'
  if (!PRIORITIES.has(input.priority)) errors[`${input.id}:priority`] = 'priority'
  if (!input.assigneeId || !input.users.some(({ id }) => id === input.assigneeId)) {
    errors[`${input.id}:assigneeId`] = 'assignee'
  }
  const minutes = estimatedMinutes(input.estimatedHours)
  if (Number.isNaN(minutes)) errors[`${input.id}:estimatedHours`] = 'estimated-hours'
  if ((input.startsAt && !DATE_PATTERN.test(input.startsAt))
    || (input.dueAt && !DATE_PATTERN.test(input.dueAt))
    || (input.startsAt && input.dueAt && input.startsAt > input.dueAt)) {
    errors[`${input.id}:dateRange`] = 'date-range'
  }
}

export function validateQuickTaskDrafts(input: {
  tasks: readonly QuickTaskDraft[]
  scope: PMScopeOptions
  users: readonly WorkItemUserOption[]
}): QuickTaskDraftErrors {
  const errors: Record<string, QuickTaskDraftError> = {}
  const requirementOptions = buildRequirementOptions(input.scope)
  input.tasks.forEach((task) => {
    validateShared({ ...task, id: task.clientId, users: input.users }, errors)
    if (!selectedRequirementOption(task.requirementScope, requirementOptions)) {
      errors[`${task.clientId}:requirementScope`] = 'requirement'
    }
    task.children.forEach((child) => validateShared({
      ...child, id: child.clientId, users: input.users,
    }, errors))
  })
  return errors
}

function optionalFields(input: {
  startsAt: string
  dueAt: string
  estimatedHours: string
}) {
  const minutes = estimatedMinutes(input.estimatedHours)
  return {
    ...(input.startsAt ? { starts_at: startOfLocalDay(input.startsAt) } : {}),
    ...(input.dueAt ? { due_at: endOfLocalDay(input.dueAt) } : {}),
    ...(minutes !== undefined && !Number.isNaN(minutes) ? { estimated_minutes: minutes } : {}),
  }
}

function childDateFields(child: QuickTaskChildDraft) {
  return {
    ...(child.startsAt
      ? { starts_at: startOfLocalDay(child.startsAt) }
      : child.touched.startsAt ? { starts_at: null } : {}),
    ...(child.dueAt
      ? { due_at: endOfLocalDay(child.dueAt) }
      : child.touched.dueAt ? { due_at: null } : {}),
  }
}

export function quickTaskBatchPayload(tasks: readonly QuickTaskDraft[]): QuickTaskBatchPayload {
  return {
    tasks: tasks.map((task) => ({
      client_id: task.clientId,
      title: task.title.trim(),
      requirement_id: task.requirementScope.requirementIds[0] || 0,
      priority: task.priority,
      ...(task.taskClientId ? { task_client_id: task.taskClientId } : {}),
      assignee_id: task.assigneeId || 0,
      ...optionalFields(task),
      children: task.children.map((child) => ({
        client_id: child.clientId,
        title: child.title.trim(),
        priority: child.priority,
        ...(child.taskClientId
          ? { task_client_id: child.taskClientId }
          : child.touched.taskClientId ? { task_client_id: null } : {}),
        assignee_id: child.assigneeId || 0,
        ...optionalFields({ ...child, startsAt: '', dueAt: '' }),
        ...childDateFields(child),
      })),
    })),
  }
}

export function quickTaskServerErrors(error: unknown): QuickTaskDraftErrors {
  const source = error && typeof error === 'object' ? error as Record<string, unknown> : {}
  const details = source.details && typeof source.details === 'object'
    ? source.details as Record<string, unknown>
    : source
  const fields = Array.isArray(details.fields) ? details.fields : []
  const result: Record<string, QuickTaskDraftError> = {}
  fields.forEach((entry) => {
    if (!entry || typeof entry !== 'object') return
    const field = entry as Record<string, unknown>
    const clientId = typeof field.client_id === 'string' ? field.client_id : ''
    const fieldName = typeof field.field === 'string' ? field.field : ''
    const mapped: Record<string, QuickTaskDraftError> = {
      title: 'title', requirement_id: 'requirement', priority: 'priority',
      assignee_id: 'assignee', estimated_minutes: 'estimated-hours',
      starts_at: 'date-range', due_at: 'date-range',
    }
    if (clientId && mapped[fieldName]) result[`${clientId}:${fieldName}`] = mapped[fieldName]
  })
  return result
}
