import { useMemo } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { Button, EmptyState, LazyEChart, LoadingSkeleton, MetricStrip, readThemePaint, useAppearance } from '@/shared/ui'
import type {
  QualityDashboard,
  QualityDashboardState,
  QualityDashboardSummary,
} from '../../model/types'
import { buildQualityDashboardChartOptions } from './chart-options'

export function DashboardMetrics({ summary }: { summary: QualityDashboardSummary }) {
  return (
    <MetricStrip
      className="xl:grid-cols-4"
      items={[
        {
          id: 'submitted',
          label: QUALITY_COPY.dashboard.submitted,
          value: summary.submitted,
          description: QUALITY_COPY.dashboard.submittedDescription,
        },
        {
          id: 'resolved',
          label: QUALITY_COPY.dashboard.resolved,
          value: summary.resolved,
          description: QUALITY_COPY.dashboard.resolvedDescription,
        },
        {
          id: 'closed',
          label: QUALITY_COPY.dashboard.closed,
          value: summary.closed,
          description: QUALITY_COPY.dashboard.closedDescription,
        },
        {
          id: 'backlog',
          label: QUALITY_COPY.dashboard.backlog,
          value: summary.backlog,
          description: QUALITY_COPY.dashboard.backlogDescription,
        },
      ]}
    />
  )
}

function ChartCard({
  title,
  children,
  className = '',
}: {
  title: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <article className={`grid min-h-[22rem] min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-md border border-border bg-card p-4 ${className}`}>
      <header className="flex min-w-0 items-start justify-between gap-3 border-b border-border pb-2.5">
        <div className="min-w-0">
          <h2 className="m-0 truncate text-sm font-semibold">{title}</h2>
        </div>
      </header>
      {children}
    </article>
  )
}

function chartProps(ariaLabel: string, caption: string) {
  return {
    ariaLabel,
    emptyTitle: QUALITY_COPY.dashboard.empty,
    emptyDescription: QUALITY_COPY.dashboard.emptyDescription,
    loadingLabel: QUALITY_COPY.dashboard.chartLoading,
    loadErrorLabel: QUALITY_COPY.dashboard.chartLoadError,
    retryLabel: QUALITY_COPY.dashboard.retry,
    summary: { caption, columns: [] as string[], rows: [] as (string | number)[][] },
  }
}

function DashboardCharts({ data }: { data: QualityDashboard }) {
  const { resolved } = useAppearance()
  const paint = useMemo(() => readThemePaint(), [resolved])
  const people = data.byPerson.slice(0, 10)
  const options = buildQualityDashboardChartOptions(data, paint, resolved)
  const personProps = chartProps(QUALITY_COPY.dashboard.personAria, QUALITY_COPY.dashboard.personSummary)
  personProps.summary.columns = [
    QUALITY_COPY.dashboard.owner,
    QUALITY_COPY.dashboard.defectCount,
    QUALITY_COPY.dashboard.resolved,
    QUALITY_COPY.dashboard.closed,
    QUALITY_COPY.dashboard.backlog,
  ]
  personProps.summary.rows = people.map((row) => [
    row.userName, row.submitted, row.resolved, row.closed, row.backlog,
  ])

  const dimensionChart = (
    rows: QualityDashboard['byStatus'],
    ariaLabel: string,
    caption: string,
    option: Readonly<Record<string, unknown>> | null,
  ) => {
    const visibleRows = rows.filter((row) => row.count > 0)
    const props = chartProps(ariaLabel, caption)
    props.summary.columns = [QUALITY_COPY.dashboard.dimension, QUALITY_COPY.dashboard.count]
    props.summary.rows = visibleRows.map((row) => [row.label, row.count])
    return { props, option }
  }
  const status = dimensionChart(
    data.byStatus,
    QUALITY_COPY.dashboard.statusAria,
    QUALITY_COPY.dashboard.statusSummary,
    options.status,
  )
  const severity = dimensionChart(
    data.bySeverity,
    QUALITY_COPY.dashboard.severityAria,
    QUALITY_COPY.dashboard.severitySummary,
    options.severity,
  )

  return (
    <section
      aria-label={QUALITY_COPY.dashboard.sectionLabel}
      className="grid min-w-0 grid-cols-1 gap-3 lg:grid-cols-3"
    >
      <ChartCard
        title={QUALITY_COPY.dashboard.personTitle}
      >
        <LazyEChart {...personProps} option={options.person} size="compact" />
      </ChartCard>
      <ChartCard title={QUALITY_COPY.dashboard.statusTitle}>
        <LazyEChart {...status.props} option={status.option} size="compact" />
      </ChartCard>
      <ChartCard title={QUALITY_COPY.dashboard.severityTitle}>
        <LazyEChart {...severity.props} option={severity.option} size="compact" />
      </ChartCard>
    </section>
  )
}

export function Dashboard({ state }: { state: QualityDashboardState }) {
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={8} label={QUALITY_COPY.dashboard.loading} className="min-h-[32rem]" />
  }
  if (state.status === 'unavailable') {
    return (
      <EmptyState
        role="alert"
        title={QUALITY_COPY.page.workspaceUnavailableTitle}
      />
    )
  }
  if (state.status === 'error') {
    return (
      <EmptyState
        role="alert"
        title={QUALITY_COPY.dashboard.loadError}
        action={<Button type="button" variant="outline" onClick={() => void state.retry()}>{QUALITY_COPY.dashboard.retry}</Button>}
      />
    )
  }
  if (state.data.summary.submitted === 0) {
    return <EmptyState title={QUALITY_COPY.dashboard.empty} />
  }
  return <DashboardCharts data={state.data} />
}
