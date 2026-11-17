import NumberFlow from '@number-flow/react'
import { motion, useReducedMotion } from 'motion/react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { EmptyState } from '@/shared/ui'

import type { Drilldown, VersionMatrixModel, VersionMatrixModule, VersionMatrixTone } from '../types'
import { PanelFrame } from './PanelFrame'

const TONE_ACCENT: Readonly<Record<VersionMatrixTone, string>> = {
  positive: 'from-emerald-500 to-teal-400',
  info: 'from-sky-500 to-cyan-400',
  warning: 'from-amber-500 to-orange-400',
  danger: 'from-red-500 to-rose-400',
}

const TONE_FILL: Readonly<Record<VersionMatrixTone, string>> = {
  positive: 'bg-gradient-to-r from-emerald-500 to-teal-400',
  info: 'bg-gradient-to-r from-sky-500 to-cyan-400',
  warning: 'bg-gradient-to-r from-amber-500 to-orange-400',
  danger: 'bg-gradient-to-r from-red-500 to-rose-400',
}

const TONE_PILL: Readonly<Record<VersionMatrixTone, string>> = {
  positive: 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-400',
  info: 'bg-sky-500/12 text-sky-700 dark:text-sky-400',
  warning: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
  danger: 'bg-red-500/12 text-destructive',
}

function MatrixCard({
  module,
  index,
  onDrilldown,
}: {
  module: VersionMatrixModule
  index: number
  onDrilldown?: (target: Drilldown) => void
}) {
  const reduceMotion = useReducedMotion()
  const spring = {
    type: 'spring' as const,
    stiffness: 260,
    damping: 24,
    delay: 0.1 + index * 0.07,
  }
  return (
    <motion.button
      type="button"
      title={BI_SCREEN_COPY.drilldown.tasksLabel}
      initial={reduceMotion ? false : { opacity: 0, y: 14, scale: 0.96 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={reduceMotion ? { duration: 0 } : spring}
      whileHover={reduceMotion ? undefined : { y: -3 }}
      onClick={() => onDrilldown?.(module.drilldown)}
      className="group relative min-w-0 overflow-hidden rounded-lg bg-gradient-to-br from-muted/60 via-transparent to-transparent p-3 text-left transition-colors hover:bg-muted/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <span
        aria-hidden="true"
        className={cn('absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r', TONE_ACCENT[module.tone])}
      />
      <div className="flex min-w-0 items-center justify-between gap-2">
        <span className="min-w-0 truncate text-xs font-medium text-foreground" title={module.label}>
          {module.label}
        </span>
        <span className={cn('shrink-0 rounded-full px-1.5 py-0.5 text-xs leading-none font-medium', TONE_PILL[module.statusTone])}>
          {module.statusLabel}
        </span>
      </div>
      <div className="mt-2 flex items-end justify-between gap-2">
        <NumberFlow
          className="text-2xl font-semibold tabular-nums text-foreground"
          value={module.pct}
          suffix="%"
        />
        <span className="pb-0.5 text-xs tabular-nums text-muted-foreground">
          {BI_SCREEN_COPY.charts.race.doneCaption(
            module.done.toLocaleString('zh-CN'),
            (module.done + module.remaining).toLocaleString('zh-CN'),
          )}
        </span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
        <motion.span
          className={cn('block h-full rounded-full', TONE_FILL[module.tone])}
          initial={reduceMotion ? false : { width: '0%' }}
          animate={{ width: `${module.pct}%` }}
          transition={reduceMotion ? { duration: 0 } : { ...spring, delay: 0.2 + index * 0.07 }}
        />
      </div>
      <div className="mt-2 flex min-w-0 items-center gap-2.5 text-xs text-muted-foreground">
        <span className="inline-flex items-center gap-1">
          <span aria-hidden="true" className="size-1.5 rounded-full bg-emerald-600 dark:bg-emerald-500" />
          <span className="tabular-nums">{module.done}</span>
        </span>
        <span className="inline-flex items-center gap-1">
          <span aria-hidden="true" className="size-1.5 rounded-full bg-slate-400 dark:bg-slate-500" />
          <span className="tabular-nums">{module.remaining}</span>
        </span>
        {module.overdue > 0 ? (
          <span className="inline-flex items-center gap-1 font-medium text-destructive">
            <span aria-hidden="true" className="size-1.5 rounded-full bg-red-500 dark:bg-red-400" />
            <span className="tabular-nums">
              {BI_SCREEN_COPY.charts.race.overdueBadge(module.overdue)}
            </span>
          </span>
        ) : null}
      </div>
    </motion.button>
  )
}

export function VersionMatrixPanel({
  model,
  onDrilldown,
  className,
  entranceDelayMs = 0,
}: {
  model: VersionMatrixModel
  onDrilldown?: (target: Drilldown) => void
  className?: string
  entranceDelayMs?: number
}) {
  return (
    <PanelFrame
      testId="bi-screen-matrix-panel"
      title={model.title}
      description={model.description}
      meta={model.meta}
      className={className}
      entranceDelayMs={entranceDelayMs}
    >
      {model.modules.length ? (
        <div
          className="grid min-h-0 grid-cols-2 content-start gap-2.5 xl:grid-cols-4"
          aria-label={model.ariaLabel}
        >
          {model.modules.map((module, index) => (
            <MatrixCard
              key={module.id}
              module={module}
              index={index}
              onDrilldown={onDrilldown}
            />
          ))}
        </div>
      ) : (
        <EmptyState title={model.emptyTitle} />
      )}
    </PanelFrame>
  )
}
