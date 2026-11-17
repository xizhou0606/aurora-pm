import { PhList } from '@/shared/ui/icons'
import { useMemo, useState } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, EmptyState, ResponsivePanel, useAppearance } from '@/shared/ui'
import { motion, useReducedMotion } from 'motion/react'

import { qualityRiskDelta, qualityTrendChart, versionQualityMatrixModel } from '../charts'
import type { DashboardData, Drilldown } from '../types'
import { ChartPanel, ChartSurface } from './ChartPanel'
import { OwnerTable } from './OwnerTable'
import { useRevealOnScroll } from './useReveal'

const severityToneClasses: Readonly<Record<string, string>> = {
  red: 'before:bg-red-500',
  orange: 'before:bg-orange-500',
  amber: 'before:bg-amber-500',
  teal: 'before:bg-emerald-600',
}

export function QualityOverview({
  data,
  onDrilldown,
}: {
  data: DashboardData
  onDrilldown: (target: Drilldown) => void
}) {
  const [allOwners, setAllOwners] = useState(false)
  const { resolved } = useAppearance()
  const trendChart = useMemo(() => qualityTrendChart(data), [data, resolved])
  const riskMatrix = useMemo(
    () => versionQualityMatrixModel(data.version_quality_risks),
    [data.version_quality_risks, resolved],
  )
  const riskDelta = useMemo(() => qualityRiskDelta(data.quality_trend), [data.quality_trend])
  const owners = data.quality_owner_load
  const preview = owners.slice(0, 6)
  const severityTotal = data.quality_severity_distribution.reduce(
    (total, item) => total + item.count,
    0,
  )
  const trendReveal = useRevealOnScroll<HTMLElement>()
  const tableReveal = useRevealOnScroll<HTMLElement>()
  const reduceMotion = useReducedMotion()
  const spring = { type: 'spring' as const, stiffness: 240, damping: 26 }
  return (
    <section aria-label={BI_SCREEN_COPY.charts.qualitySectionLabel} className="grid min-w-0 gap-3">
      <motion.article
        ref={trendReveal.ref}
        data-testid="bi-screen-quality-trend-panel"
        initial={reduceMotion ? false : { opacity: 0, y: 18, scale: 0.985 }}
        animate={trendReveal.revealed ? { opacity: 1, y: 0, scale: 1 } : undefined}
        transition={reduceMotion ? { duration: 0 } : spring}
        className="grid min-w-0 overflow-hidden rounded-lg bg-card p-4"
      >
        <header className="flex min-h-12 min-w-0 items-start justify-between gap-3 px-0 pb-3">
          <div className="min-w-0">
            <h2 className="m-0 truncate text-sm font-semibold">{trendChart.title}</h2>
            <p className="mt-1 mb-0 text-xs text-muted-foreground">{trendChart.description}</p>
          </div>
          <div
            className={cn(
              'flex shrink-0 items-baseline gap-1.5 pt-px text-xs',
              riskDelta.tone === 'positive' && 'text-emerald-700 dark:text-emerald-400',
              riskDelta.tone === 'negative' && 'text-destructive',
              riskDelta.tone === 'neutral' && 'text-muted-foreground',
            )}
          >
            <span>{riskDelta.label}</span>
            <strong className="text-lg font-semibold tabular-nums">{riskDelta.value}</strong>
          </div>
        </header>
        <div className="grid min-w-0 gap-1 md:grid-cols-[122px_minmax(0,1fr)] md:gap-3.5">
          <aside
            className="grid min-w-0 grid-cols-2 gap-1.5 py-2 md:flex md:flex-col md:border-r md:border-border md:border-b-0 md:pr-3 md:pl-0.5"
            aria-label={BI_SCREEN_COPY.qualitySeverity.ariaLabel}
          >
            <span className="col-span-full mb-1 text-xs font-semibold text-muted-foreground">
              {BI_SCREEN_COPY.qualitySeverity.title}
            </span>
            {data.quality_severity_distribution.map((item) => (
              <div
                key={item.key}
                className={cn(
                  'relative flex min-h-10 items-center justify-between gap-2 py-1 pr-0.5 pl-3 text-xs text-muted-foreground before:absolute before:inset-y-2 before:left-0 before:w-[3px] before:rounded-sm before:bg-muted-foreground',
                  severityToneClasses[item.tone ?? ''],
                )}
              >
                <span className="truncate">{item.label}</span>
                <strong className="text-base font-semibold tabular-nums text-foreground">{item.count}</strong>
              </div>
            ))}
            <div className="col-span-full flex items-baseline justify-end gap-1.5 pt-2 text-xs text-muted-foreground">
              <span>{BI_SCREEN_COPY.qualitySeverity.total}</span>
              <strong className="text-base font-semibold tabular-nums text-foreground">{severityTotal}</strong>
            </div>
          </aside>
          <ChartSurface model={trendChart} onDrilldown={onDrilldown} />
        </div>
      </motion.article>

      <div className="grid min-w-0 gap-3 xl:grid-cols-2">
        <ChartPanel model={riskMatrix} onDrilldown={onDrilldown} entranceDelayMs={160} />
        <motion.article
          ref={tableReveal.ref}
          initial={reduceMotion ? false : { opacity: 0, y: 18, scale: 0.985 }}
          animate={tableReveal.revealed ? { opacity: 1, y: 0, scale: 1 } : undefined}
          transition={reduceMotion ? { duration: 0 } : { ...spring, delay: 0.24 }}
          className="grid min-h-72 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-lg bg-card p-4"
        >
          <header className="flex min-h-12 flex-wrap items-start justify-between gap-3 px-0 pb-3">
            <div className="min-w-0">
              <h2 className="m-0 text-sm font-semibold">{BI_SCREEN_COPY.ownerTable.title}</h2>
              <p className="mt-1 mb-0 text-xs text-muted-foreground">
                {BI_SCREEN_COPY.ownerTable.description}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
              <span>{BI_SCREEN_COPY.ownerTable.meta(owners.length)}</span>
              {owners.length > preview.length ? (
                <Button type="button" size="sm" variant="ghost" onClick={() => setAllOwners(true)}>
                  <PhList aria-hidden="true" />
                  {BI_SCREEN_COPY.ownerTable.showAll}
                </Button>
              ) : null}
            </div>
          </header>
          <div className="min-w-0 overflow-x-auto pt-1.5">
            {preview.length ? (
              <OwnerTable items={preview} />
            ) : (
              <EmptyState
                title={BI_SCREEN_COPY.ownerTable.emptyTitle}
              />
            )}
          </div>
        </motion.article>
      </div>

      <ResponsivePanel
        open={allOwners}
        onOpenChange={setAllOwners}
        title={BI_SCREEN_COPY.ownerTable.dialogTitle}
        description={BI_SCREEN_COPY.ownerTable.dialogDescription(owners.length)}
        className="max-w-5xl"
      >
        <div className="overflow-x-auto">
          <OwnerTable items={owners} />
        </div>
      </ResponsivePanel>
    </section>
  )
}
