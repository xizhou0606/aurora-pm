import { Popover, PopoverContent, UserPreview, LoadingSkeleton } from '@/shared/ui'
import { useMentionPreview } from './useMentionPreview'
import { MENTION_COPY } from './copy'

export function UserMentionPreview({workspaceId}: {workspaceId:number}) {
  const {anchor,close,user,loading,loadingVisible,label} = useMentionPreview(workspaceId)
  if (!anchor) return null
  return <Popover open onOpenChange={open=>{if(!open) close()}}>
    <PopoverContent anchor={anchor} container={anchor.closest<HTMLElement>('[role="dialog"]') || undefined} side="top" sideOffset={8} className="z-[100] w-72 p-4" initialFocus={false} finalFocus={false} data-user-mention-card="" aria-label={MENTION_COPY.title}>
      <UserPreview name={user?.display_name || label} avatarUrl={user?.avatar_url} size="md" />
      {user ? <dl className="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
        <dt className="text-muted-foreground">{MENTION_COPY.username}</dt><dd className="m-0 break-all">{user.username || MENTION_COPY.missing}</dd>
        <dt className="text-muted-foreground">{MENTION_COPY.email}</dt><dd className="m-0 break-all">{user.email || MENTION_COPY.missing}</dd>
      </dl> : loading ? (loadingVisible ? <LoadingSkeleton rows={2} label={MENTION_COPY.loading} /> : <div className="h-16" aria-busy="true" />) : <p role="status" className="mt-2 text-xs text-muted-foreground">{MENTION_COPY.unavailable}</p>}
    </PopoverContent>
  </Popover>
}
