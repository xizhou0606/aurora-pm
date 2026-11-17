import { useRouterState } from '@tanstack/react-router'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { OperationalPage, Skeleton } from '@/shared/ui'

// Route-level pending screens: rendered inside the router match Suspense
// boundary while a lazy feature chunk loads, so switching menus shows the
// destination layout immediately instead of freezing the previous page.
// They must only import from the main bundle (shared/ui, copy) — never from
// lazily loaded feature chunks.

export type RoutePendingVariant = 'table' | 'project-split' | 'project-board'

const TABLE_ROW_GRID
  = 'grid grid-cols-[minmax(10rem,2fr)_minmax(7rem,1fr)_repeat(3,minmax(5rem,0.8fr))] gap-3'

function PendingToolbarRow() {
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      <Skeleton className="h-8 w-52" />
      <Skeleton className="h-8 w-28" />
      <Skeleton className="h-8 w-28" />
      <Skeleton className="ml-auto h-8 w-8 rounded-md" />
    </div>
  )
}

function PendingTable({ rows = 7 }: { rows?: number }) {
  return (
    <div className="min-w-0 overflow-hidden rounded-lg border border-border">
      <div className={cn(TABLE_ROW_GRID, 'border-b border-border bg-muted/50 px-3 py-2.5')}>
        {Array.from({ length: 5 }, (_, index) => (
          <Skeleton key={index} className="h-3 w-full" />
        ))}
      </div>
      {Array.from({ length: rows }, (_, index) => (
        <div
          key={index}
          className={cn(TABLE_ROW_GRID, 'border-b border-border px-3 py-3 last:border-b-0')}
        >
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-4/5" />
          <Skeleton className="h-3 w-3/4" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-2/3" />
        </div>
      ))}
    </div>
  )
}

function PendingBoard() {
  return (
    <div className="grid min-w-0 gap-4">
      <div className="flex min-w-0 items-start justify-between gap-4">
        <div className="grid min-w-0 flex-1 gap-2">
          <Skeleton className="h-5 w-28" />
          <Skeleton className="h-3 w-full max-w-80" />
        </div>
        <Skeleton className="h-8 w-28 shrink-0" />
      </div>
      <div className="grid grid-cols-1 gap-2 rounded-lg border border-border p-2.5 sm:grid-cols-2 xl:grid-cols-4">
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
      <PendingTable rows={6} />
    </div>
  )
}

function PendingListNav() {
  return (
    <div className="min-w-0 overflow-hidden rounded-lg border border-border bg-card p-2.5">
      <Skeleton className="mb-2.5 h-8 w-full" />
      {Array.from({ length: 6 }, (_, index) => (
        <div key={index} className="flex items-center gap-2 px-1.5 py-2">
          <Skeleton className="size-4 shrink-0 rounded" />
          <Skeleton className={cn('h-3 flex-1', index === 5 && 'max-w-1/2')} />
        </div>
      ))}
    </div>
  )
}

function PendingBody({ variant }: { variant: RoutePendingVariant }) {
  if (variant === 'project-split') {
    return (
      <div className="grid h-full min-h-0 min-w-0 gap-4 lg:grid-cols-[minmax(15rem,20rem)_minmax(0,1fr)]">
        <PendingListNav />
        <div className="min-w-0 overflow-hidden rounded-lg border border-border bg-card p-3 sm:p-4">
          <PendingBoard />
        </div>
      </div>
    )
  }
  if (variant === 'project-board') {
    return (
      <div className="min-w-0 rounded-lg border border-border bg-card p-3 sm:p-4">
        <PendingBoard />
      </div>
    )
  }
  return (
    <div className="grid min-w-0 gap-4">
      <PendingToolbarRow />
      <PendingTable />
    </div>
  )
}

export function RoutePendingScreen({
  title,
  variant = 'table',
}: {
  title: string
  variant?: RoutePendingVariant
}) {
  return (
    <OperationalPage title={title}>
      <div
        role="status"
        aria-busy="true"
        aria-label={copy.ui.loading.label}
        data-testid="route-pending"
        className="pm-loading-reveal grid min-w-0 gap-4 p-3 sm:p-5"
      >
        <PendingBody variant={variant} />
      </div>
    </OperationalPage>
  )
}

// Pre-commit half of the route transition (styles.css `page-fade` owns the
// post-commit cross-fade): while a path-changing navigation is in flight the
// persistent content area eases to 70% opacity, so leaving a menu reads as a
// deliberate fade instead of a frozen page. Only `isLoading` counts — once
// the destination commits, the view transition takes over.
export function useRouteContentDim(): boolean {
  return useRouterState({
    select: (state) => {
      const pathTransition = state.resolvedLocation
        ? state.location.pathname !== state.resolvedLocation.pathname
        : state.isLoading
      return Boolean(state.isLoading && pathTransition)
    },
  })
}
