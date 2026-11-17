import { motion, useReducedMotion } from 'motion/react'
import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

import { useRevealOnScroll } from './useReveal'

type PanelReveal = { revealed: boolean; reduceMotion: boolean | null }

/** BI panel geometry and viewport entrance; renderers retain their own data and actions. */
export function PanelFrame({
  title,
  description,
  meta,
  testId,
  className,
  entranceDelayMs = 0,
  children,
}: {
  title: string
  description: string
  meta?: string
  testId: string
  className?: string
  entranceDelayMs?: number
  children: ReactNode | ((state: PanelReveal) => ReactNode)
}) {
  const { ref, revealed } = useRevealOnScroll<HTMLElement>()
  const reduceMotion = useReducedMotion()
  return (
    <motion.article
      ref={ref}
      data-testid={testId}
      initial={reduceMotion ? false : { opacity: 0, y: 18, scale: 0.985 }}
      animate={revealed ? { opacity: 1, y: 0, scale: 1 } : undefined}
      transition={reduceMotion
        ? { duration: 0 }
        : { type: 'spring', stiffness: 240, damping: 26, delay: entranceDelayMs / 1000 }}
      className={cn(
        'grid min-h-72 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-lg bg-card p-4',
        className,
      )}
    >
      <header className="flex min-h-12 min-w-0 items-start justify-between gap-3 px-0 pb-3">
        <div className="min-w-0">
          <h2 data-bi-panel-title className="m-0 truncate text-sm font-semibold">{title}</h2>
          <p className="mt-1 mb-0 text-xs text-muted-foreground">{description}</p>
        </div>
        {meta ? <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{meta}</span> : null}
      </header>
      {typeof children === 'function' ? children({ revealed, reduceMotion }) : children}
    </motion.article>
  )
}
