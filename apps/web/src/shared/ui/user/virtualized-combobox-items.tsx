import * as React from 'react'

import { Combobox } from '@base-ui/react/combobox'
import {
  useVirtualizer,
  type VirtualItem,
  type Virtualizer,
} from '@tanstack/react-virtual'

import { cn } from '@/shared/lib/utils'

/**
 * Options at or below this count render as a plain list; larger directories
 * render a windowed virtual list so full-directory pickers stay smooth.
 */
export const USER_SELECT_VIRTUALIZE_THRESHOLD = 60

const VIRTUALIZE_OVERSCAN = 12
const ITEM_ESTIMATED_SIZE = 40

export type ComboboxVirtualizerRef = Virtualizer<HTMLDivElement, Element>

/** Keyboard highlight → virtual scroll sync (Base UI virtualized pattern). */
export function scrollHighlightedItemIntoView(
  virtualizer: ComboboxVirtualizerRef | null,
  index: number | undefined,
  reason: string,
) {
  if (!virtualizer || index == null) return
  const isStart = index === 0
  const isEnd = index === virtualizer.options.count - 1
  const shouldScroll = reason === 'none'
    || (reason === 'keyboard' && (isStart || isEnd))
  if (shouldScroll) {
    queueMicrotask(() => {
      virtualizer.scrollToIndex(index, { align: isEnd ? 'start' : 'end' })
    })
  }
}

export type VirtualizedComboboxItemsProps<T> = {
  virtualizerRef: React.RefObject<ComboboxVirtualizerRef | null>
  renderItem: (
    item: T,
    virtualItem: VirtualItem,
    virtualizer: ComboboxVirtualizerRef,
  ) => React.ReactNode
  className?: string
  /** Load-more hook for paginated directories (server-search pickers). */
  onNearEnd?: () => void
  nearEndBusy?: boolean
}

/**
 * Windowed body for a Base UI combobox whose root is rendered with
 * `virtualized`. Reads the root's filtered items via `useFilteredItems` and
 * renders only the visible slice, absolutely positioned inside a sized
 * placeholder (see the Base UI virtualized combobox pattern).
 */
export function VirtualizedComboboxItems<T>({
  virtualizerRef,
  renderItem,
  className,
  onNearEnd,
  nearEndBusy = false,
}: VirtualizedComboboxItemsProps<T>) {
  const filteredItems = Combobox.useFilteredItems<T>()
  const scrollElementRef = React.useRef<HTMLDivElement | null>(null)
  const virtualizer = useVirtualizer<HTMLDivElement, Element>({
    count: filteredItems.length,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: () => ITEM_ESTIMATED_SIZE,
    overscan: VIRTUALIZE_OVERSCAN,
  })
  React.useImperativeHandle(virtualizerRef, () => virtualizer)
  const setScrollElement = React.useCallback((element: HTMLDivElement | null) => {
    scrollElementRef.current = element
    if (element) virtualizer.measure()
  }, [virtualizer])
  const handleScroll = React.useCallback((event: React.UIEvent<HTMLDivElement>) => {
    if (!onNearEnd || nearEndBusy) return
    const list = event.currentTarget
    if (list.scrollHeight - list.scrollTop - list.clientHeight <= 32) onNearEnd()
  }, [onNearEnd, nearEndBusy])

  if (filteredItems.length === 0) return null
  return (
    <div
      role="presentation"
      data-slot="virtualized-combobox-scroller"
      ref={setScrollElement}
      onScroll={onNearEnd ? handleScroll : undefined}
      className={
        cn('max-h-[min(20rem,var(--available-height))] overflow-y-auto overscroll-contain p-1', className)
      }
    >
      <div
        role="presentation"
        className="relative h-(--virtual-list-height) w-full"
        style={{ '--virtual-list-height': `${virtualizer.getTotalSize()}px` }}
      >
        {virtualizer.getVirtualItems().map((virtualItem) => {
          const item = filteredItems[virtualItem.index]
          if (item === undefined) return null
          return renderItem(item, virtualItem, virtualizer)
        })}
      </div>
    </div>
  )
}
