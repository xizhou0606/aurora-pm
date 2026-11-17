import { useVirtualizer } from '@tanstack/react-virtual'
import { useRef, type ReactNode } from 'react'
import { cn } from '@/shared/lib/utils'

export function VirtualRows<T>({
  rows,
  rowKey,
  renderRow,
  estimateSize = 76,
  threshold = 100,
  className,
  ariaLabel,
  ariaLabelledBy,
}: {
  rows: readonly T[]
  rowKey: (row: T) => string
  renderRow: (row: T) => ReactNode
  estimateSize?: number
  threshold?: number
  className?: string
  ariaLabel?: string
  ariaLabelledBy?: string
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => estimateSize,
    overscan: 8,
  })
  if (rows.length <= threshold) {
    return <div className={className}>{rows.map((row) => <div key={rowKey(row)}>{renderRow(row)}</div>)}</div>
  }
  const virtualItems = virtualizer.getVirtualItems()
  const visible = virtualItems.length
    ? virtualItems
    : rows.slice(0, 20).map((_, index) => ({ index, key: index, start: index * estimateSize, size: estimateSize, end: (index + 1) * estimateSize, lane: 0 }))
  return (
    <div
      ref={scrollRef}
      className={cn('max-h-[34rem] min-h-48 overflow-y-auto', className)}
      data-virtualized="true"
      data-row-count={rows.length}
      role="region"
      tabIndex={0}
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
    >
      <div
        className="relative h-(--virtual-rows-total-height) w-full"
        style={{ '--virtual-rows-total-height': `${virtualizer.getTotalSize() || rows.length * estimateSize}px` }}
      >
        {visible.map((virtualRow) => {
          const row = rows[virtualRow.index]
          return (
            <div
              key={rowKey(row)}
              ref={virtualizer.measureElement}
              data-index={virtualRow.index}
              className="absolute inset-x-0 top-0 translate-y-(--virtual-row-offset) box-border pb-2"
              style={{ '--virtual-row-offset': `${virtualRow.start}px` }}
            >
              {renderRow(row)}
            </div>
          )
        })}
      </div>
    </div>
  )
}
