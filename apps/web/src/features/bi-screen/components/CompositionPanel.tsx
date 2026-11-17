import { cn } from '@/shared/lib/utils'
import { EChartSummaryDetails, EmptyState } from '@/shared/ui'
import { motion } from 'motion/react'

import type { CompositionModel, Drilldown } from '../types'
import { PanelFrame } from './PanelFrame'

const SEGMENT_FILL: Readonly<Record<string, string>> = {
  blue: 'bg-blue-500 dark:bg-blue-400',
  teal: 'bg-emerald-600 dark:bg-emerald-500',
  amber: 'bg-amber-500 dark:bg-amber-400',
  purple: 'bg-violet-500 dark:bg-violet-400',
  red: 'bg-red-500 dark:bg-red-400',
  orange: 'bg-orange-500 dark:bg-orange-400',
  gray: 'bg-slate-400 dark:bg-slate-500',
}

function segmentFill(tone: string | undefined) {
  return SEGMENT_FILL[tone ?? ''] ?? 'bg-slate-400 dark:bg-slate-500'
}

export function CompositionPanel({
  model,
  onDrilldown,
  className,
  entranceDelayMs = 0,
}: {
  model: CompositionModel
  onDrilldown?: (target: Drilldown) => void
  className?: string
  entranceDelayMs?: number
}) {
  return (
    <PanelFrame
      testId={`bi-screen-${model.id}-panel`}
      title={model.title}
      description={model.description}
      meta={model.meta}
      className={className}
      entranceDelayMs={entranceDelayMs}
    >
      {({ revealed, reduceMotion }) => model.groups.length ? (
        <div className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto] gap-2 p-1 pt-3">
          <div className="grid min-h-0 content-start gap-4" aria-label={model.ariaLabel}>
            {model.groups.map((group) => (
              <div key={group.id}>
                <div className="flex min-w-0 items-baseline justify-between gap-2 text-xs">
                  <span className="truncate font-medium text-foreground">{group.label}</span>
                  <span className="shrink-0 tabular-nums text-muted-foreground">{group.total.toLocaleString('zh-CN')}</span>
                </div>
                <div className="mt-1.5 flex h-2.5 min-w-0 gap-px overflow-hidden rounded-full bg-muted">
                  {group.items.map((item, index) => {
                    const percent = group.total > 0 ? (item.count / group.total) * 100 : 0
                    const fill = segmentFill(item.tone)
                    const label = `${item.label} ${item.count.toLocaleString('zh-CN')}`
                    const segmentClass = cn('h-full origin-left', fill)
                    const segmentProps = reduceMotion
                      ? { style: { width: `${percent}%` } }
                      : {
                          initial: { width: '0%' },
                          animate: revealed ? { width: `${percent}%` } : { width: '0%' },
                          transition: {
                            type: 'spring' as const,
                            stiffness: 110,
                            damping: 22,
                            delay: 0.25 + index * 0.08,
                          },
                        }
                    if (!group.drilldownQuery) {
                      return (
                        <motion.span
                          key={item.key}
                          title={label}
                          className={segmentClass}
                          {...segmentProps}
                        />
                      )
                    }
                    return (
                      <motion.button
                        key={item.key}
                        type="button"
                        title={label}
                        aria-label={label}
                        className={cn(segmentClass, 'cursor-pointer transition-opacity hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset')}
                        {...segmentProps}
                        onClick={() => onDrilldown?.({
                          domain: 'tasks',
                          query: { [group.drilldownQuery as string]: item.key },
                        })}
                      />
                    )
                  })}
                </div>
                <div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                  {group.items.map((item) => (
                    <span key={item.key} className="inline-flex min-w-0 items-center gap-1.5">
                      <span className={cn('size-1.5 shrink-0 rounded-full', segmentFill(item.tone))} aria-hidden="true" />
                      <span className="truncate">{item.label}</span>
                      <span className="tabular-nums">{item.count.toLocaleString('zh-CN')}</span>
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
          <EChartSummaryDetails
            summary={{
              caption: model.summaryCaption,
              columns: model.summaryColumns,
              rows: model.summaryRows,
            }}
          />
        </div>
      ) : (
        <EmptyState title={model.emptyTitle} />
      )}
    </PanelFrame>
  )
}
