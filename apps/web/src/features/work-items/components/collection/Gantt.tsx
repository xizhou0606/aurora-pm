import { lazy, Suspense, useMemo, useState } from 'react'
import { PhCaretLeft, PhCaretRight } from '@/shared/ui/icons'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useMediaQuery } from '@/shared/hooks'
import { Button, EmptyState, LoadingSkeleton, ViewTabs } from '@/shared/ui'
import {
  ganttWindow,
  groupScheduledWorkItems,
  shiftGanttAnchor,
  type GanttScale,
} from '../../model/gantt'
import type { WorkItemDetailPatch } from '../../model/mutation'
import type { UserOptionsState, WorkItem, WorkItemsGroupBy } from '../../model/types'

const GanttScheduler = lazy(() => import('./GanttScheduler').then((m) => ({ default: m.GanttScheduler })))
const NARROW_QUERY = '(max-width: 720px)'

function windowTitle(anchor: Date, scale: GanttScale): string {
  const year = anchor.getUTCFullYear()
  const month = anchor.getUTCMonth() + 1
  if (scale === 'year') return WORK_ITEMS_COPY.gantt.yearTitle(year)
  if (scale === 'quarter') return WORK_ITEMS_COPY.gantt.quarterTitle(year, Math.ceil(month / 3))
  if (scale === 'week') return WORK_ITEMS_COPY.gantt.weekTitle(year, month, anchor.getUTCDate())
  return WORK_ITEMS_COPY.gantt.monthTitle(year, month)
}

export function Gantt({
  rows,
  groupBy,
  users,
  pagination,
  onOpen,
  onUpdate,
}: {
  rows: readonly WorkItem[]
  groupBy: WorkItemsGroupBy
  users: UserOptionsState
  pagination: {
    hasMore: boolean
    loadingMore: boolean
    loaded: number
    total: number
    loadMore: () => void
  }
  onOpen?: (row: WorkItem, trigger: HTMLElement) => void
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  const narrow = useMediaQuery(NARROW_QUERY)
  const [scale, setScale] = useState<GanttScale>('month')
  const [anchor, setAnchor] = useState(() => new Date())
  const [collapsedGroupIds, setCollapsedGroupIds] = useState<ReadonlySet<string>>(
    () => new Set(),
  )
  const userOptions = users.status === 'ready' ? users.data : []
  const groups = useMemo(
    () => groupScheduledWorkItems(rows, groupBy, userOptions),
    [groupBy, rows, userOptions],
  )
  const window = useMemo(() => ganttWindow(anchor, scale, narrow), [anchor, narrow, scale])

  const toggleGroup = (groupId: string) => {
    setCollapsedGroupIds((current) => {
      const next = new Set(current)
      if (next.has(groupId)) next.delete(groupId)
      else next.add(groupId)
      return next
    })
  }

  if (!groups.length) {
    return (
      <EmptyState
        title={WORK_ITEMS_COPY.gantt.empty}
        action={pagination.hasMore ? (
          <Button type="button" variant="outline" disabled={pagination.loadingMore} onClick={pagination.loadMore}>
            {pagination.loadingMore ? WORK_ITEMS_COPY.gantt.loadingMore : WORK_ITEMS_COPY.gantt.loadMore}
          </Button>
        ) : undefined}
      />
    )
  }

  return (
    <section
      className="grid min-w-0 overflow-hidden rounded-lg border border-border bg-card"
      aria-label={WORK_ITEMS_COPY.gantt.label}
      data-testid="work-items-daypilot-gantt"
    >
      <div className="flex min-w-0 flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="grid min-w-0 gap-0.5">
          <strong className="text-base font-semibold text-foreground">{windowTitle(anchor, scale)}</strong>
          <span className="text-xs text-muted-foreground">{WORK_ITEMS_COPY.gantt.interactionHint}</span>
        </div>
        <div className="flex min-w-0 flex-wrap items-center justify-end gap-2 max-[390px]:grid max-[390px]:grid-cols-[1fr_auto_auto_auto] max-[390px]:justify-stretch">
          <ViewTabs
            value={scale}
            label={WORK_ITEMS_COPY.gantt.scale}
            className="border-0 bg-transparent p-0 max-[390px]:col-span-4 max-[390px]:justify-end [&_button]:px-2"
            tabs={(Object.keys(WORK_ITEMS_COPY.gantt.scales) as GanttScale[]).map((value) => ({
              id: value,
              label: WORK_ITEMS_COPY.gantt.scales[value],
            }))}
            onValueChange={(value) => setScale(value as GanttScale)}
          />
          <Button type="button" size="sm" variant="ghost" onClick={() => setAnchor(new Date())}>
            {WORK_ITEMS_COPY.gantt.today}
          </Button>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label={WORK_ITEMS_COPY.gantt.previous}
            onClick={() => setAnchor((current) => shiftGanttAnchor(current, scale, -1))}
          >
            <PhCaretLeft aria-hidden="true" />
          </Button>
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label={WORK_ITEMS_COPY.gantt.next}
            onClick={() => setAnchor((current) => shiftGanttAnchor(current, scale, 1))}
          >
            <PhCaretRight aria-hidden="true" />
          </Button>
        </div>
      </div>
      <div
        className="min-w-0 overflow-hidden"
        role="region"
        aria-label={WORK_ITEMS_COPY.gantt.timeline}
        tabIndex={0}
        onScrollCapture={(event) => {
          const target = event.target
          if (!(target instanceof HTMLElement) || !pagination.hasMore || pagination.loadingMore) return
          const remaining = target.scrollHeight - target.scrollTop - target.clientHeight
          if (remaining <= 160) pagination.loadMore()
        }}
      >
        <Suspense fallback={<LoadingSkeleton rows={6} label={WORK_ITEMS_COPY.gantt.loading} className="min-h-[32rem]" />}>
          <GanttScheduler
            groups={groups}
            window={window}
            narrow={narrow}
            collapsedGroupIds={collapsedGroupIds}
            onToggleGroup={toggleGroup}
            onOpen={onOpen}
            onUpdate={onUpdate}
          />
        </Suspense>
      </div>
      {pagination.hasMore || pagination.loadingMore ? (
        <div className="flex items-center justify-center border-t border-border px-4 py-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={pagination.loadingMore}
            onClick={pagination.loadMore}
          >
            {pagination.loadingMore
              ? WORK_ITEMS_COPY.gantt.loadingMore
              : WORK_ITEMS_COPY.gantt.loadMoreProgress(pagination.loaded, pagination.total)}
          </Button>
        </div>
      ) : null}
    </section>
  )
}
