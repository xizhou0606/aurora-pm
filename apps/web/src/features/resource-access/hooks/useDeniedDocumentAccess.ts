import { useEffect } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { PMSafeApiError, canRetainStaleApiData } from '@/shared/api'
import { pmKeys } from '@/shared/query'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { accessRequestsClient } from '../api/requests'
import { isCurrentRequestScope } from './useAccessRequests'
import { resourceAccessKeys } from './keys'

export type DeniedResourceTarget = { documentId: number; resourceType?: 'rich_text' } | { bitableId: string; resourceType: 'bitable' }

export function useDeniedDocumentAccess(target: number | DeniedResourceTarget) {
  const documentId = typeof target === 'number' ? target : target.resourceType === 'bitable' ? 0 : target.documentId
  const bitableId = typeof target !== 'number' && target.resourceType === 'bitable' ? target.bitableId : ''
  const resourceType = bitableId ? 'bitable' : 'rich_text'
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId) ?? 0
  const sessionRevision = useSessionStore((state) => state.revision)
  const scope = { workspaceId, sessionRevision, resourceId: bitableId ? `bitable-${bitableId}` : `document-${documentId}` }
  const capabilities = useOperationCapabilities('documents')
  const enabled = workspaceId > 0 && (documentId > 0 || Boolean(bitableId)) && capabilities.allows(PM_ACTION.DocumentsAccessView)
  const cache = useQueryClient()
  const query = useQuery({
    queryKey: [...resourceAccessKeys(scope).root, 'denied-context'],
    queryFn: async ({ signal }) => {
      if (!isCurrentRequestScope(scope)) throw new PMSafeApiError('RESOURCE_ACCESS_SCOPE_CHANGED')
      const result = bitableId
        ? await accessRequestsClient.bitableContext(bitableId, signal)
        : await accessRequestsClient.documentContext(documentId, signal)
      if (result.resourceType !== resourceType) throw new PMSafeApiError('RESOURCE_ACCESS_SCOPE_CHANGED', 403)
      if (!isCurrentRequestScope(scope)) throw new PMSafeApiError('RESOURCE_ACCESS_SCOPE_CHANGED')
      return result
    },
    enabled, retry: false, refetchInterval: 30_000,
  })
  const data = !query.error || canRetainStaleApiData(query.error) ? query.data : undefined
  const latestRequest = data?.items[0]
  useEffect(() => {
    if (enabled && data?.hasAccess) void cache.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, bitableId ? 'bitables' : 'documents') })
  }, [enabled, latestRequest?.id, latestRequest?.status, data?.hasAccess, workspaceId, bitableId, cache])
  const missing = query.error instanceof PMSafeApiError && [401, 403, 404].includes(query.error.status ?? 0)
  const status = !enabled || data?.hasAccess || missing ? 'fallback'
    : !data ? query.isError ? 'error' : 'loading' : 'ready'
  return { status, data, resourceType, latestRequest, canRequest: Boolean(data?.canRequest && capabilities.allows(PM_ACTION.DocumentsAccessRequest)), retry: () => { void query.refetch() } } as const
}
