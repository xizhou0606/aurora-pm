import { useId, type ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

export interface DetailSectionProps {
  id?: string
  title: ReactNode
  description?: ReactNode
  hint?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
  contentClassName?: string
}

export function DetailSection({
  id,
  title,
  description,
  hint,
  actions,
  children,
  className,
  contentClassName,
}: DetailSectionProps) {
  const generatedTitleId = useId()
  const titleId = id ? `${id}-title` : generatedTitleId

  return (
    <section
      id={id}
      aria-labelledby={titleId}
      data-slot="detail-section"
      className={cn('grid min-w-0 gap-3 rounded-xl border border-border/70 bg-card p-4 shadow-xs transition-[box-shadow,border-color] duration-150', className)}
    >
      <header className="flex min-w-0 items-start justify-between gap-3 border-b border-border/60 pb-3">
        <div className="min-w-0">
          <h2 id={titleId} className="m-0 text-sm font-semibold">
            {title}
          </h2>
          {description ? <p className="mt-1 mb-0 text-xs text-muted-foreground">{description}</p> : null}
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
      </header>
      <div className={cn('min-w-0', contentClassName)}>{children}</div>
      {hint ? <p className="m-0 text-xs text-muted-foreground">{hint}</p> : null}
    </section>
  )
}
