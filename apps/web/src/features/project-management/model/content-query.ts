import { keepPreviousData, queryOptions, useQuery } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'
import { PMSafeApiError } from '@/shared/api'
import {
  getProjectWorkTask,
  listScrumProductRequirementsNew,
  listScrumTasksNew,
  type ProjectWorkTask,
  type ListProjectWorkTasksInput,
  type ListScrumRequirementsInput,
} from '../api'

type TaskDeepLinkInput = {
  projectNewId: number
  versionTaskNewId: number
  requirementNewId: number
  taskId: number
}

export const DEFAULT_REQUIREMENT_LIST_INPUT = Object.freeze({
  projectNewId: 0,
  versionTaskNewId: 0,
  q: undefined,
  status: undefined,
  priority: undefined,
  limit: 50,
  offset: 0,
})

export const DEFAULT_TASK_LIST_INPUT = Object.freeze({
  projectNewId: 0,
  versionTaskNewId: 0,
  requirementNewId: undefined,
  limit: 50,
  offset: 0,
})

function positiveId(value: unknown, resource: string): number {
  const id = Number(value)
  if (!Number.isInteger(id) || id <= 0) {
    throw new Error(`PM ${resource} query requires a positive ID`)
  }
  return id
}

function pageValue(value: unknown, fallback: number, minimum: number): number {
  const number = Number(value)
  return Number.isInteger(number) && number >= minimum ? number : fallback
}

function inWorkspace(row: { workspace_id?: number }, workspaceId: number): boolean {
  return row.workspace_id === undefined || row.workspace_id === workspaceId
}

function incompleteContent(): never {
  throw new PMSafeApiError('PROJECT_CONTENT_INCOMPLETE', 409)
}

export function normalizeRequirementListInput(
  input: ListScrumRequirementsInput,
): Required<Pick<ListScrumRequirementsInput, 'projectNewId' | 'versionTaskNewId' | 'limit' | 'offset'>> &
  Pick<ListScrumRequirementsInput, 'q' | 'status' | 'priority'> {
  return {
    projectNewId: positiveId(input.projectNewId, 'requirement project'),
    versionTaskNewId: positiveId(input.versionTaskNewId, 'requirement plan unit'),
    q: input.q?.trim() || undefined,
    status: input.status,
    priority: input.priority,
    limit: pageValue(input.limit, 50, 1),
    offset: pageValue(input.offset, 0, 0),
  }
}

export function normalizeTaskListInput(
  input: ListProjectWorkTasksInput,
): Required<Pick<ListProjectWorkTasksInput, 'projectNewId' | 'versionTaskNewId' | 'limit' | 'offset'>> &
  Pick<ListProjectWorkTasksInput, 'requirementNewId'> {
  const requirementId = Number(input.requirementNewId || 0)
  return {
    projectNewId: positiveId(input.projectNewId, 'task project'),
    versionTaskNewId: positiveId(input.versionTaskNewId, 'task plan unit'),
    requirementNewId: Number.isInteger(requirementId) && requirementId > 0
      ? requirementId
      : undefined,
    limit: pageValue(input.limit, 50, 1),
    offset: pageValue(input.offset, 0, 0),
  }
}

export function requirementListQueryOptions(
  workspaceId: number,
  input: ListScrumRequirementsInput,
) {
  const normalized = normalizeRequirementListInput(input)
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'requirements',
      'list',
      'project',
      normalized.projectNewId,
      'plan-unit',
      normalized.versionTaskNewId,
      {
        q: normalized.q,
        status: normalized.status,
        priority: normalized.priority,
        limit: normalized.limit,
        offset: normalized.offset,
      },
    ),
    queryFn: async ({ signal }) => {
      const page = await listScrumProductRequirementsNew(normalized, signal)
      if (page.rows.some((row) => (
        !inWorkspace(row, workspaceId) ||
        row.project_new_id !== normalized.projectNewId ||
        row.version_task_new_id !== normalized.versionTaskNewId
      ))) incompleteContent()
      return page
    },
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function taskListQueryOptions(
  workspaceId: number,
  input: ListProjectWorkTasksInput,
) {
  const normalized = normalizeTaskListInput(input)
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'tasks',
      'list',
      'project',
      normalized.projectNewId,
      'plan-unit',
      normalized.versionTaskNewId,
      'requirement',
      normalized.requirementNewId ?? 'all',
      { limit: normalized.limit, offset: normalized.offset },
    ),
    queryFn: async ({ signal }) => {
      const page = await listScrumTasksNew(normalized, signal)
      if (page.rows.some((row) => (
        !inWorkspace(row, workspaceId) ||
        row.project_set_id !== normalized.projectNewId ||
        row.project_id !== normalized.versionTaskNewId ||
        (normalized.requirementNewId !== undefined &&
          row.requirement_id !== normalized.requirementNewId)
      ))) incompleteContent()
      return page
    },
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function taskDeepLinkQueryOptions(
  workspaceId: number,
  input: TaskDeepLinkInput,
) {
  const normalized = {
    projectNewId: positiveId(input.projectNewId, 'task deep-link project'),
    versionTaskNewId: positiveId(input.versionTaskNewId, 'task deep-link plan unit'),
    requirementNewId: positiveId(input.requirementNewId, 'task deep-link requirement'),
    taskId: positiveId(input.taskId, 'task deep-link target'),
  }
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'tasks',
      'deep-link',
      'project',
      normalized.projectNewId,
      'plan-unit',
      normalized.versionTaskNewId,
      'requirement',
      normalized.requirementNewId,
      'task',
      normalized.taskId,
    ),
    queryFn: async ({ signal }): Promise<ProjectWorkTask | null> => {
      const target = await getProjectWorkTask(normalized.taskId, signal)
      if (!inWorkspace(target, workspaceId)) incompleteContent()
      return target.project_set_id === normalized.projectNewId &&
        target.project_id === normalized.versionTaskNewId &&
        target.requirement_id === normalized.requirementNewId
        ? target
        : null
    },
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function useRequirementListQuery(
  input: ListScrumRequirementsInput,
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0 &&
    Number(input.projectNewId) > 0 && Number(input.versionTaskNewId) > 0
  return useQuery({
    ...requirementListQueryOptions(
      valid ? workspaceId : 1,
      valid ? input : { ...input, projectNewId: 1, versionTaskNewId: 1 },
    ),
    enabled: valid,
    placeholderData: keepPreviousData,
  })
}

export function useTaskListQuery(
  input: ListProjectWorkTasksInput,
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0 &&
    Number(input.projectNewId) > 0 && Number(input.versionTaskNewId) > 0
  return useQuery({
    ...taskListQueryOptions(
      valid ? workspaceId : 1,
      valid ? input : { ...input, projectNewId: 1, versionTaskNewId: 1 },
    ),
    enabled: valid,
    placeholderData: keepPreviousData,
  })
}

export function useTaskDeepLinkQuery(
  input: TaskDeepLinkInput,
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0 &&
    Number(input.projectNewId) > 0 && Number(input.versionTaskNewId) > 0 &&
    Number(input.requirementNewId) > 0 && Number(input.taskId) > 0
  return useQuery({
    ...taskDeepLinkQueryOptions(
      valid ? workspaceId : 1,
      valid
        ? input
        : {
            projectNewId: 1,
            versionTaskNewId: 1,
            requirementNewId: 1,
            taskId: 1,
          },
    ),
    enabled: valid,
  })
}
