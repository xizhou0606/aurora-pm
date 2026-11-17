import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

export const TASK_PLAN_CAPABILITY = 'requirement.task_plan'

export type TaskPlanItem = {
  title?: string
  description?: string
  acceptance_criteria?: readonly string[]
}

export type TaskPlanChain = {
  lane?: string
  summary?: string
  tasks?: readonly TaskPlanItem[]
}

export type TaskPlanCreatedTask = {
  id: number
  task_key?: string
  title?: string
  lane?: string
}

export type TaskPlanRun = {
  id: number
  capability: string
  status: string
  subject_id?: number
  error_message?: string
  output?: {
    taskPlan?: { chains?: readonly TaskPlanChain[] }
    createdTasks?: readonly TaskPlanCreatedTask[]
  }
}

function requiredData<T>(response: unknown): T {
  const value = unwrapApiData<T>(response)
  if (value === undefined) throw new Error('PM API returned no data')
  return value
}

function taskPlanRequest<T>(work: () => Promise<T>, code: string): Promise<T> {
  return work().catch((error: unknown) => {
    throw toSafeApiError(error, code)
  })
}

export function createTaskPlanRun(requirementId: number, userPrompt?: string): Promise<TaskPlanRun> {
  return taskPlanRequest(async () => requiredData<TaskPlanRun>(await platformApi.post('/ai/runs', {
    capability: TASK_PLAN_CAPABILITY,
    subject_type: 'requirement',
    subject_id: requirementId,
    ...(userPrompt ? { user_prompt: userPrompt } : {}),
  })), 'AI_TASK_PLAN_CREATE_FAILED')
}

export function fetchTaskPlanRun(id: number, signal?: AbortSignal): Promise<TaskPlanRun> {
  return taskPlanRequest(async () => requiredData<TaskPlanRun>(
    await platformApi.get(`/ai/runs/${id}`, { signal }),
  ), 'AI_TASK_PLAN_FETCH_FAILED')
}

export type TaskPlanLaneAssignees = {
  frontend?: number | null
  backend?: number | null
  other?: number | null
}

export function applyTaskPlanRun(
  id: number,
  assignees?: TaskPlanLaneAssignees,
): Promise<TaskPlanRun> {
  return taskPlanRequest(async () => requiredData<TaskPlanRun>(
    await platformApi.post(`/ai/runs/${id}/apply`, assignees ? { assignees } : {}),
  ), 'AI_TASK_PLAN_APPLY_FAILED')
}
