import { PhWarningCircle, PhWarning, PhCheckCircle, PhInfo, PhX } from '@/shared/ui/icons'
import type { ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'

export type AlertBannerTone = 'error' | 'warning' | 'success' | 'info'

export type AlertBannerProps = {
  tone: AlertBannerTone
  children: ReactNode
  /** Optional action button rendered at the end of the banner. */
  action?: ReactNode
  /** If provided, renders a dismiss (X) button that calls this callback. */
  onDismiss?: () => void
  className?: string
}

const TONE_ICONS: Record<AlertBannerTone, typeof PhWarningCircle> = {
  error: PhWarningCircle,
  warning: PhWarning,
  success: PhCheckCircle,
  info: PhInfo,
}

const TONE_CLASSES: Record<AlertBannerTone, string> = {
  error: 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/55 dark:bg-red-950/35 dark:text-red-200/90',
  warning:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800/55 dark:bg-amber-950/35 dark:text-amber-200/90',
  success:
    'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800/50 dark:bg-emerald-950/35 dark:text-emerald-200/90',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800/50 dark:bg-blue-950/35 dark:text-blue-200/90',
}

export function AlertBanner({
  tone,
  children,
  action,
  onDismiss,
  className,
}: AlertBannerProps) {
  const Icon = TONE_ICONS[tone]
  return (
    <div
      role="alert"
      className={cn(
        'flex items-start gap-2.5 rounded-lg border p-3 text-sm leading-6',
        TONE_CLASSES[tone],
        className,
      )}
    >
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0 flex-1">{children}</div>
      {action ? <div className="flex shrink-0 items-center">{action}</div> : null}
      {onDismiss ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="-mr-1 shrink-0 text-current opacity-70 hover:opacity-100"
          onClick={onDismiss}
          aria-label={copy.ui.close}
        >
          <PhX aria-hidden="true" className="size-4" />
        </Button>
      ) : null}
    </div>
  )
}