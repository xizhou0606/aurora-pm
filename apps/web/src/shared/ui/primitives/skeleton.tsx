import * as React from 'react'

import { cn } from '@/shared/lib/utils'

function Skeleton({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="skeleton"
      aria-hidden="true"
      className={cn('relative isolate overflow-hidden rounded-sm bg-muted before:absolute before:inset-0 before:-translate-x-full before:animate-[pm-shimmer_1.8s_ease-in-out_infinite] before:bg-gradient-to-r before:from-transparent before:via-white/60 dark:before:via-white/5 before:to-transparent', className)}
      {...props}
    />
  )
}

export { Skeleton }
