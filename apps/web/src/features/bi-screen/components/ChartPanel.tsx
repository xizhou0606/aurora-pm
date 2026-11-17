import { BI_SCREEN_COPY } from '@/shared/copy'
import { LazyEChart } from '@/shared/ui'

import type { ChartModel, Drilldown } from '../types'
import { PanelFrame } from './PanelFrame'

export function ChartPanel({
  model,
  onDrilldown,
  className,
  entranceDelayMs = 0,
  fluid = false,
}: {
  model: ChartModel
  onDrilldown?: (target: Drilldown) => void
  className?: string
  entranceDelayMs?: number
  fluid?: boolean
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
      <ChartSurface model={model} onDrilldown={onDrilldown} fluid={fluid} />
    </PanelFrame>
  )
}

export function ChartSurface({
  model,
  onDrilldown,
  fluid = false,
}: {
  model: ChartModel
  onDrilldown?: (target: Drilldown) => void
  fluid?: boolean
}) {
  const interactive = Boolean(onDrilldown) && model.points.some((point) => Boolean(point.drilldown))
  return (
    <LazyEChart
      option={model.option}
      ariaLabel={model.ariaLabel}
      emptyTitle={model.emptyTitle}
      emptyDescription={model.emptyDescription}
      loadingLabel={BI_SCREEN_COPY.chart.loading}
      loadErrorLabel={BI_SCREEN_COPY.chart.loadError}
      retryLabel={BI_SCREEN_COPY.chart.retry}
      size={model.size}
      fluid={fluid}
      summary={{
        caption: model.summaryCaption,
        columns: model.summaryColumns,
        rows: model.summaryRows,
      }}
      onDataPointActivate={interactive
        ? ({ dataIndex }: { dataIndex: number }) => {
            const target = model.points[dataIndex]?.drilldown
            if (target) onDrilldown?.(target)
          }
        : undefined}
    />
  )
}
