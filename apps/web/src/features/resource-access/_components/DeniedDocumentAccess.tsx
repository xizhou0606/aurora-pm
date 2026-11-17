import type { ReactNode } from 'react'
import { accessRequestsCopy } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton } from '@/shared/ui'
import { useDeniedDocumentAccess, type DeniedResourceTarget } from '../hooks/useDeniedDocumentAccess'
import { ResourceAccessRequestsToolbar } from './ResourceAccessRequestsToolbar'

export type DeniedDocumentAccessProps = DeniedResourceTarget & { fallback?: ReactNode }

export function DeniedDocumentAccess({ fallback, ...target }: DeniedDocumentAccessProps) {
  const copy = accessRequestsCopy(target.resourceType)
  const state = useDeniedDocumentAccess(target)
  if (state.status === 'fallback') return fallback ?? null
  if (state.status === 'loading') return <LoadingSkeleton rows={3} label={copy.loading} />
  if (state.status === 'error' || !state.data) return <EmptyState title={copy.unavailable} action={<Button variant="outline" onClick={state.retry}>{copy.retry}</Button>} />
  const pending = state.latestRequest?.status === 'pending'
  return <div className="flex h-full items-center justify-center p-6 text-center">
    <div className="flex w-full max-w-md flex-col items-center gap-3">
      <EmptyState className="flex-none" title={pending ? copy.waiting : copy.denied} />
      <p className="max-w-md text-sm text-muted-foreground">{pending ? copy.waitingHint : !state.canRequest ? copy.noRequestPermission : state.latestRequest?.status === 'expired' ? copy.expiredHint : state.latestRequest?.status === 'rejected' ? copy.rejectedHint : copy.deniedHint}</p>
      <p className="text-sm text-muted-foreground">{state.data.ownerUserId === 0 ? copy.ownerMissing : <>{copy.owner}<span className="ml-2 text-foreground">{state.data.ownerName}</span></>}</p>
      {!pending && state.canRequest ? <ResourceAccessRequestsToolbar resourceId={state.data.resourceId} resourceType={target.resourceType} canEdit={false} triggerVariant="primary" /> : null}
    </div>
  </div>
}
