import { useDelayedInboxLoading } from './useDelayedInboxLoading'
import { RequestActions } from './RequestActions'
import { ProjectAccessRequests } from '@/features/resource-access'
import { useMembershipRequestTarget } from './useMembershipRequestTarget'
import { Button, EmptyState, StatusPill } from '@/shared/ui'
import { NOTIFICATIONS_COPY as copy } from '@/shared/copy/notifications'
import { useMentions, useNotificationRead } from './useMentions'
import { notificationPresentation } from './presentation'
import type { NotificationCategory } from './api'
export function InboxMessages({category = '', onOpen}: {category?:NotificationCategory; onOpen?:()=>void}) {
 const query = useMentions(true, category)
 const showLoading = useDelayedInboxLoading(query.isPending, category)
 const membershipTarget = useMembershipRequestTarget()
 const read = useNotificationRead()
 const page = query.data?.pages[0]
 const unread = category ? page?.unread_by_category?.[category] ?? 0 : page?.unread_count ?? 0
 const items = query.data?.pages.flatMap(page => page.items) ?? []
 return <div aria-busy={query.isPending || undefined}>
  {membershipTarget ? <div className="p-3"><ProjectAccessRequests key={`${membershipTarget.kind}:${membershipTarget.id}`} target={membershipTarget} /></div> : null}
  <div className="flex items-center justify-between border-b border-border p-3 text-xs">
   <span>{copy.unread} {query.isPending ? copy.unreadUnknown : unread}</span>
   <Button size="sm" variant="ghost" disabled={query.isPending || !page?.high_watermark || !unread || read.isPending} onClick={()=>read.mutate({id:page?.high_watermark ?? 0, all:true, category})}>{copy.readAll}</Button>
  </div>
  {read.isError && <p role="alert" className="p-3 text-sm text-destructive">{copy.updateError}</p>}
  {query.isError && <EmptyState title={copy.error} action={<Button variant="ghost" onClick={()=>void query.refetch()}>{copy.retry}</Button>} />}
  {showLoading && <p role="status" className="p-3 text-sm">{copy.loading}</p>}
  {!query.isPending && !query.isError && !items.length ? <EmptyState title={copy.empty} /> : null}
  <ul className="divide-y divide-border">{items.map(item => {
   const display = notificationPresentation(item)
   if (!display) return null
   const approval = display.approval
   const content = <>
    <p className={item.read_at ? 'line-clamp-2' : 'line-clamp-2 pr-12'}>{display.title}</p>
    {display.resourceTitle && <p className="line-clamp-2" title={display.resourceTitle}>{display.resourceTitle}</p>}
    {display.detail && <p className="text-xs text-muted-foreground">{display.detail}</p>}
    <time className="text-xs text-muted-foreground" dateTime={item.occurred_at}>{new Date(item.occurred_at).toLocaleString()}</time>
   </>
   const className = `block w-full space-y-1 p-3 text-left text-sm transition-control outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 ${item.read_at ? 'text-muted-foreground' : 'font-medium'}`
   return <li key={item.id} className="relative">{!item.read_at && <StatusPill tone="info" className="pointer-events-none absolute right-3 top-3 min-w-0 px-1.5 py-0">{copy.unread}</StatusPill>}{approval ? <button type="button" className={className} disabled={read.isPending} onClick={()=>{ if (!item.read_at) read.mutate({id:item.id}) }}>{content}</button> : <a href={display.href} className={className} onClick={event=>{
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    event.preventDefault()
    read.mutate({id:item.id}, {onSuccess:()=>{onOpen?.(); window.location.assign(display.href)}})
   }}>{content}</a>}<RequestActions item={item} /></li>
  })}</ul>
  {query.hasNextPage && <Button variant="ghost" className="w-full" disabled={query.isFetchingNextPage} onClick={()=>void query.fetchNextPage()}>{copy.more}</Button>}
 </div>
}
