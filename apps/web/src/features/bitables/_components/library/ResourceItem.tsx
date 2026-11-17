import { useRef } from 'react'
import { BITABLE_LIBRARY_COPY as copy } from '@/shared/copy/bitables-library'
import { cn } from '@/shared/lib/utils'
import { Button, ContextMenu, ContextMenuTrigger, ResourceContextMenu } from '@/shared/ui'
import { PhArrowSquareOut, PhCopy, PhDotsThree, PhFolderPlus, PhLink, PhPencil, PhPushPin, PhShareNetwork, PhStack, PhStar, PhTable, PhTrash, PhUserGear } from '@/shared/ui/icons'
import type { ResourceDirectoryItem, ResourceMenuAction } from '../../model/library'

const presentation = {
  open: [copy.open, PhArrowSquareOut], share: [copy.share, PhShareNetwork], copyLink: [copy.copyLink, PhLink],
  permissions: [copy.permissions, PhUserGear],
  duplicate: [copy.duplicate, PhCopy], shortcuts: [copy.shortcut, PhStack], pin: [copy.pin, PhPushPin], favorite: [copy.favorite, PhStar],
  owner: [copy.owner, PhUserGear], location: [copy.move, PhFolderPlus], name: [copy.rename, PhPencil], delete: [copy.remove, PhTrash],
} as const

export function ResourceItem({ item, selected, depth, pending, onSelect, onAction }: {
  item: ResourceDirectoryItem; selected: boolean; depth: number; pending: boolean
  onSelect: (targetId: string) => void; onAction: (id: string, action: ResourceMenuAction) => void
}) {
  const anchor = useRef<HTMLDivElement>(null)
  const indent = ['pl-1', 'pl-4', 'pl-7', 'pl-10', 'pl-13', 'pl-16'][Math.min(5, depth)]
  const className = cn(
    'group flex min-w-0 items-center gap-1 rounded-md transition-colors hover:bg-muted/70 focus-within:bg-muted/70',
    indent,
    selected && 'bg-secondary',
  )
  return <ContextMenu onOpenChange={(open, details) => { if (open && pending) details.cancel() }}><ContextMenuTrigger ref={anchor} render={<li className={className} />} tabIndex={-1}>
    <Button type="button" variant="ghost" size="sm" className="h-9 min-w-0 flex-1 justify-start gap-2 px-2 text-sm font-normal hover:bg-transparent [&_svg]:size-4" aria-current={selected ? 'page' : undefined} onClick={() => onSelect(item.targetId)}>{item.shortcut ? <PhLink aria-hidden="true" /> : <PhTable aria-hidden="true" />}<span className="truncate">{item.name}</span>{item.pinned ? <PhPushPin className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" /> : null}</Button>
    <Button type="button" size="icon-xs" variant="ghost" disabled={pending} aria-label={copy.more(item.name)} className="shrink-0 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100" onClick={() => { const node = anchor.current; if (!node) return; const rect = node.getBoundingClientRect(); node.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: rect.right, clientY: rect.bottom })) }}><PhDotsThree aria-hidden="true" /></Button>
  </ContextMenuTrigger><ResourceContextMenu label={copy.more(item.name)} items={item.actions.map(action => {
    const [label, Icon] = presentation[action]
    const checked = action === 'pin' ? item.pinned : action === 'favorite' ? item.favorited : undefined
    return { id: action, label: action === 'pin' && checked ? copy.unpin : action === 'favorite' && checked ? copy.unfavorite : label, icon: <Icon aria-hidden="true" />, checked, disabled: pending, destructive: action === 'delete', separatorAfter: action === 'copyLink' || action === 'owner', onSelect: () => onAction(item.id, action) }
  })} /></ContextMenu>
}
