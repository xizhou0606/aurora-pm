import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

import { PhClock, HugeArrowRight01 } from '../icons'
import { EmptyState } from '../feedback/empty-state'

import { Button } from '../primitives/button'
import { LoadingSkeleton } from '../feedback/loading-skeleton'

export function ActivityDateValue({ value }: { value: string }) {
  const label = value.replace(/ (?:00:00:00|23:59:59)$/, '').replace(/(\d{2}:\d{2}):00$/, '$1')
  return <span title={value} className="inline-flex rounded-md border border-border/60 bg-muted/50 px-2 py-0.5 text-xs leading-5 tabular-nums text-foreground">{label}</span>
}

export function ActivityChange({ before, after }: { before: ReactNode; after: ReactNode }) {
  return <span className="inline-flex min-w-0 flex-wrap items-center gap-2">
    {before}
    <HugeArrowRight01 className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
    {after}
  </span>
}

export interface ActivityTimelineItem {
  id: string | number
  summary: ReactNode
  meta?: ReactNode
  timestamp?: ReactNode
  dateTime?: string
  icon?: ReactNode
  content?: ReactNode
  tone?: 'default' | 'positive' | 'warning' | 'danger'
}

export type ActivityTimelineState =
  | { status: 'loading'; loadingLabel: string }
  | { status: 'error'; errorLabel: ReactNode; retryLabel: string; onRetry: () => void }
  | { status: 'empty'; emptyLabel: ReactNode }
  | {
      status: 'ready'
      items: readonly ActivityTimelineItem[]
      emptyLabel: ReactNode
      warning?: ReactNode
      hasMore?: boolean
      loadingMore?: boolean
      loadMoreLabel?: string
      loadingMoreLabel?: string
      onLoadMore?: () => void
    }

export interface ActivityTimelineProps {
  title?: ReactNode
  state: ActivityTimelineState
  className?: string
  renderItem?: (item: ActivityTimelineItem) => ReactNode
}

const toneClassName: Record<NonNullable<ActivityTimelineItem['tone']>, string> = {
  default: 'bg-muted text-muted-foreground',
  positive: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
  warning: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
  danger: 'bg-destructive/10 text-destructive',
}

function DefaultActivityItem({ item }: { item: ActivityTimelineItem }) {
  return (
    <div className="relative grid min-w-0 gap-2 pl-11 pb-5 before:absolute before:left-4 before:top-9 before:bottom-0 before:w-px before:bg-border group-last:pb-0 group-last:before:hidden">
      <span
        aria-hidden="true"
        className={cn(
          'absolute top-0 left-0 flex size-8 items-center justify-center rounded-lg [&_svg]:size-4',
          toneClassName[item.tone || 'default'],
        )}
      >{item.icon || <PhClock />}</span>
      <div className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-sm">
        <span className="min-w-0 font-normal leading-6">{item.summary}</span>
        {item.timestamp ? <time dateTime={item.dateTime} className="shrink-0 text-xs text-muted-foreground">{item.timestamp}</time> : null}
      </div>
      {item.meta ? <div className="text-xs text-muted-foreground">{item.meta}</div> : null}
      {item.content ? <div className="min-w-0 text-sm text-muted-foreground">{item.content}</div> : null}
    </div>
  )
}

export function ActivityTimeline({ title, state, className, renderItem }: ActivityTimelineProps) {
  const items = state.status === 'ready' ? state.items : []

  return (
    <section data-slot="activity-timeline" className={cn('grid min-w-0 gap-3', className)}>
      {title ? <h2 className="m-0 text-sm font-semibold">{title}</h2> : null}
      {state.status === 'loading' ? <LoadingSkeleton rows={3} label={state.loadingLabel} /> : null}
      {state.status === 'error' ? (
        <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
          <span>{state.errorLabel}</span>
          <Button type="button" size="sm" variant="outline" onClick={state.onRetry}>
            {state.retryLabel}
          </Button>
        </div>
      ) : null}
      {state.status === 'empty' || (state.status === 'ready' && !items.length) ? (
        <EmptyState title={state.emptyLabel} />
      ) : null}
      {state.status === 'ready' && items.length ? (
        <ol className="m-0 grid list-none gap-0 p-0">
          {items.map((item) => (
            <li key={item.id} className="group min-w-0">
              {renderItem ? renderItem(item) : <DefaultActivityItem item={item} />}
            </li>
          ))}
        </ol>
      ) : null}
      {state.status === 'ready' && state.warning ? (
        <p role="status" className="m-0 text-xs text-amber-700 dark:text-amber-300">
          {state.warning}
        </p>
      ) : null}
      {state.status === 'ready' && state.hasMore && state.onLoadMore ? (
        <Button
          type="button"
          variant="outline"
          disabled={state.loadingMore}
          onClick={state.onLoadMore}
        >
          {state.loadingMore ? state.loadingMoreLabel : state.loadMoreLabel}
        </Button>
      ) : null}
    </section>
  )
}
