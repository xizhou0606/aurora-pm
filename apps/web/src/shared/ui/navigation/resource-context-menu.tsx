import type { ReactNode } from 'react'
import { PhCheck } from '../icons'
import { ContextMenuCheckboxIndicator, ContextMenuCheckboxItem, ContextMenuContent, ContextMenuItem, ContextMenuSeparator } from '../primitives/context-menu'

export type ResourceMenuItem = {
  id: string; label: string; icon: ReactNode; onSelect: () => void
  checked?: boolean; destructive?: boolean; disabled?: boolean; separatorAfter?: boolean
}

/** Presentation only. Consumers own permissions, action semantics and mutations. */
export function ResourceContextMenu({ label, items }: { label: string; items: readonly ResourceMenuItem[] }) {
  return <ContextMenuContent aria-label={label}>{items.flatMap((item, index) => [
    item.checked !== undefined
      ? <ContextMenuCheckboxItem key={item.id} checked={item.checked} disabled={item.disabled} onClick={item.onSelect}>{item.icon}{item.label}<ContextMenuCheckboxIndicator><PhCheck aria-hidden="true" /></ContextMenuCheckboxIndicator></ContextMenuCheckboxItem>
      : <ContextMenuItem key={item.id} disabled={item.disabled} destructive={item.destructive} onClick={item.onSelect}>{item.icon}{item.label}</ContextMenuItem>,
    item.separatorAfter && index < items.length - 1 ? <ContextMenuSeparator key={`${item.id}-divider`} /> : null,
  ])}</ContextMenuContent>
}
