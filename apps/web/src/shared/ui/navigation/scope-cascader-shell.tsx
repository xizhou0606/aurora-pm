import { type ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button } from '../primitives/button'
import { LoadingSkeleton } from '../feedback/loading-skeleton'

export type ScopeCascaderState =
  | { status: 'loading' }
  | { status: 'error'; onRetry: () => void }
  | { status: 'ready' }

export function ScopeCascader({
  state,
  children,
  summary,
  onReset,
  label = copy.ui.scope.label,
  className,
  layout = 'grid',
}: {
  state: ScopeCascaderState
  children?: ReactNode
  summary?: ReactNode
  onReset?: () => void
  label?: string
  className?: string
  layout?: 'grid' | 'single'
}) {
  return (
    <fieldset
      data-slot="scope-cascader"
      className={cn('min-w-0 rounded-lg border border-border bg-card p-3', className)}
    >
      <legend className="px-1 text-sm font-medium">{label}</legend>
      {state.status === 'loading' ? (
        <LoadingSkeleton rows={1} label={copy.ui.scope.loading} />
      ) : state.status === 'error' ? (
        <div role="alert" className="flex flex-wrap items-center justify-between gap-2 text-sm">
          <span>{copy.ui.scope.error}</span>
          <Button type="button" size="sm" variant="outline" onClick={state.onRetry}>
            {copy.ui.retry}
          </Button>
        </div>
      ) : (
        <div className="grid min-w-0 gap-2">
          <div
            data-slot="scope-cascader-fields"
            className={cn('grid min-w-0 gap-2', layout === 'grid' && 'md:grid-cols-3')}
          >
            {children}
          </div>
          {(summary || onReset) ? (
            <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
              <span className="truncate">{summary}</span>
              {onReset ? (
                <Button type="button" size="sm" variant="ghost" onClick={onReset}>
                  {copy.ui.scope.reset}
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      )}
    </fieldset>
  )
}