import { BI_SCREEN_COPY } from '@/shared/copy'

import {
  TEAM_ROLE_METRICS,
  TEAM_ROLE_ORDER,
	isSeverityShareMetric,
  teamMetricValue,
  type TeamDisplayMetric,
  type TeamDisplaySummary,
} from '../model/team-display'

function presentMetricValue(row: TeamDisplaySummary, metric: TeamDisplayMetric) {
  const copy = BI_SCREEN_COPY.team
  const result = teamMetricValue(row, metric)
  if (result.value === undefined) return copy.noSample
  const value = metric.unit === 'hours'
    ? copy.hours(result.value)
    : metric.unit === 'percent'
      ? copy.percent(result.value)
      : result.value.toLocaleString('zh-CN')
  return isSeverityShareMetric(metric) && result.supportingValue !== undefined
    ? `${value} · ${copy.percent(result.supportingValue)}`
    : value
}

export function TeamSummaryTable({ rows }: { rows: readonly TeamDisplaySummary[] }) {
  const copy = BI_SCREEN_COPY.team
  return (
    <section aria-labelledby="bi-screen-team-summary-title" className="grid min-w-0 gap-3">
      <header>
        <h2 id="bi-screen-team-summary-title" className="m-0 text-sm font-semibold">{copy.summaryTitle}</h2>
      </header>
      <div className="grid min-w-0 gap-3 lg:grid-cols-3">
        {TEAM_ROLE_ORDER.map((role) => {
          const row = rows.find((item) => item.role_key === role)
          return (
            <article key={role} className="min-w-0 rounded-lg border border-border bg-card p-4">
              <header className="flex items-start justify-between gap-3 border-b border-border pb-3">
                <h3 className="m-0 text-base font-semibold">{copy.roleLabels[role]}</h3>
                <span className="text-xs tabular-nums text-muted-foreground">
                  {row ? copy.chartMeta(row.member_count) : copy.noSample}
                </span>
              </header>
              <dl className="mt-3 mb-0 grid gap-2">
                {TEAM_ROLE_METRICS[role].map((metric) => (
                  <div key={metric.key} className="flex min-w-0 items-baseline justify-between gap-3">
                    <dt className="truncate text-xs text-muted-foreground">{metric.label}</dt>
                    <dd className="m-0 shrink-0 text-sm font-semibold tabular-nums">
                      {row ? presentMetricValue(row, metric) : copy.noSample}
                    </dd>
                  </div>
                ))}
              </dl>
            </article>
          )
        })}
      </div>
    </section>
  )
}
