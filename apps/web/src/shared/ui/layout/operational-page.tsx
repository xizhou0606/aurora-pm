import type { ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

export type OperationalRefreshState = 'idle' | 'refreshing' | 'stale_error'

export type OperationalPageProps = {
  title: ReactNode
  titleAccessory?: ReactNode
  description?: ReactNode
  actions?: ReactNode
  context?: ReactNode
  metrics?: ReactNode
  refresh?: OperationalRefreshState
  bodyOverflow?: 'auto' | 'hidden'
  visuallyHiddenHeader?: boolean
  children: ReactNode
  className?: string
}

export function OperationalPage({
  title,
  titleAccessory,
  description,
  actions,
  context,
  metrics,
  refresh = 'idle',
  bodyOverflow = 'auto',
  visuallyHiddenHeader = false,
  children,
  className,
}: OperationalPageProps) {
  return (
    <section
      data-slot="operational-page"
      aria-busy={refresh === 'refreshing' || undefined}
      className={cn('flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-muted/25 text-foreground', className)}
    >
      <header className={cn(
        'grid shrink-0 gap-3 border-b border-border/60 bg-card px-4 py-4 sm:px-6',
        visuallyHiddenHeader && 'sr-only',
      )}>
        <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <div className={cn('flex min-w-0 items-center gap-3', titleAccessory && 'pr-1')}>
              <h1 className="m-0 min-w-0 truncate text-xl leading-7 font-semibold tracking-tight">{title}</h1>
              {titleAccessory ? (
                <div data-slot="operational-page-title-accessory" className="min-w-0 shrink-0">
                  {titleAccessory}
                </div>
              ) : null}
            </div>
            {description ? <p className="mt-1 mb-0 text-sm text-muted-foreground">{description}</p> : null}
          </div>
          {actions ? <div data-slot="operational-page-actions" className="flex max-w-full shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
        </div>
        {context}
        {metrics}
      </header>
      {refresh === 'refreshing' ? (
        <span role="status" className="sr-only">{copy.ui.dataTable.refreshingLabel}</span>
      ) : null}
      {refresh === 'stale_error' ? (
        <div role="alert" className="shrink-0 border-b border-border bg-muted px-4 py-2 text-sm text-foreground">
          {copy.ui.operationalPage.staleError}
        </div>
      ) : null}
      <div
        data-slot="operational-page-scroll"
        className={cn(
          'min-h-0 min-w-0 flex-1 animate-[pm-loading-reveal_0s_linear_150ms_forwards] opacity-0',
          bodyOverflow === 'hidden' ? 'flex flex-col overflow-hidden' : 'overflow-auto',
        )}
      >
        {children}
      </div>
    </section>
  )
}
