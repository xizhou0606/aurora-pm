import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

export interface EntityDetailLayoutProps {
  header?: ReactNode
  main: ReactNode
  rail?: ReactNode
  actions?: ReactNode
  ariaLabel?: string
  ariaLabelledBy?: string
  className?: string
  mainClassName?: string
  railClassName?: string
}

export function EntityDetailLayout({
  header,
  main,
  rail,
  actions,
  ariaLabel,
  ariaLabelledBy,
  className,
  mainClassName,
  railClassName,
}: EntityDetailLayoutProps) {
  return (
    <article
      data-slot="entity-detail-layout"
      aria-label={ariaLabelledBy ? undefined : ariaLabel}
      aria-labelledby={ariaLabelledBy}
      className={cn('grid min-w-0 gap-4', className)}
    >
      {header || actions ? (
        <header className="flex min-w-0 flex-col gap-3 border-b border-border pb-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 flex-1">{header}</div>
          {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
        </header>
      ) : null}
      <div
        className={cn(
          'grid min-w-0 items-start gap-4',
          rail && 'xl:grid-cols-[minmax(0,1fr)_22rem]',
        )}
      >
        <div data-slot="entity-detail-main" className={cn('grid min-w-0 gap-4', mainClassName)}>
          {main}
        </div>
        {rail ? (
          <aside
            data-slot="entity-detail-rail"
            className={cn('grid min-w-0 gap-4 xl:sticky xl:top-0', railClassName)}
          >
            {rail}
          </aside>
        ) : null}
      </div>
    </article>
  )
}
