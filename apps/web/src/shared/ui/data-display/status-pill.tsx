import { forwardRef } from 'react'

import { cn } from '@/shared/lib/utils'

export type StatusPillTone =
  | 'neutral'
  | 'info'
  | 'warning'
  | 'attention'
  | 'danger'
  | 'success'
  | 'accent'
  | 'highlight'

export type StatusPillProps = React.HTMLAttributes<HTMLSpanElement> & {
  tone?: StatusPillTone
}

const TONE_CLASSES: Readonly<Record<StatusPillTone, string>> = {
  neutral: 'border-border bg-muted text-muted-foreground dark:border-slate-500/35 dark:bg-slate-400/12 dark:text-slate-200',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-400/35 dark:bg-blue-400/14 dark:text-blue-200',
  warning: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-400/40 dark:bg-amber-400/14 dark:text-amber-200',
  attention: 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-400/40 dark:bg-orange-400/14 dark:text-orange-200',
  danger: 'border-red-200 bg-red-50 text-red-700 dark:border-red-400/40 dark:bg-red-400/14 dark:text-red-200',
  success: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-400/35 dark:bg-emerald-400/14 dark:text-emerald-200',
  accent: 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-400/35 dark:bg-violet-400/14 dark:text-violet-200',
  highlight: 'border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-400/35 dark:bg-cyan-400/14 dark:text-cyan-200',
}

export const StatusPill = forwardRef<HTMLSpanElement, StatusPillProps>(
  ({ tone = 'neutral', className, ...props }, ref) => (
    <span
      ref={ref}
      data-slot="status-pill"
      data-tone={tone}
      className={cn(
        'inline-flex min-w-16 max-w-full items-center justify-center rounded-full border px-2 py-0.5 text-xs font-medium leading-4 whitespace-nowrap transition-[box-shadow,colors] duration-150 hover:ring-1 hover:ring-current/30',
        TONE_CLASSES[tone],
        className,
      )}
      {...props}
    />
  ),
)
StatusPill.displayName = 'StatusPill'
