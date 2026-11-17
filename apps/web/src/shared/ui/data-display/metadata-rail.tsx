import { useId, type ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

export interface MetadataRailProps {
  id?: string
  title?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
}

export function MetadataRail({ id, title, actions, children, className }: MetadataRailProps) {
  const generatedTitleId = useId()
  const titleId = title ? (id ? `${id}-title` : generatedTitleId) : undefined
  return (
    <section
      id={id}
      aria-labelledby={titleId}
      data-slot="metadata-rail"
      className={cn('grid min-w-0 gap-3 rounded-lg border border-border bg-card p-4', className)}
    >
      {title || actions ? (
        <header className="flex min-w-0 items-center justify-between gap-3">
          {title ? <h2 id={titleId} className="m-0 text-sm font-semibold">{title}</h2> : <span />}
          {actions}
        </header>
      ) : null}
      {children}
    </section>
  )
}
