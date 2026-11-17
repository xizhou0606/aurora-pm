import { queryOptions, useQuery } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'
import { listScrumProjectNewsNew } from '../api'

function positiveProjectId(value: unknown): number {
  const projectId = Number(value || 0)
  if (!Number.isInteger(projectId) || projectId <= 0) {
    throw new Error('PM project news query requires a positive project ID')
  }
  return projectId
}

export function latestProjectNewsQueryOptions(
  workspaceId: number,
  projectId: number,
) {
  const id = positiveProjectId(projectId)
  return queryOptions({
    queryKey: pmKeys.resource(
      workspaceId,
      'projectSets',
      'project',
      id,
      'news',
      'latest',
    ),
    queryFn: async ({ signal }) => (
      (await listScrumProjectNewsNew(id, signal))[0] ?? null
    ),
    staleTime: 30_000,
    retry: false,
    retryOnMount: true,
    refetchOnMount: true,
  })
}

export function useLatestProjectNewsQuery(
  projectId: number,
  enabled = true,
) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const valid = enabled
    && Number.isInteger(workspaceId)
    && workspaceId > 0
    && Number.isInteger(projectId)
    && projectId > 0
  return useQuery({
    ...latestProjectNewsQueryOptions(
      valid ? workspaceId : 1,
      valid ? projectId : 1,
    ),
    enabled: valid,
  })
}
