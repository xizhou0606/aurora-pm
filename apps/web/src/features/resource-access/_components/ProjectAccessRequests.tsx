import { Button, EmptyState, FormField, Textarea, Popover, PopoverTrigger, PopoverContent, PopoverTitle } from '@/shared/ui'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'
import { ACCESS_REQUESTS_COPY } from '@/shared/copy'
import { isSafeApiError } from '@/shared/api'
import { useProjectRequests } from '../hooks/useProjectRequests'
import type { JoinTarget } from '../api/project-requests'
import { useJoinRequestOpen } from '../hooks/useJoinRequestOpen'

const copy = RESOURCE_ACCESS_COPY.projectRequests

export function ProjectAccessRequests({ target, inline = false, label, prominent = false }: {
  target: JoinTarget; inline?: boolean; label?: string; prominent?: boolean
}) {
  const popup = useJoinRequestOpen(target)
  const title = target.kind === 'workspace' ? copy.workspaceTitle : copy.title
  if (inline) return <ProjectRequestBody key={`${target.kind}:${target.id}`} target={target} />
  return (
    <Popover {...popup}>
      <PopoverTrigger render={<Button variant={prominent ? 'default' : 'outline'} size="sm" />}>{label ?? title}</PopoverTrigger>
      <PopoverContent className="w-80 max-w-[calc(100vw-2rem)] p-4">
        <PopoverTitle className="text-sm font-semibold">{title}</PopoverTitle>
        <ProjectRequestBody target={target} onClose={() => popup.onOpenChange(false)} />
      </PopoverContent>
    </Popover>
  )
}

function ProjectRequestBody({ target, onClose }: { target: JoinTarget; onClose?: () => void }) {
  const flow = useProjectRequests(target)
  if (!flow.data) return <div className="grid gap-3 pt-3 text-sm"><p className="text-muted-foreground">{flow.loading ? copy.loading : copy.failed}</p>{!flow.loading ? <Button size="sm" onClick={flow.retry}>{copy.retry}</Button> : null}</div>
  const data = flow.data
  const inputId = `project-request-${target.kind}-${target.id}`
  const error = isSafeApiError(flow.error) && flow.error.code === 'TARGET_WORKSPACE_MEMBERSHIP_REQUIRED' ? copy.workspaceRequired : copy.failed
  return <div className="grid gap-4 pt-3 text-sm" id="access-requests">
    {flow.error ? <p role="alert" className="text-destructive">{error}</p> : null}
    {target.kind !== 'workspace' && !data.workspace_member && !flow.canReview ? <p className="leading-relaxed text-muted-foreground">{copy.workspaceRequired}</p> : null}
    {data.has_access && !flow.canReview ? <p className="leading-relaxed text-muted-foreground">{target.kind === 'workspace' ? copy.workspaceHasAccess : copy.hasAccess}</p> : null}
    {!data.can_request && !data.has_access && (target.kind === 'workspace' || data.workspace_member) && data.items.length === 0 ? <p className="text-muted-foreground">{copy.noApprover}</p> : null}
    {data.can_request ? <>
      <p className="text-xs leading-relaxed text-muted-foreground">{target.kind === 'workspace' ? copy.workspaceScope : copy.scope}</p>
      <FormField id={inputId} label={copy.reason}>
        {(field) => <Textarea {...field} className="min-h-24 max-h-48 resize-y text-sm" rows={3} placeholder={copy.reasonPlaceholder} value={flow.reason} onChange={event => flow.setReason(event.target.value)} maxLength={1000} disabled={flow.busy} />}
      </FormField>
      <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
        <span className="text-xs text-muted-foreground">{copy.expiry}</span>
        <div className="flex gap-2">
          {onClose ? <Button variant="ghost" size="sm" disabled={flow.busy} onClick={onClose}>{copy.cancel}</Button> : null}
          <Button size="sm" disabled={flow.busy || !flow.reason.trim()} onClick={flow.submit}>{flow.busy ? copy.submitting : copy.submit}</Button>
        </div>
      </div>
    </> : null}
    {data.items.length > 0 ? <div className="grid max-h-80 divide-y divide-border overflow-y-auto">
      {data.items.map(item => <article key={item.id} className="grid gap-2 py-3 first:pt-0 last:pb-0">
        <div className="flex justify-between gap-2"><span>{item.applicant_name}</span><span className="text-xs text-muted-foreground">{ACCESS_REQUESTS_COPY.statuses[item.status]}</span></div>
        <p className="whitespace-pre-wrap break-words">{item.reason}</p>
        {flow.canReview && item.status === 'pending' ? <div className="flex items-center gap-3"><Button variant="link" size="sm" className="h-auto p-0" disabled={flow.busy} onClick={() => flow.review(item.id, 'rejected')}>{copy.reject}</Button><Button variant="link" size="sm" className="h-auto p-0" disabled={flow.busy} onClick={() => flow.review(item.id, 'approved')}>{copy.approve}</Button></div> : null}
      </article>)}
    </div> : null}
    {flow.canReview && data.items.length === 0 ? <EmptyState title={copy.empty} /> : null}
    {data.has_more ? <p className="text-xs text-muted-foreground">{copy.more}</p> : null}
  </div>
}
