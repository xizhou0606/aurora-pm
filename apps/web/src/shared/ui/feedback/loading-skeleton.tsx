import * as React from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { Skeleton } from '../primitives/skeleton'

type LoadingSkeletonProps = Omit<React.ComponentProps<'div'>, 'children'> & {
  rows?: number
  label?: string
}

function normalizedRowCount(rows: number): number {
  if (!Number.isFinite(rows)) return 3
  return Math.max(1, Math.min(20, Math.floor(rows)))
}

function LoadingSkeleton({
  rows = 3,
  label = copy.ui.loading.label,
  className,
  ...props
}: LoadingSkeletonProps) {
  const rowCount = normalizedRowCount(rows)
  return (
    <div
      data-slot="loading-skeleton"
      role="status"
      aria-busy="true"
      aria-label={label}
      className={cn('pm-loading-reveal grid w-full content-start gap-2', className)}
      {...props}
    >
      {Array.from({ length: rowCount }, (_, index) => (
        <Skeleton
          key={index}
          className={cn('h-3 w-full', index === rowCount - 1 && 'w-3/4')}
        />
      ))}
    </div>
  )
}

export { LoadingSkeleton }
export type { LoadingSkeletonProps }
