import { useQuery } from '@tanstack/react-query'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { pmKeys } from '@/shared/query'
import { isSafeApiError } from '@/shared/api'
import { getProjectGuide, type ProjectGuideTarget } from '../api/project-guide'
export function useProjectAccessGuide(target: ProjectGuideTarget) {
  const workspaceId = useWorkspaceStore( s => s.currentWorkspaceId)
  const revision = useSessionStore( s => s.revision)
  const authenticated = useSessionStore( s => Boolean(s.token))
  const query = useQuery({
    queryKey: workspaceId > 0 ? pmKeys.resource(workspaceId, 'projectSets', 'access-guide', revision, target.kind, target.id) : [...pmKeys.session(), 'project-guide-disabled', revision],
    queryFn: ({ signal }) => getProjectGuide(target, signal),
    enabled: authenticated && workspaceId > 0 && target.id > 0,
    retry: false,
  })
  return {
    data: query.isError ? undefined : query.data,
    status: query.isError ? isSafeApiError(query.error) && query.error.status === 404 ? 'missing' : 'error' : query.data ? 'ready' : 'loading',
    retry: () => { void query.refetch() },
  } as const
}
