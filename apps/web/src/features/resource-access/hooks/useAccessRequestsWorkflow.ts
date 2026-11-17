import { useEffect, useState } from 'react'
import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { canRetainStaleApiData } from '@/shared/api'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { isCurrentRequestScope, useAccessRequestsQuery, useCreateAccessRequest, useUpdateAccessRequest } from './useAccessRequests'
import { useQueryClient } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import type { AccessRequestUpdate, AccessRequestResourceType, RequestGrantTerms } from '../model/requests'

export function useAccessRequestsWorkflow(resourceId: string, canEdit: boolean, onAccessChanged?: () => void, resourceType: AccessRequestResourceType = 'rich_text') {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId) ?? 0
  const sessionRevision = useSessionStore((state) => state.revision)
  const operations = useOperationCapabilities('documents')
  const canViewRequests = operations.allows(PM_ACTION.DocumentsAccessView)
  const scope = { workspaceId, sessionRevision, resourceId, resourceType }
  const cache = useQueryClient()
  const query = useAccessRequestsQuery(scope, operations.allows(PM_ACTION.DocumentsAccessView))
  const create = useCreateAccessRequest(scope)
  const update = useUpdateAccessRequest(scope)
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const [notifyOwner, setNotifyOwner] = useState(true)
  const [read, setRead] = useState(false)
  const [terms, setTerms] = useState<Record<number, RequestGrantTerms>>({})
  const grantTerms = (id: number): RequestGrantTerms => terms[id] ?? { grantLevel: query.data?.items.find(item => item.id === id)?.grantLevel ?? 'editor', grantDuration: 'never' }
  const setGrantTerms = (id: number, patch: Partial<RequestGrantTerms>) => setTerms((current) => ({ ...current, [id]: { ...(current[id] ?? { grantLevel: query.data?.items.find(item => item.id === id)?.grantLevel ?? 'editor', grantDuration: 'never' }), ...patch } }))
  useEffect(() => { setOpen(false); setReason(''); setNotifyOwner(true); setRead(false); setTerms({}) }, [resourceId, resourceType, workspaceId, sessionRevision])
  const data = !query.error || canRetainStaleApiData(query.error) ? query.data : undefined
  const canReview = !query.isError && Boolean(data?.canReview && operations.allows(PM_ACTION.DocumentsAccessManage))
  const canRequest = Boolean(data?.canRequest && !canEdit && operations.allows(PM_ACTION.DocumentsAccessRequest))
  const busy = create.isPending || update.isPending
  const items = data?.items.filter((item) => item.read === read) ?? []
  const latestRequest = !canReview ? data?.items[0] : undefined
  useEffect(() => {
    if (latestRequest?.status === 'approved' && !canEdit && workspaceId > 0) void cache.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, resourceType === 'bitable' ? 'bitables' : 'documents') })
  }, [latestRequest?.id, latestRequest?.status, canEdit, workspaceId, resourceType, cache])
  const status = !data ? query.isError ? 'error' : 'loading' : 'ready'
  useEffect(() => {
    const sync = () => { if (window.location.hash === '#access-requests') setOpen(true) }
    sync()
    window.addEventListener('hashchange', sync)
    return () => window.removeEventListener('hashchange', sync)
  }, [resourceId])
  const submit = async () => {
    if (!canRequest || busy) return
    try { await create.mutateAsync({ reason, notifyOwner }); if (isCurrentRequestScope(scope)) { setOpen(false); setReason('') } } catch { /* mutation owns safe error */ }
  }
  const review = async (input: AccessRequestUpdate) => {
    if (!canReview || busy) return
    try { await update.mutateAsync(input.decision === 'approved' ? { ...input, ...grantTerms(input.id) } : input); if (input.decision && isCurrentRequestScope(scope)) onAccessChanged?.() } catch { /* mutation owns safe error */ }
  }
  return { status, grantTerms, setGrantTerms, visible: canViewRequests && (Boolean(data && (canReview || canRequest || latestRequest)) || status === 'error'), latestRequest, canReview, canRequest, data, items, busy, error: query.isError || create.isError || update.isError, open, setOpen, reason, setReason, notifyOwner, setNotifyOwner, read, setRead, submit, review, retry: () => { void query.refetch() } }
}
