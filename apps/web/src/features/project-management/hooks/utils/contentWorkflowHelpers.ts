import { isSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
} from '../../api'
import type {
  ContentDeleteState,
  ContentTargetState,
  PlanUnitContentSearch,
  PlanUnitTab,
  RequirementEditorState,
  RequirementMoveState,
  TaskEditorState,
} from '../../model'

export const CLOSED_REQUIREMENT_EDITOR: RequirementEditorState = { status: 'closed' }
export const CLOSED_TASK_EDITOR: TaskEditorState = { status: 'closed' }
export const CLOSED_MOVE: RequirementMoveState = { status: 'closed' }
export const CLOSED_DELETE: ContentDeleteState = { status: 'closed' }
export const DEFAULT_SEARCH: PlanUnitContentSearch = {
  requirement: { q: '', page: 1, pageSize: 50 },
  task: { page: 1, pageSize: 50 },
}
export const contentCopy = copy.projectManagement.content

export const DEFAULT_OPERATION_ACCESS = {
  viewProjectSets: true,
  viewBoard: true,
  viewTaskExecution: true,
  viewPlanRequirements: true,
  createPlanRequirement: true,
  updatePlanRequirement: true,
  deletePlanRequirement: true,
  createTask: true,
  updateTask: true,
  deleteTask: true,
} as const

export function optionalPageSize(value: number): ContentPageSize | undefined {
  if (value === 50) return undefined
  return value === 20 || value === 100 ? value : undefined
}

export function safeErrorMessage(error: unknown, fallback: string): string {
  return isSafeApiError(error) && error.status === 403
    ? copy.projectManagement.content.common.contextChanged
    : fallback
}

export function targetState(input: {
  status: 'pending' | 'error' | 'success'
  items: Array<{ id: number; label: string }>
  retry: () => void
}): ContentTargetState {
  if (input.status === 'pending') return { status: 'loading' }
  if (input.status === 'error') return { status: 'error', retry: input.retry }
  return input.items.length > 0
    ? { status: 'ready', items: input.items }
    : { status: 'empty' }
}

export function findRequirement(
  id: number,
  rows: ScrumProductRequirementNew[],
  fallback: ScrumProductRequirementNew | null,
): ScrumProductRequirementNew | undefined {
  return rows.find((row) => row.id === id) ?? (fallback?.id === id ? fallback : undefined)
}

export function findTask(
  id: number,
  rows: ProjectWorkTask[],
  fallback: ProjectWorkTask | null,
): ProjectWorkTask | undefined {
  return rows.find((row) => row.id === id) ?? (fallback?.id === id ? fallback : undefined)
}

export type ContentPageSize = 20 | 50 | 100
export type ProjectContentSearchPatch = {
  tab?: PlanUnitTab
  req_q?: string
  req_status?: PlanUnitContentSearch['requirement']['status']
  req_priority?: PlanUnitContentSearch['requirement']['priority']
  req_page?: number
  req_page_size?: ContentPageSize
  task_requirement_id?: number
  task_id?: number
  task_page?: number
  task_page_size?: ContentPageSize
}