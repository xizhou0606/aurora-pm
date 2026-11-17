import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { accessRequestsClient } from '../api/requests'
import type { AccessRequestUpdate, AccessRequestResourceType } from '../model/requests'
import { resourceAccessKeys } from './keys'
import { pmKeys } from '@/shared/query'
import { PMSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'

export type RequestScope = { workspaceId: number; sessionRevision: number; resourceId: string; resourceType?: AccessRequestResourceType }
export function isCurrentRequestScope(scope: RequestScope) {
  return runtimeWorkspaceStore.getState().currentWorkspaceId === scope.workspaceId && runtimeSessionStore.getState().revision === scope.sessionRevision
}
function assertScope(scope: RequestScope) { if (!isCurrentRequestScope(scope)) throw new PMSafeApiError('RESOURCE_ACCESS_SCOPE_CHANGED') }
export const requestKey = (scope: RequestScope) => [...resourceAccessKeys(scope).root, 'requests'] as const

export function useAccessRequestsQuery(scope: RequestScope, enabled: boolean) {
  return useQuery({ queryKey: [...requestKey(scope), scope.resourceType ?? 'rich_text'], queryFn: async ({ signal }) => { assertScope(scope); const result = await accessRequestsClient.context(scope.resourceId, signal, scope.resourceType); assertScope(scope); return result }, enabled: enabled && scope.workspaceId > 0 && Boolean(scope.resourceId), retry: false, refetchInterval: 30_000 })
}
export function useCreateAccessRequest(scope: RequestScope) {
  const cache = useQueryClient()
  return useMutation({ mutationFn: (input: { reason: string; notifyOwner: boolean }) => { assertScope(scope); return accessRequestsClient.create(scope.resourceId, input) }, onSuccess: async () => { await cache.invalidateQueries({ queryKey: pmKeys.resource(scope.workspaceId, 'notifications') }); await cache.invalidateQueries({ queryKey: requestKey(scope) }); await cache.invalidateQueries({ queryKey: pmKeys.resource(scope.workspaceId, 'documents') }); await cache.invalidateQueries({ queryKey: pmKeys.resource(scope.workspaceId, 'bitables') }) } })
}
export function useUpdateAccessRequest(scope: RequestScope) {
  const cache = useQueryClient()
  return useMutation({ mutationFn: (input: AccessRequestUpdate) => { assertScope(scope); return accessRequestsClient.update(scope.resourceId, input) }, onSuccess: async () => { await cache.invalidateQueries({ queryKey: pmKeys.resource(scope.workspaceId, 'notifications') }); await cache.invalidateQueries({ queryKey: resourceAccessKeys(scope).root }); await cache.invalidateQueries({ queryKey: pmKeys.resource(scope.workspaceId, 'documents') }); await cache.invalidateQueries({ queryKey: pmKeys.resource(scope.workspaceId, 'bitables') }) } })
}
