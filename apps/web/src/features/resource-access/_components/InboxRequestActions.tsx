import type { ReactNode } from 'react'
import { RequestGrantFields } from './RequestGrantFields'
import { Button } from '@/shared/ui'
import { NOTIFICATIONS_COPY as copy } from '@/shared/copy/notifications'
import { useProjectRequests } from '../hooks/useProjectRequests'
import { useAccessRequestsWorkflow } from '../hooks/useAccessRequestsWorkflow'
import type { JoinTarget } from '../api/project-requests'
import type { AccessRequestResourceType } from '../model/requests'

function ReviewLinks({ busy, error, review, children }: { busy: boolean; error: boolean; review: (decision: 'approved' | 'rejected') => void; children?: ReactNode }) {
 return <div className="space-y-2 px-3 pb-3">
  {children}
  <div className="flex items-center gap-3">
   <Button variant="link" size="sm" className="h-auto p-0" disabled={busy} onClick={() => review('approved')}>{copy.approve}</Button>
   <Button variant="link" size="sm" className="h-auto p-0" disabled={busy} onClick={() => review('rejected')}>{copy.reject}</Button>
  </div>
  {error ? <p role="alert" className="text-xs text-destructive">{copy.reviewFailed}</p> : null}
 </div>
}
function RequestFeedback({ error, status, retry }: { error: boolean; status?: string; retry: () => void }) {
 if (error) return <div role="alert" className="px-3 pb-3 text-xs text-destructive">{copy.reviewFailed}<Button variant="link" size="sm" onClick={retry}>{copy.retry}</Button></div>
 if (status && status in copy.requestStatuses) return <p className="px-3 pb-3 text-xs text-muted-foreground">{copy.requestStatuses[status as keyof typeof copy.requestStatuses]}</p>
 return null
}
export function InboxMembershipActions({ target, requestId }: { target: JoinTarget; requestId: number }) {
 const flow = useProjectRequests(target)
 const request = flow.data?.items.find(item => item.id === requestId)
 if (!flow.canReview || request?.status !== 'pending') return <RequestFeedback error={Boolean(flow.error)} status={request?.status} retry={flow.retry} />
 return <ReviewLinks busy={flow.busy} error={Boolean(flow.error)} review={decision => flow.review(requestId, decision)} />
}
export function InboxDocumentActions({ resourceId, resourceType, requestId }: { resourceId: string; resourceType: AccessRequestResourceType; requestId: number }) {
 const flow = useAccessRequestsWorkflow(resourceId, false, undefined, resourceType)
 const request = flow.data?.items.find(item => item.id === requestId)
 if (!flow.canReview || request?.status !== 'pending') return <RequestFeedback error={Boolean(flow.error)} status={request?.status} retry={flow.retry} />
 return <ReviewLinks busy={flow.busy} error={flow.error} review={decision => { void flow.review({ id: requestId, decision }) }}>
  <RequestGrantFields resourceType={resourceType} terms={flow.grantTerms(requestId)} busy={flow.busy} change={patch => flow.setGrantTerms(requestId, patch)} />
 </ReviewLinks>
}
