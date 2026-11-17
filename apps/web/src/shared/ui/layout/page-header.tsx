import * as React from 'react'

import { cn } from '@/shared/lib/utils'

type PageHeaderProps = Omit<React.ComponentProps<'header'>, 'title'> & {
  title: React.ReactNode
  description?: React.ReactNode
  meta?: React.ReactNode
  actions?: React.ReactNode
  headingLevel?: 'h1' | 'h2'
  /** 'page' (default): border-b + bg-card, for use as the top-level page heading.
   *  'inline': plain header without border/background, for use inside a card or section. */
  variant?: 'page' | 'inline'
}

function PageHeader({
  title,
  description,
  meta,
  actions,
  headingLevel = 'h1',
  variant = 'page',
  className,
  ...props
}: PageHeaderProps) {
  const Heading = headingLevel
  return (
    <header
      data-slot="page-header"
      className={cn(
        'flex min-w-0 flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-start',
        variant === 'page' && 'border-b border-border/60 bg-card px-4 py-4 sm:px-6',
        className,
      )}
      {...props}
    >
      <div data-slot="page-header-content" className="min-w-0">
        <Heading className="text-xl leading-7 font-semibold tracking-tight text-foreground">
          {title}
        </Heading>
        {description ? (
          <div
            data-slot="page-header-description"
            className="mt-1 text-sm leading-5 text-muted-foreground"
          >
            {description}
          </div>
        ) : null}
        {meta ? (
          <div
            data-slot="page-header-meta"
            className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
          >
            {meta}
          </div>
        ) : null}
      </div>
      {actions ? (
        <div
          data-slot="page-header-actions"
          className="flex shrink-0 flex-wrap items-center gap-2"
        >
          {actions}
        </div>
      ) : null}
    </header>
  )
}

export { PageHeader }
export type { PageHeaderProps }
