import { canRetainStaleApiData } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  PlatformUser,
  ProjectWorkTask,
  ScrumProductRequirementNew,
} from '../api'
import {
  contentPriorityLabel,
  requirementPriorityLabel,
  taskStatusLabel,
} from './content-options'
import {
  formatPMDate,
  ownerPreview,
  type UserPreviewModel,
} from './selectors'
import type { ProjectRefreshState } from './refresh-state'

type RetryIntent = () => void
type ReadStatus = 'pending' | 'error' | 'success'

export type RequirementProgressModel = {
  completed: number
  total: number
  label: string
  warning?: string
}

export type RequirementRowModel = {
  id: number
  title: string
  description: string
  documentUrl?: string
  designUrl?: string
  deliverables: {
    requirementDocument: boolean
    design: boolean
    testCases: boolean
  }
  planUnitTitle: string
  priority: string
  priorityKey: string
  workflow: {
    instanceId?: number
    status: string
    currentNodeNames: string[]
    currentNodes: Array<{
      id?: string
      name: string
      plannedEndAt?: string
      overdue: boolean
    }>
    completedSteps: number
    totalSteps: number
  }
  progress: RequirementProgressModel
  owner: UserPreviewModel
  developers: UserPreviewModel[]
  acceptance: string
}

export type TaskRowModel = {
  id: number
  requirementId: number
  key: string
  title: string
  description: string
  status: string
  statusKey: string
  priority: string
  priorityKey: string
  assignee: UserPreviewModel
  due: string
  dueAt?: string
  overdue: boolean
  unassigned: boolean
  estimate?: string
}

export type RequirementListState =
  | { status: 'loading' }
  | { status: 'error'; retry: RetryIntent }
  | {
      status: 'empty'
      reason: 'initial' | 'filtered'
      refresh: ProjectRefreshState
    }
  | {
      status: 'ready'
      rows: RequirementRowModel[]
      total: number
      page: number
      pageSize: number
      refresh: ProjectRefreshState
    }

export type TaskListState =
  | { status: 'loading' }
  | { status: 'error'; retry: RetryIntent }
  | { status: 'empty'; refresh: ProjectRefreshState }
  | {
      status: 'ready'
      rows: TaskRowModel[]
      total: number
      page: number
      pageSize: number
      refresh: ProjectRefreshState
    }

function progressModel(requirement: ScrumProductRequirementNew): RequirementProgressModel {
  const total = Math.max(0, Number(requirement.total_task_count ?? requirement.task_count ?? 0))
  const completed = Math.min(total, Math.max(0, Number(requirement.completed_task_count ?? 0)))
  const requirementCopy = copy.projectManagement.content.requirement
  return {
    completed,
    total,
    label: requirementCopy.progress(completed, total),
  }
}

function workflowModel(requirement: ScrumProductRequirementNew): RequirementRowModel['workflow'] {
  const workflow = requirement.workflow_overview
  if (!workflow) {
    return {
      status: 'unbound',
      currentNodeNames: [],
      currentNodes: [],
      completedSteps: 0,
      totalSteps: 0,
    }
  }
  return {
    instanceId: workflow.instance_id,
    status: workflow.status,
    currentNodeNames: workflow.current_node_names ?? [],
    currentNodes: (workflow.current_nodes ?? []).map((node) => ({
      id: node.id,
      name: node.name,
      plannedEndAt: node.planned_end_at || undefined,
      overdue: Boolean(node.overdue),
    })),
    completedSteps: Math.max(0, Number(workflow.completed_steps || 0)),
    totalSteps: Math.max(0, Number(workflow.total_steps || 0)),
  }
}

function requirementRow(
  requirement: ScrumProductRequirementNew,
  users: PlatformUser[],
  planUnitTitle: string,
): RequirementRowModel {
  return {
    id: requirement.id,
    title: requirement.title,
    description: requirement.description || '',
    documentUrl: requirement.product_doc_url?.trim() || undefined,
    designUrl: requirement.design_url?.trim() || undefined,
    deliverables: {
      requirementDocument: Boolean(
        requirement.deliverable_overview?.requirement_document || requirement.product_doc_url?.trim(),
      ),
      design: Boolean(requirement.deliverable_overview?.design || requirement.design_url?.trim()),
      testCases: Boolean(requirement.deliverable_overview?.test_cases),
    },
    planUnitTitle,
    priority: requirementPriorityLabel(requirement.priority),
    priorityKey: String(requirement.priority || 'p1'),
    workflow: workflowModel(requirement),
    progress: progressModel(requirement),
    owner: ownerPreview(requirement.product_owner_user_id, users),
    developers: (requirement.developer_user_ids ?? [])
      .map((id) => ownerPreview(id, users)),
    acceptance: requirement.acceptance_criteria || copy.projectManagement.content.common.noOwner,
  }
}

function localDateKey(value: Date): string {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function taskOverdue(task: ProjectWorkTask): boolean {
  const dueAt = task.due_at?.trim()
  if (!dueAt || ['completed', 'done', 'cancelled'].includes(String(task.status || ''))) {
    return false
  }
  const dueKey = dueAt.slice(0, 10)
  return /^\d{4}-\d{2}-\d{2}$/.test(dueKey) && dueKey < localDateKey(new Date())
}

function taskRow(task: ProjectWorkTask, users: PlatformUser[]): TaskRowModel {
  const estimatedMinutes = Math.max(0, Number(task.estimated_minutes || 0))
  return {
    id: task.id,
    requirementId: task.requirement_id,
    key: task.task_key || `#${task.id}`,
    title: task.title,
    description: task.description || '',
    status: taskStatusLabel(task.status),
    statusKey: String(task.status || 'draft'),
    priority: contentPriorityLabel(task.priority),
    priorityKey: String(task.priority || 'normal'),
    assignee: ownerPreview(task.assignee_id, users),
    due: formatPMDate(task.due_at),
    dueAt: task.due_at || undefined,
    overdue: taskOverdue(task),
    unassigned: !Number(task.assignee_id || 0),
    estimate: estimatedMinutes > 0
      ? estimatedMinutes % 60 === 0
        ? `${estimatedMinutes / 60}h`
        : `${Math.round((estimatedMinutes / 60) * 10) / 10}h`
      : undefined,
  }
}

export function resolveRequirementListState(input: {
  status: ReadStatus
  data?: { rows: ScrumProductRequirementNew[]; total: number }
  error?: unknown
  page: number
  pageSize: number
  fetching: boolean
  filtered: boolean
  users: PlatformUser[]
  planUnitTitle: string
  retry: RetryIntent
}): RequirementListState {
  if (input.status === 'pending' && !input.data) return { status: 'loading' }
  if (
    input.status === 'error' &&
    (!input.data || !canRetainStaleApiData(input.error))
  ) {
    return { status: 'error', retry: input.retry }
  }
  const refresh: ProjectRefreshState = input.status === 'error'
    ? { status: 'stale_error', retry: input.retry }
    : input.fetching
      ? { status: 'refreshing' }
      : { status: 'idle' }
  const rows = input.data?.rows ?? []
  if (rows.length === 0) {
    return {
      status: 'empty',
      reason: input.filtered ? 'filtered' : 'initial',
      refresh,
    }
  }
  return {
    status: 'ready',
    rows: rows.map((row) => requirementRow(row, input.users, input.planUnitTitle)),
    total: input.data?.total ?? 0,
    page: input.page,
    pageSize: input.pageSize,
    refresh,
  }
}

export function resolveTaskListState(input: {
  status: ReadStatus
  data?: { rows: ProjectWorkTask[]; total: number }
  error?: unknown
  page: number
  pageSize: number
  fetching: boolean
  users: PlatformUser[]
  retry: RetryIntent
}): TaskListState {
  if (input.status === 'pending' && !input.data) return { status: 'loading' }
  if (
    input.status === 'error' &&
    (!input.data || !canRetainStaleApiData(input.error))
  ) {
    return { status: 'error', retry: input.retry }
  }
  const refresh: ProjectRefreshState = input.status === 'error'
    ? { status: 'stale_error', retry: input.retry }
    : input.fetching
      ? { status: 'refreshing' }
      : { status: 'idle' }
  const rows = input.data?.rows ?? []
  if (rows.length === 0) return { status: 'empty', refresh }
  return {
    status: 'ready',
    rows: rows.map((row) => taskRow(row, input.users)),
    total: input.data?.total ?? 0,
    page: input.page,
    pageSize: input.pageSize,
    refresh,
  }
}
