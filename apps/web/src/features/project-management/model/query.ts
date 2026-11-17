import { keepPreviousData, queryOptions, useQueries, useQuery } from '@tanstack/react-query'

import {
  platformUsersQueryOptions as sharedPlatformUsersQueryOptions,
  pmKeys,
} from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'
import {
  getProjectMembers,
  getScrumBoardNew,
  listScrumProjectAuditLogsNew,
  listScrumProjectsNew,
  listPlatformUsers,
  listPublishedPlanUnitWorkflowTemplates,
  listPublishedWorkflowTemplates,
  listWorkspaceTeamMembers,
  listWorkspaceTeams,
  type ListScrumProjectsNewInput,
} from '../api'

export const DEFAULT_PROJECT_LIST_INPUT = Object.freeze({
  limit: 20,
  offset: 0,
})

export const DEFAULT_PROJECT_AUDIT_INPUT = Object.freeze({
  limit: 30,
  offset: 0,
})

function positiveResourceId(value: unknown, resource: string): number {
  const id = Number(value || 0)
  if (!Number.isInteger(id) || id <= 0) {
    throw new Error(`PM ${resource} query requires a positive ID`)
  }
  return id
}

function normalizedPageValue(
  value: unknown,
  fallback: number,
  minimum: number,
): number {
  const number = Number(value)
  return Number.isFinite(number) && Number.isInteger(number) && number >= minimum
    ? number
    : fallback
}

export function normalizeProjectListInput(
  input: ListScrumProjectsNewInput = DEFAULT_PROJECT_LIST_INPUT,
) {
  const teamId = Number(input.teamId || 0)
  return {
    teamId: Number.isInteger(teamId) && teamId > 0 ? teamId : undefined,
    q: input.q?.trim() || undefined,
    limit: normalizedPageValue(input.limit, DEFAULT_PROJECT_LIST_INPUT.limit, 1),
    offset: normalizedPageValue(input.offset, 0, 0),
  }
}

export function projectListQueryOptions(
  workspaceId: number,
  input: ListScrumProjectsNewInput = DEFAULT_PROJECT_LIST_INPUT,
) {
  const normalized = normalizeProjectListInput(input)
  return queryOptions({
    queryKey: pmKeys.resource(workspaceId, 'projectSets', 'list', normalized),
    queryFn: ({ signal }) => listScrumProjectsNew(normalized, signal),
    staleTime: 30_000,
    retry: false,
    retryOnMount: true,
    refetchOnMount: false,
  })
}

export function projectBoardQueryOptions(workspaceId: number, projectId: number) {
  const id = positiveResourceId(projectId, 'board')
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'planUnits',
      'board',
      id,
      'summary-version-tasks',
    ),
    queryFn: ({ signal }) => getScrumBoardNew(
      id,
      ['summary', 'version_tasks'],
      signal,
    ),
    staleTime: 30_000,
    retry: false,
    retryOnMount: true,
    refetchOnMount: false,
  })
}

export function planUnitMembersQueryOptions(workspaceId: number, planUnitId: number) {
  const id = positiveResourceId(planUnitId, 'plan unit members')
  return queryOptions({
    queryKey: pmKeys.resource(workspaceId, 'planUnits', 'members', id),
    queryFn: ({ signal }) => getProjectMembers(id, { page_size: 50 }, signal),
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function planUnitMembersQueryKey(workspaceId: number) {
  return pmKeys.resource(workspaceId, 'planUnits', 'members')
}

export function usePlanUnitMembersQueries(planUnitIds: readonly number[], enabled = true) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const ids = [...new Set(planUnitIds
    .map((id) => Number(id))
    .filter((id) => Number.isInteger(id) && id > 0))]
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0
  return useQueries({
    queries: ids.map((planUnitId) => ({
      ...planUnitMembersQueryOptions(valid ? workspaceId : 1, planUnitId),
      enabled: valid,
    })),
  })
}

export function projectAuditQueryOptions(
  workspaceId: number,
  projectId: number,
  input: { limit?: number; offset?: number } = DEFAULT_PROJECT_AUDIT_INPUT,
) {
  const id = positiveResourceId(projectId, 'audit')
  const normalized = {
    limit: normalizedPageValue(input.limit, DEFAULT_PROJECT_AUDIT_INPUT.limit, 1),
    offset: normalizedPageValue(input.offset, 0, 0),
  }
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'audit',
      'project',
      id,
      normalized,
    ),
    queryFn: ({ signal }) => listScrumProjectAuditLogsNew(
      id,
      normalized,
      signal,
    ),
    staleTime: 15_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function platformUsersQueryOptions(workspaceId: number) {
  return sharedPlatformUsersQueryOptions(workspaceId, listPlatformUsers)
}

export function workspaceTeamsQueryOptions(workspaceId: number) {
  return queryOptions({
    queryKey: pmKeys.resource(workspaceId, 'workspace', 'teams'),
    queryFn: ({ signal }) => listWorkspaceTeams(signal),
    staleTime: 120_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function workspaceTeamMembersQueryOptions(
  workspaceId: number,
  teamId: number,
) {
  const id = positiveResourceId(teamId, 'team members')
  return queryOptions({
    queryKey: pmKeys.resource(workspaceId, 'workspace', 'teams', id, 'members'),
    queryFn: ({ signal }) => listWorkspaceTeamMembers(id, signal),
    staleTime: 120_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
}

export function useProjectListQuery(
  input: ListScrumProjectsNewInput = DEFAULT_PROJECT_LIST_INPUT,
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0
  return useQuery({
    ...projectListQueryOptions(valid ? workspaceId : 1, input),
    enabled: valid,
    placeholderData: keepPreviousData,
  })
}

export function useProjectBoardQuery(projectId: number, enabled = true) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && workspaceId > 0 && Number.isInteger(projectId) && projectId > 0
  return useQuery({
    ...projectBoardQueryOptions(valid ? workspaceId : 1, valid ? projectId : 1),
    enabled: valid,
  })
}

export function useProjectAuditQuery(
  projectId: number,
  enabled: boolean,
  input: { limit?: number; offset?: number } = DEFAULT_PROJECT_AUDIT_INPUT,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && workspaceId > 0 && Number.isInteger(projectId) && projectId > 0
  return useQuery({
    ...projectAuditQueryOptions(valid ? workspaceId : 1, valid ? projectId : 1, input),
    enabled: valid,
    placeholderData: keepPreviousData,
  })
}

export function usePlatformUsersQuery(
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0
  return useQuery({
    ...platformUsersQueryOptions(valid ? workspaceId : 1),
    enabled: valid,
  })
}

export function useWorkspaceTeamsQuery(enabled = true) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0
  return useQuery({
    ...workspaceTeamsQueryOptions(valid ? workspaceId : 1),
    enabled: valid,
  })
}

export function usePublishedPlanUnitWorkflowTemplatesQuery(enabled = true) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && workspaceId > 0
  return useQuery({
    queryKey: pmKeys.resource(valid ? workspaceId : 1, 'workflowTemplates', 'planUnit', 'published'),
    queryFn: ({ signal }) => listPublishedPlanUnitWorkflowTemplates(signal),
    enabled: valid,
    staleTime: 30_000,
    retry: false,
  })
}

export function usePublishedRequirementWorkflowTemplatesQuery(enabled = true) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled && workspaceId > 0
  return useQuery({
    queryKey: pmKeys.resource(valid ? workspaceId : 1, 'workflowTemplates', 'requirement', 'published'),
    queryFn: ({ signal }) => listPublishedWorkflowTemplates('requirement', signal),
    enabled: valid,
    staleTime: 30_000,
    retry: false,
  })
}

export function useWorkspaceTeamMembersQueries(
  teamIds: readonly number[],
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const ids = [...new Set(teamIds.filter((id) => Number.isInteger(id) && id > 0))]
  const valid = enabled && Number.isInteger(workspaceId) && workspaceId > 0
  return useQueries({
    queries: ids.map((teamId) => ({
      ...workspaceTeamMembersQueryOptions(valid ? workspaceId : 1, teamId),
      enabled: valid,
    })),
  })
}
