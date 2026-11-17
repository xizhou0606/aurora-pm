import * as React from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

type FilterBarProps = Omit<React.ComponentProps<'section'>, 'children'> & {
  controls: React.ReactNode
  actions?: React.ReactNode
}

function FilterBar({
  controls,
  actions,
  className,
  'aria-label': ariaLabel = copy.ui.filter.label,
  ...props
}: FilterBarProps) {
  return (
    <section
      data-slot="filter-bar"
      aria-label={ariaLabel}
      className={cn(
        'flex min-w-0 flex-col gap-2 rounded-xl bg-muted/50 p-3 sm:flex-row sm:items-center',
        className,
      )}
      {...props}
    >
      <div
        data-slot="filter-bar-controls"
        className="flex min-w-0 flex-1 flex-wrap items-center gap-2"
      >
        {controls}
      </div>
      {actions ? (
        <div
          data-slot="filter-bar-actions"
          className="flex shrink-0 flex-wrap items-center gap-2"
        >
          {actions}
        </div>
      ) : null}
    </section>
  )
}

export { FilterBar }
export type { FilterBarProps }
