import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'

import { copy } from '@/shared/copy'
import { Button, EmptyState } from '@/shared/ui'
import { RequirementCard, RequirementCardSkeleton } from './RequirementPoolCard'
import type { RequirementTableRow } from './RequirementPoolTable'
import type { WorkflowAssignee } from '../model/types'

const poolCopy = copy.aiPM.requirementPool
const VIRTUALIZE_AFTER = 24
const CARD_MIN_WIDTH = 240
const CARD_GAP = 16
const CARD_BODY = 184

function createdAtTime(value: string) {
  const time = Date.parse(value)
  return Number.isFinite(time) ? time : 0
}

function sortCardsByCreatedAtDesc<Row extends RequirementTableRow & { created_at: string }>(rows: readonly Row[]) {
  return [...rows].sort((left, right) => {
    const delta = createdAtTime(right.created_at) - createdAtTime(left.created_at)
    return delta !== 0 ? delta : right.id - left.id
  })
}

function useElementWidth<Element extends HTMLElement>(active: boolean) {
  const ref = useRef<Element>(null)
  const [width, setWidth] = useState(0)
  useEffect(() => {
    if (!active) return
    const node = ref.current
    if (!node) return
    const update = () => setWidth(node.clientWidth)
    update()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(update)
    observer.observe(node)
    return () => observer.disconnect()
  }, [active])
  return { ref, width }
}

function cardColumns(width: number) {
  if (width <= 0) return 1
  return Math.max(1, Math.floor((width + CARD_GAP) / (CARD_MIN_WIDTH + CARD_GAP)))
}

function cardRowHeight(width: number, columns: number) {
  const cardWidth = Math.max(CARD_MIN_WIDTH, (Math.max(width, CARD_MIN_WIDTH) - CARD_GAP * (columns - 1)) / columns)
  return Math.ceil(cardWidth / 2 + CARD_BODY + CARD_GAP)
}

export function RequirementPoolCards<Row extends RequirementTableRow & { created_at: string }>({
  rows,
  workspaceId,
  people,
  pending,
  error,
  refreshing,
  onRetry,
  onOpen,
  renderOverflow,
}: {
  rows: Row[]
  workspaceId: number
  people?: readonly WorkflowAssignee[]
  pending: boolean
  error: boolean
  refreshing: boolean
  onRetry: () => void
  onOpen: (requirement: Row) => void
  renderOverflow?: (requirement: Row) => ReactNode
}) {
  const orderedRows = sortCardsByCreatedAtDesc(rows)
  const { ref: widthRef, width } = useElementWidth<HTMLDivElement>(!error && (pending || orderedRows.length > 0))
  const scrollRef = useRef<HTMLDivElement>(null)
  const columns = cardColumns(width)
  const virtualize = !pending && orderedRows.length > VIRTUALIZE_AFTER
  const rowCount = Math.ceil(orderedRows.length / Math.max(columns, 1))
  const estimateSize = cardRowHeight(width, columns)
  const virtualizer = useVirtualizer({
    count: virtualize ? rowCount : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => estimateSize,
    overscan: 4,
  })

  if (error) {
    return (
      <EmptyState
        role="alert"
        className="border border-border bg-card"
        title={poolCopy.loadFailedTitle}
        action={<Button variant="outline" onClick={onRetry}>{copy.ui.retry}</Button>}
      />
    )
  }
  if (!pending && orderedRows.length === 0) {
    return (
      <EmptyState
        className="border border-border bg-card"
        title={poolCopy.emptyTitle}
      />
    )
  }

  const renderCard = (requirement: Row) => (
    <RequirementCard
      requirement={requirement}
      workspaceId={workspaceId}
      people={people}
      onOpen={onOpen}
      renderOverflow={renderOverflow}
    />
  )

  const virtualItems = virtualizer.getVirtualItems()
  const visible = virtualItems.length
    ? virtualItems
    : Array.from({ length: Math.min(rowCount, 8) }, (_, index) => ({
      index,
      key: index,
      start: index * estimateSize,
      size: estimateSize,
      end: (index + 1) * estimateSize,
      lane: 0,
    }))

  return (
    <div ref={widthRef} className="min-w-0">
      {virtualize ? (
        <div
          ref={scrollRef}
          className="max-h-[calc(100dvh-12rem)] min-h-48 overflow-y-auto"
          data-virtualized="true"
          data-row-count={orderedRows.length}
        >
          <ul
            aria-label={poolCopy.cards}
            aria-busy={refreshing}
            className="relative m-0 h-(--pool-list-height) list-none p-0"
            style={{ '--pool-list-height': `${virtualizer.getTotalSize() || rowCount * estimateSize}px` }}
          >
            {visible.map((virtualRow) => {
              const start = virtualRow.index * columns
              const chunk = orderedRows.slice(start, start + columns)
              return (
                <li
                  key={virtualRow.key}
                  ref={virtualizer.measureElement}
                  data-index={virtualRow.index}
                  className="absolute inset-x-0 top-0 m-0 grid translate-y-(--pool-row-offset) list-none gap-4 pb-4 [grid-template-columns:var(--pool-row-columns)]"
                  style={{
                    '--pool-row-offset': `${virtualRow.start}px`,
                    '--pool-row-columns': `repeat(${columns}, minmax(0, 1fr))`,
                  }}
                >
                  {chunk.map((requirement) => (
                    <div key={requirement.id} className="min-w-0">{renderCard(requirement)}</div>
                  ))}
                </li>
              )
            })}
          </ul>
        </div>
      ) : (
        <ul
          aria-label={poolCopy.cards}
          aria-busy={pending || refreshing}
          className="pm-requirement-card-grid m-0 grid min-w-0 list-none gap-4 p-0"
        >
          {pending
            ? Array.from({ length: 15 }, (_, index) => <RequirementCardSkeleton key={index} />)
            : orderedRows.map((requirement) => (
              <li key={requirement.id} className="min-w-0">{renderCard(requirement)}</li>
            ))}
        </ul>
      )}
    </div>
  )
}