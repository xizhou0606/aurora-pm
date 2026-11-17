import { useState } from 'react'
import { Button, Popover, PopoverContent, PopoverTitle, PopoverTrigger } from '@/shared/ui'
import { NOTIFICATIONS_COPY as copy } from '@/shared/copy/notifications'
import { InboxMessages } from './InboxMessages'
import { useMentions } from './useMentions'
import type { NotificationCategory } from './api'
const categories = [{value:'',label:copy.all},{value:'mention',label:copy.mentions},{value:'message',label:copy.messages},{value:'access',label:copy.access}] as const
export function MentionNotifications() {
 const [open,setOpen] = useState(false)
 const [category,setCategory] = useState<NotificationCategory>('')
 const query = useMentions(true)
 const unread = query.data?.pages[0]?.unread_count ?? 0
 return <Popover open={open} onOpenChange={setOpen}>
  <PopoverTrigger aria-label={copy.title} className="inline-flex h-8 items-center justify-center gap-1 rounded-md px-2 text-sm text-muted-foreground hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
   <span aria-hidden="true">{copy.symbol}</span>{unread > 0 && <span>{unread > 99 ? '99+' : unread}</span>}
  </PopoverTrigger>
  <PopoverContent align="end" className="w-96 max-w-[calc(100vw-2rem)]">
   <PopoverTitle className="p-3">{copy.title}</PopoverTitle>
   <div className="flex flex-wrap gap-1 px-3" role="group" aria-label={copy.title}>{categories.map(item=><Button key={item.value} variant={category===item.value ? 'secondary':'ghost'} size="sm" aria-pressed={category===item.value} onClick={()=>setCategory(item.value)}>{item.label}</Button>)}</div>
   <div className="h-80 overflow-y-auto"><InboxMessages category={category} onOpen={()=>setOpen(false)} /></div>
  </PopoverContent>
 </Popover>
}
