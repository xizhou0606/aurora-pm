import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { endOfLocalDay, startOfLocalDay } from '@/shared/lib/date-boundary'

import type { WorkItemUserOption, WorkItemWorkflowStatus } from './collection'
import type { WorkItemClientOption } from './client-option'

export type WorkItemCreateDraft = {
  requirementScope: PMScopeSelection
  title: string
  description: string
  status: string
  priority: string
  taskClientId?: number
  assigneeId?: number
  estimatedHours: string
  startsAt: string
  dueAt: string
  attachments: File[]
}

export type WorkItemCreatePayload = {
  requirement_id: number
  kind: 'task'
  title: string
  description: string
  status: string
  priority: string
  task_client_id?: number
  reporter_user_id: number
  assignee_id?: number
  estimated_minutes?: number
  starts_at?: string
  due_at?: string
}

export type WorkItemCreateValidation =
  | 'workspace'
  | 'requirement'
  | 'title'
  | 'status'
  | 'priority'
  | 'client'
  | 'assignee'
  | 'estimated-hours'
  | 'date-range'

export type WorkItemCreateState =
  | { status: 'closed' }
  | {
      status: 'editing'
      draft: WorkItemCreateDraft
      submit: 'idle' | 'pending' | 'error'
      validation?: WorkItemCreateValidation
    }

export type WorkItemRequirementOption = {
  value: string
  label: string
  selection: PMScopeSelection
}

const VALID_PRIORITIES = new Set(['low', 'normal', 'high', 'urgent'])
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

function positiveNumber(value: string): number | undefined {
  if (!value.trim()) return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : Number.NaN
}

export function buildRequirementOptions(scope: PMScopeOptions): WorkItemRequirementOption[] {
  const projectSets = new Map(scope.projectSets.map((item) => [item.id, item]))
  const projects = new Map(scope.projects.map((item) => [item.id, item]))
  return scope.requirements.flatMap((requirement) => {
    const project = requirement.parentId ? projects.get(requirement.parentId) : undefined
    const projectSet = project?.parentId ? projectSets.get(project.parentId) : undefined
    if (!project || !projectSet) return []
    return [{
      value: String(requirement.id),
      label: [projectSet.label, project.label, requirement.label].join(' / '),
      selection: {
        projectSetIds: [projectSet.id],
        projectIds: [project.id],
        requirementIds: [requirement.id],
      },
    }]
  })
}

export function selectedRequirementOption(
  selection: PMScopeSelection,
  options: readonly WorkItemRequirementOption[],
): WorkItemRequirementOption | undefined {
  if (
    selection.projectSetIds.length !== 1
    || selection.projectIds.length !== 1
    || selection.requirementIds.length !== 1
  ) return undefined
  return options.find((option) => (
    option.selection.projectSetIds[0] === selection.projectSetIds[0]
    && option.selection.projectIds[0] === selection.projectIds[0]
    && option.selection.requirementIds[0] === selection.requirementIds[0]
  ))
}

export function initialCreateDraft(
  selected: PMScopeSelection,
  scope: PMScopeOptions,
  defaults: { assigneeId?: number; status?: string } = {},
): WorkItemCreateDraft {
  const requirements = buildRequirementOptions(scope)
  const selectedRequirement = selected.requirementIds.length === 1
    ? requirements.find((option) => Number(option.value) === selected.requirementIds[0])
    : undefined
  return {
    requirementScope: selectedRequirement?.selection ?? {
      projectSetIds: [], projectIds: [], requirementIds: [],
    },
    title: '',
    description: '',
    status: defaults.status || 'draft',
    priority: 'normal',
    assigneeId: defaults.assigneeId,
    estimatedHours: '',
    startsAt: '',
    dueAt: '',
    attachments: [],
  }
}

export function validateCreateDraft(input: {
  workspaceId: number
  draft: WorkItemCreateDraft
  scope: PMScopeOptions
  users: readonly WorkItemUserOption[]
  clientOptions?: readonly WorkItemClientOption[]
  statuses: readonly WorkItemWorkflowStatus[]
}): WorkItemCreateValidation | undefined {
  const { draft } = input
  if (!Number.isSafeInteger(input.workspaceId) || input.workspaceId <= 0) return 'workspace'
  if (!selectedRequirementOption(draft.requirementScope, buildRequirementOptions(input.scope))) {
    return 'requirement'
  }
  if (!draft.title.trim() || draft.title.trim().length > 256) return 'title'
  if (!draft.status || !input.statuses.some((status) => status.key === draft.status)) return 'status'
  if (!draft.priority || !VALID_PRIORITIES.has(draft.priority)) return 'priority'
  if (draft.taskClientId && !(input.clientOptions || []).some((option) => option.id === draft.taskClientId)) return 'client'
  if (draft.assigneeId && !input.users.some((user) => user.id === draft.assigneeId)) return 'assignee'
  const estimatedHours = positiveNumber(draft.estimatedHours)
  if (Number.isNaN(estimatedHours) || (estimatedHours !== undefined && estimatedHours > 16_666)) {
    return 'estimated-hours'
  }
  if (
    (draft.startsAt && !DATE_PATTERN.test(draft.startsAt))
    || (draft.dueAt && !DATE_PATTERN.test(draft.dueAt))
    || (draft.startsAt && draft.dueAt && draft.startsAt > draft.dueAt)
  ) return 'date-range'
  return undefined
}

export function createPayload(
  draft: WorkItemCreateDraft,
  reporterUserId: number,
): WorkItemCreatePayload {
  const estimatedHours = positiveNumber(draft.estimatedHours)
  const estimatedMinutes = estimatedHours === undefined || Number.isNaN(estimatedHours)
    ? undefined
    : Math.round(estimatedHours * 60)
  const payload: WorkItemCreatePayload = {
    requirement_id: draft.requirementScope.requirementIds[0] || 0,
    kind: 'task',
    title: draft.title.trim(),
    description: draft.description.trim(),
    status: draft.status,
    priority: draft.priority,
    reporter_user_id: reporterUserId,
  }
  if (draft.assigneeId) payload.assignee_id = draft.assigneeId
  if (draft.taskClientId) payload.task_client_id = draft.taskClientId
  if (estimatedMinutes !== undefined) payload.estimated_minutes = estimatedMinutes
  if (draft.startsAt) payload.starts_at = startOfLocalDay(draft.startsAt)
  if (draft.dueAt) payload.due_at = endOfLocalDay(draft.dueAt)
  return payload
}
