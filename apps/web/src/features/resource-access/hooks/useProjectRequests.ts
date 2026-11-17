import { useActionPermissions, useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime/state'
import { pmKeys } from '@/shared/query'
import { getProjectRequests, mutateProjectRequest } from '../api/project-requests'
import type { JoinTarget } from '../api/project-requests'
export function useProjectRequests(target: JoinTarget) {
  const workspaceId = useWorkspaceStore( s => s.currentWorkspaceId)
  const revision = useSessionStore( s => s.revision)
  const authenticated = useSessionStore( s => Boolean(s.token))
  const projectOperations = useOperationCapabilities('project-management')
  const permissions = useActionPermissions()
  const [reason, setReason] = useState('')
  const cache = useQueryClient()
  const key = workspaceId > 0 ? pmKeys.resource(workspaceId, 'projectSets', 'join-requests', revision, target.kind, target.id) : [...pmKeys.session(), 'join-requests-disabled', revision]
  const query = useQuery({ queryKey: key, queryFn: ({ signal }) => getProjectRequests(target, signal), enabled: authenticated && workspaceId > 0 && target.id > 0, retry: false, refetchInterval: 15000 })
  const canReview = !query.isError && Boolean(query.data?.can_review) && (target.kind === 'workspace' ? permissions.canAction(PM_ACTION.AccessManagementWorkspacesMembersCreate) : projectOperations.allows(PM_ACTION.ProjectsParticipantsManage))
  const mutation = useMutation({ mutationFn: (input: { reason: string } | { id: number; decision: 'approved' | 'rejected' }) => {
 if (runtimeSessionStore.getState().revision !== revision || runtimeWorkspaceStore.getState().currentWorkspaceId !== workspaceId || !authenticated) return Promise.reject(new Error('SESSION_CHANGED'))
 if ('decision' in input && !canReview) return Promise.reject(new Error('ACCESS_REQUEST_REVIEW_FORBIDDEN'))
 if ('reason' in input && (query.isError || !query.data?.can_request)) return Promise.reject(new Error('ACCESS_REQUEST_UNAVAILABLE'))
 return mutateProjectRequest(target, input)
 },
    onSuccess: async () => { setReason(''); await cache.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'notifications') }); await cache.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'projectSets') }) },
  })
  return { canReview, data: query.isError ? undefined : query.data, loading: query.isPending, error: query.error || mutation.error, busy: mutation.isPending,
    reason, setReason, retry: () => { void query.refetch() },
    submit: () => mutation.mutate({ reason }), review: (id: number, decision: 'approved' | 'rejected') => mutation.mutate({ id, decision }),
  }
}
