import { RequestGrantFields } from './RequestGrantFields'
import { accessRequestsCopy, RESOURCE_ACCESS_COPY as accessCopy } from '@/shared/copy'
import { AlertBanner, Button, Checkbox, EmptyState, FormField, Popover, PopoverContent, PopoverTitle, PopoverTrigger, Textarea } from '@/shared/ui'
import { useAccessRequestsWorkflow } from '../hooks/useAccessRequestsWorkflow'
import type { AccessRequestResourceType } from '../model/requests'

export type ResourceAccessRequestsToolbarProps = { resourceId: string; resourceType?: AccessRequestResourceType; canEdit: boolean; onAccessChanged?: () => void; triggerVariant?: 'primary' | 'ghost' }

export function ResourceAccessRequestsToolbar({ resourceId, resourceType = 'rich_text', canEdit, onAccessChanged, triggerVariant = 'ghost' }: ResourceAccessRequestsToolbarProps) {
  const control = useAccessRequestsWorkflow(resourceId, canEdit, onAccessChanged, resourceType)
  const copy = accessRequestsCopy(resourceType)
  if (!control.visible) return null
  if (!control.data) return <Button size="sm" variant="ghost" title={copy.error} onClick={control.retry}>{copy.review}{copy.separator}{copy.retry}</Button>
  return (
    <Popover open={control.open} onOpenChange={control.setOpen}>
      <PopoverTrigger render={<Button size="sm" variant={triggerVariant === 'primary' ? 'default' : 'ghost'} />}>
        {control.canReview ? copy.pendingCount(control.data.pendingCount) : control.canRequest ? copy.request : control.latestRequest ? copy.statuses[control.latestRequest.status] : copy.request}
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-3">
        <div className="grid gap-3">
          <PopoverTitle>{control.canReview ? copy.review : copy.title}</PopoverTitle>
          {control.error ? <AlertBanner tone="error">{copy.error}<Button size="sm" variant="ghost" onClick={control.retry}>{copy.retry}</Button></AlertBanner> : null}
          {control.canReview ? <>
            <div className="flex gap-1" role="group" aria-label={copy.review}>
              <Button size="sm" variant={!control.read ? 'secondary' : 'ghost'} aria-pressed={!control.read} onClick={() => control.setRead(false)}>{copy.unread}</Button>
              <Button size="sm" variant={control.read ? 'secondary' : 'ghost'} aria-pressed={control.read} onClick={() => control.setRead(true)}>{copy.read}</Button>
            </div>
            <div className="max-h-80 divide-y divide-border overflow-y-auto">
              {control.items.length === 0 ? <EmptyState title={copy.empty} /> : control.items.map((item) => (
                <div key={item.id} className="grid gap-2 py-3 text-sm first:pt-0 last:pb-0">
                  <div className="flex items-center justify-between gap-2"><span className="truncate font-medium">{item.applicantName}</span><span className="shrink-0 text-xs text-muted-foreground">{copy.statuses[item.status]}</span></div>
                  <p className="whitespace-pre-wrap break-words text-muted-foreground">{item.reason}</p>
                  {item.status === 'pending' ? <RequestGrantFields resourceType={resourceType} terms={control.grantTerms(item.id)} busy={control.busy} change={patch => control.setGrantTerms(item.id, patch)} /> : item.status === 'approved' ? <p className="text-xs text-muted-foreground">{accessCopy.level[item.grantLevel]}{accessCopy.separator}{item.grantExpiresAt ? <time dateTime={item.grantExpiresAt}>{new Date(item.grantExpiresAt).toLocaleString()}</time> : accessCopy.links.expiryNever}</p> : null}
                  <div className="flex flex-wrap items-center gap-3">
                    {item.status === 'approved' ? <Button size="sm" variant="outline" disabled={control.busy} onClick={() => { void control.review({ id: item.id, decision: 'revoked' }) }}>{copy.revoke}</Button> : null}
                    {item.status === 'pending' ? <><Button variant="link" className="h-auto p-0" size="sm" disabled={control.busy} onClick={() => { void control.review({ id: item.id, decision: 'approved' }) }}>{copy.approve}</Button><Button size="sm" variant="link" className="h-auto p-0" disabled={control.busy} onClick={() => { void control.review({ id: item.id, decision: 'rejected' }) }}>{copy.reject}</Button></> : null}
                    <Button size="sm" variant="ghost" disabled={control.busy} onClick={() => { void control.review({ id: item.id, read: !item.read }) }}>{item.read ? copy.markUnread : copy.markRead}</Button>
                  </div>
                </div>
              ))}
            </div>
            {control.data.hasMore ? <p className="text-xs text-muted-foreground">{copy.more}</p> : null}
          </> : <>
            <div className="text-sm"><span className="text-muted-foreground">{copy.owner}</span><span className="ml-2 font-medium">{control.data.ownerName}</span></div>
            {control.latestRequest ? <p className="text-sm text-muted-foreground">{copy.statuses[control.latestRequest.status]}</p> : null}
            {control.latestRequest?.status === 'pending' ? <p className="text-sm text-muted-foreground">{copy.waitingHint}</p> : null}
            {control.canRequest ? <>
              <p className="text-xs text-muted-foreground">{copy.validity}</p>
              <FormField id={`access-reason-${resourceId}`} label={copy.reason}>{(field) => <Textarea {...field} value={control.reason} onChange={(event) => control.setReason(event.target.value)} maxLength={1000} placeholder={copy.placeholder} disabled={control.busy} />}</FormField>
              <label className="flex items-center gap-2 text-sm"><Checkbox checked={control.notifyOwner} onCheckedChange={(value) => control.setNotifyOwner(value === true)} disabled={control.busy} />{copy.notify}</label>
              <Button size="sm" disabled={control.busy || !control.reason.trim()} onClick={() => { void control.submit() }}>{control.busy ? copy.submitting : copy.submit}</Button>
            </> : null}
          </>}
        </div>
      </PopoverContent>
    </Popover>
  )
}
