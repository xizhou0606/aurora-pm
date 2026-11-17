import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

export function MetadataList({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <dl className={cn(
      'grid grid-cols-[minmax(7rem,auto)_minmax(0,1fr)] gap-x-3 gap-y-2 text-sm',
      className,
    )}>
      {children}
    </dl>
  )
}

export function MetadataItem({
  label,
  children,
  className,
}: {
  label: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <div className={cn('contents', className)}>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="m-0 min-w-0">{children}</dd>
    </div>
  )
}
