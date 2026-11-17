import {
  platformApi,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'
import type {
  CreateProjectWorkTaskPayload,
  CreateProjectWorkTaskResult,
  DeleteProjectWorkTaskResult,
  ListProjectWorkTasksInput,
  PatchProjectWorkTaskPayload,
  PatchProjectWorkTaskResult,
  ProjectWorkTask,
  ScrumDeferredListInput,
  ScrumPagedRows,
} from './contracts'
import {
  requireResourceMutationResult,
  resolveDeleteMutationResult,
  resolvePatchMutationResult,
} from './mutation-response'
import { clearProjectRequirementScopeOptionsCache } from './scope'
import { listResponseUsers } from './users'

export async function listScrumTasksNew(
  input: ListProjectWorkTasksInput | ScrumDeferredListInput,
  signal?: AbortSignal,
): Promise<ScrumPagedRows<ProjectWorkTask>> {
  if (!input.projectNewId && !input.requirementNewId) return { rows: [], total: 0, users: [] }
  const params: Record<string, string> = {
    limit: String(input.limit ?? 50),
    offset: String(input.offset ?? 0),
  }
  if (input.projectNewId) params.project_new_id = String(input.projectNewId)
  if ('versionTaskNewId' in input && input.versionTaskNewId) {
    params.version_task_new_id = String(input.versionTaskNewId)
  }
  if (input.requirementNewId) params.requirement_new_id = String(input.requirementNewId)
  try {
    const response = await platformApi.get('/scrum-tasks-new', { params, signal })
    const rows = unwrapApiRows<ProjectWorkTask>(response)
    return {
      rows: rows.filter((row) => Number(row.id) > 0),
      total: unwrapApiTotal(response),
      users: listResponseUsers(response),
    }
  } catch (error) {
    throw toSafeApiError(error, 'TASKS_FAILED')
  }
}

export async function getProjectWorkTask(
  id: number,
  signal?: AbortSignal,
): Promise<ProjectWorkTask> {
  if (!Number.isInteger(id) || id <= 0) {
    throw toSafeApiError(null, 'TASK_NOT_FOUND')
  }
  try {
    const response = await platformApi.get(`/tasks/${id}`, { signal })
    const task = unwrapApiData<ProjectWorkTask>(response)
    if (!task || Number(task.id) !== id) throw new Error('invalid task response')
    return task
  } catch (error) {
    throw toSafeApiError(error, 'TASKS_FAILED')
  }
}

export async function createProjectWorkTask(
  body: CreateProjectWorkTaskPayload,
): Promise<CreateProjectWorkTaskResult> {
  try {
    const response = await platformApi.post('/tasks', body)
    const result = requireResourceMutationResult(
      unwrapApiData<CreateProjectWorkTaskResult>(response),
      'TASK_CREATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'TASK_CREATE_FAILED')
  }
}

export async function patchProjectWorkTask(
  id: number,
  body: PatchProjectWorkTaskPayload,
): Promise<PatchProjectWorkTaskResult> {
  try {
    const response = await platformApi.patch(`/tasks/${id}`, body)
    const result = resolvePatchMutationResult(
      unwrapApiData<PatchProjectWorkTaskResult>(response),
      { id, ...body } as PatchProjectWorkTaskResult,
      'TASK_UPDATE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'TASK_UPDATE_FAILED')
  }
}

export async function deleteProjectWorkTask(
  id: number,
): Promise<DeleteProjectWorkTaskResult> {
  try {
    const response = await platformApi.delete(`/tasks/${id}`)
    const result = resolveDeleteMutationResult(
      unwrapApiData<DeleteProjectWorkTaskResult>(response),
      'TASK_DELETE_FAILED',
    )
    clearProjectRequirementScopeOptionsCache()
    return result
  } catch (error) {
    throw toSafeApiError(error, 'TASK_DELETE_FAILED')
  }
}
