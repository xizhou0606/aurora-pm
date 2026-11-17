import { useId } from 'react'
import { PhNewspaper, PhArrowsClockwise } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, LoadingSkeleton } from '@/shared/ui'
import { useLatestProjectNewsQuery } from '../hooks/useProjectNews'
import { formatPMDateTime } from '../model/selectors'

export function LatestProjectUpdate({
  projectId,
  className,
}: {
  projectId: number
  className?: string
}) {
  const headingId = useId()
  const query = useLatestProjectNewsQuery(projectId)
  const update = query.data
  const latestUpdateCopy = copy.projectManagement.overview.latestUpdate

  return (
    <section
      aria-labelledby={headingId}
      aria-busy={query.isFetching}
      className={cn('grid min-w-0 gap-2.5', className)}
    >
      <header className="flex min-w-0 items-center gap-2">
        <PhNewspaper aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
        <h3 id={headingId} className="min-w-0 text-sm font-semibold">
          {latestUpdateCopy.heading}
        </h3>
        {query.isFetching && !query.isPending ? (
          <PhArrowsClockwise
            aria-label={latestUpdateCopy.refreshing}
            className="ml-auto size-3.5 animate-spin text-muted-foreground"
          />
        ) : null}
      </header>

      {query.isPending && !update ? (
        <LoadingSkeleton rows={2} label={latestUpdateCopy.loading} />
      ) : null}

      {query.isError && !update ? (
        <div
          role="alert"
          className="flex min-w-0 flex-wrap items-center justify-between gap-2 border-l-2 border-destructive/50 pl-3 text-sm"
        >
          <span className="text-muted-foreground">{latestUpdateCopy.loadError}</span>
          <Button type="button" size="xs" variant="outline" onClick={() => void query.refetch()}>
            {copy.projectManagement.common.retry}
          </Button>
        </div>
      ) : null}

      {!query.isPending && !query.isError && !update ? (
        <p className="m-0 border-l-2 border-border pl-3 text-sm text-muted-foreground">
          {latestUpdateCopy.empty}
        </p>
      ) : null}

      {update ? (
        <article className="grid min-w-0 gap-1.5 border-l-2 border-primary/40 pl-3">
          <div className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
            <strong className="min-w-0 break-words text-sm font-medium text-foreground">
              {update.title}
            </strong>
            <time
              dateTime={update.created_at || undefined}
              className="shrink-0 text-xs text-muted-foreground"
            >
              {formatPMDateTime(update.created_at)}
            </time>
          </div>
          {update.content.trim() ? (
            <p className="m-0 whitespace-pre-wrap break-words text-sm leading-6 text-muted-foreground">
              {update.content}
            </p>
          ) : null}
          {query.isError ? (
            <div role="status" className="flex flex-wrap items-center gap-2 text-xs text-amber-700 dark:text-amber-400">
              <span>{latestUpdateCopy.stale}</span>
              <Button type="button" size="xs" variant="ghost" onClick={() => void query.refetch()}>
                {copy.projectManagement.common.retry}
              </Button>
            </div>
          ) : null}
        </article>
      ) : null}
    </section>
  )
}
