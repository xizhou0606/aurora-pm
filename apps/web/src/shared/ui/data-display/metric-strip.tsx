import type { ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

export type MetricStripItem = {
  id: string
  label: ReactNode
  value: ReactNode
  description?: ReactNode
  action?: ReactNode
}

export function MetricStrip({
  items,
  className,
  'aria-label': ariaLabel = copy.ui.metrics.label,
}: {
  items: readonly MetricStripItem[]
  className?: string
  'aria-label'?: string
}) {
  return (
    <dl
      aria-label={ariaLabel}
      data-slot="metric-strip"
      className={cn('grid min-w-0 grid-cols-2 gap-px overflow-hidden rounded-lg border border-border bg-border lg:grid-cols-3 xl:grid-cols-6', className)}
    >
      {items.map((item) => (
        <div key={item.id} className="min-w-0 bg-card p-3">
          <dt className="truncate text-xs text-muted-foreground">{item.label}</dt>
          <dd className="mt-1 mb-0 flex min-w-0 items-baseline justify-between gap-2">
            <span className="truncate text-xl font-semibold tabular-nums">{item.value}</span>
            {item.action}
          </dd>
          {item.description ? <dd className="mt-1 mb-0 text-xs text-muted-foreground">{item.description}</dd> : null}
        </div>
      ))}
    </dl>
  )
}
