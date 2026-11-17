import { queryOptions, useQuery } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'
import { PMSafeApiError } from '@/shared/api'
import {
  listProjectOverviewBugs,
  listScrumProductRequirementsNew,
  listScrumTasksNew,
  type ProjectOverviewBug,
  type ProjectWorkTask,
  type ScrumPagedRows,
  type ScrumProductRequirementNew,
} from '../api'

const AGGREGATE_PAGE_SIZE = 500

export type ProjectOverviewScope = {
  projectId: number
  teamId: number
  projectWorkspaceId?: number
  itemId?: number
}

export type ProjectOverviewFacts = {
  requirements: ScrumProductRequirementNew[]
  tasks: ProjectWorkTask[]
  bugs: ProjectOverviewBug[]
}

function positiveId(value: unknown, label: string): number {
  const id = Number(value)
  if (!Number.isInteger(id) || id <= 0) {
    throw new Error(`PM overview ${label} requires a positive ID`)
  }
  return id
}

function nonNegativeId(value: unknown, label: string): number {
  const id = Number(value)
  if (!Number.isInteger(id) || id < 0) {
    throw new Error(`PM overview ${label} requires a non-negative ID`)
  }
  return id
}

export function normalizeProjectOverviewScope(
  input: ProjectOverviewScope,
): ProjectOverviewScope {
  return {
    projectId: positiveId(input.projectId, 'project'),
    teamId: nonNegativeId(input.teamId, 'team'),
    ...(input.projectWorkspaceId === undefined
      ? {}
      : { projectWorkspaceId: positiveId(input.projectWorkspaceId, 'workspace') }),
    ...(input.itemId === undefined
      ? {}
      : { itemId: positiveId(input.itemId, 'item') }),
  }
}

function belongsToWorkspaceTeam(
  row: { workspace_id?: number; team_id: number },
  workspaceId: number,
  teamId: number,
): boolean {
  if (teamId === 0) {
    return row.team_id === 0 && row.workspace_id === workspaceId
  }
  return row.team_id === teamId && (
    row.workspace_id === undefined || row.workspace_id === workspaceId
  )
}

async function completeRows<T extends { id: number }>(
  first: ScrumPagedRows<T>,
  load: (offset: number) => Promise<ScrumPagedRows<T>>,
): Promise<T[]> {
  const expectedTotal = Math.max(0, Number(first.total || 0))
  const pages = Math.ceil(expectedTotal / AGGREGATE_PAGE_SIZE)
  const rest = pages > 1
    ? await Promise.all(
        Array.from({ length: pages - 1 }, (_, index) => (
          load((index + 1) * AGGREGATE_PAGE_SIZE)
        )),
      )
    : []
  const rows = new Map<number, T>()
  for (const page of [first, ...rest]) {
    if (Number(page.total || 0) !== expectedTotal) {
      throw new PMSafeApiError('PROJECT_OVERVIEW_INCOMPLETE', 409)
    }
    for (const row of page.rows) {
      const id = Number(row.id)
      if (!Number.isInteger(id) || id <= 0 || rows.has(id)) {
        throw new PMSafeApiError('PROJECT_OVERVIEW_INCOMPLETE', 409)
      }
      rows.set(id, row)
    }
  }
  if (rows.size !== expectedTotal) {
    throw new PMSafeApiError('PROJECT_OVERVIEW_INCOMPLETE', 409)
  }
  return [...rows.values()]
}

async function loadRequirements(
  workspaceId: number,
  scope: ProjectOverviewScope,
  signal: AbortSignal,
): Promise<ScrumProductRequirementNew[]> {
  const input = {
    projectNewId: scope.projectId,
    versionTaskNewId: scope.itemId,
    limit: AGGREGATE_PAGE_SIZE,
  }
  const first = await listScrumProductRequirementsNew({ ...input, offset: 0 }, signal)
  const rows = await completeRows(first, async (offset) => (
    listScrumProductRequirementsNew({ ...input, offset }, signal)
  ))
  if (rows.some((row) => (
    !belongsToWorkspaceTeam(row, workspaceId, scope.teamId) ||
    row.project_new_id !== scope.projectId ||
    (scope.itemId !== undefined && row.version_task_new_id !== scope.itemId)
  ))) {
    throw new PMSafeApiError('PROJECT_OVERVIEW_INCOMPLETE', 409)
  }
  return rows
}

async function loadTasks(
  workspaceId: number,
  scope: ProjectOverviewScope,
  signal: AbortSignal,
): Promise<ProjectWorkTask[]> {
  const input = {
    projectNewId: scope.projectId,
    versionTaskNewId: scope.itemId,
    limit: AGGREGATE_PAGE_SIZE,
  }
  const first = await listScrumTasksNew({ ...input, offset: 0 }, signal)
  const rows = await completeRows(first, async (offset) => (
    listScrumTasksNew({ ...input, offset }, signal)
  ))
  if (rows.some((row) => (
    !belongsToWorkspaceTeam(row, workspaceId, scope.teamId) ||
    row.project_set_id !== scope.projectId ||
    (scope.itemId !== undefined && row.project_id !== scope.itemId)
  ))) {
    throw new PMSafeApiError('PROJECT_OVERVIEW_INCOMPLETE', 409)
  }
  return rows
}

export function projectOverviewQueryOptions(
  workspaceId: number,
  input: ProjectOverviewScope,
) {
  const scope = normalizeProjectOverviewScope(input)
  if (
    (scope.projectWorkspaceId !== undefined && scope.projectWorkspaceId !== workspaceId) ||
    (scope.teamId === 0 && scope.projectWorkspaceId !== workspaceId)
  ) {
    throw new PMSafeApiError('PROJECT_OVERVIEW_INCOMPLETE', 409)
  }
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'projectSets',
      'overview-aggregate',
      'project',
      scope.projectId,
      'team',
      scope.teamId,
      'item',
      scope.itemId ?? 'all',
    ),
    queryFn: async ({ signal }): Promise<ProjectOverviewFacts> => {
      const [requirements, tasks, bugs] = await Promise.all([
        loadRequirements(workspaceId, scope, signal),
        loadTasks(workspaceId, scope, signal),
        listProjectOverviewBugs({
          projectSetId: scope.projectId,
          planUnitId: scope.itemId,
        }, signal),
      ])
      return { requirements, tasks, bugs }
    },
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function useProjectOverviewQuery(
  input: ProjectOverviewScope,
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const trustedProjectWorkspace = input.projectWorkspaceId === undefined ||
    input.projectWorkspaceId === workspaceId
  const trustedLegacyTeam = input.teamId > 0 || input.projectWorkspaceId === workspaceId
  const valid = enabled && workspaceId > 0 && input.projectId > 0 && (
    input.itemId === undefined || input.itemId > 0
  ) && input.teamId >= 0 && trustedProjectWorkspace && trustedLegacyTeam
  const safeScope = valid
    ? input
    : { projectId: 1, teamId: 1 }
  return useQuery({
    ...projectOverviewQueryOptions(
      valid ? workspaceId : 1,
      safeScope,
    ),
    enabled: valid,
  })
}
