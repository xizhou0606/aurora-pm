import { PhSlidersHorizontal } from '@/shared/ui/icons'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { Button, ViewTabs } from '@/shared/ui'

import type { TeamMetricRow, TeamScoreProfile } from '../types'
import { presentTeamScoreFormula } from '../model/presenters'

const ROLE_ORDER: TeamMetricRow['role_key'][] = ['product', 'testing', 'development']

export function TeamScoreFormula({ profile, role, onRoleChange, onEdit }: {
  profile: TeamScoreProfile
  role: TeamMetricRow['role_key']
  onRoleChange: (role: TeamMetricRow['role_key']) => void
  onEdit: () => void
}) {
  const copy = BI_SCREEN_COPY.team
  const metrics = profile.items.filter((metric) => metric.role_key === role && metric.enabled)
  return (
    <section aria-labelledby="bi-screen-team-formula-title" className="grid gap-3 rounded-lg border border-border bg-card p-4">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 id="bi-screen-team-formula-title" className="m-0 text-sm font-semibold">{copy.formulaTitle}</h2>
          <p className="mt-1 mb-0 text-xs text-muted-foreground">{copy.formulaDescription}</p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={onEdit}>
          <PhSlidersHorizontal aria-hidden="true" />{copy.settings}
        </Button>
      </header>
      <ViewTabs
        value={role}
        label={copy.formulaAriaLabel}
        tabs={ROLE_ORDER.map((roleKey) => ({ id: roleKey, label: copy.roleLabels[roleKey] }))}
        onValueChange={(value) => onRoleChange(value as TeamMetricRow['role_key'])}
      />
      <p className="m-0 rounded-lg bg-muted/50 px-3 py-2 text-sm text-foreground" aria-live="polite">
        <strong>{copy.scoreFormulaPrefix}</strong>{' '}{presentTeamScoreFormula(metrics)}
      </p>
      <div className="grid gap-2 lg:grid-cols-2">
        {metrics.map((metric) => (
          <article key={`${metric.role_key}:${metric.metric_key}`} className="grid gap-1 rounded-lg border border-border p-3 text-sm">
            <div className="flex items-center justify-between gap-3">
              <strong>{metric.label}</strong>
              <span className="font-semibold tabular-nums">{copy.percent(metric.weight_pct)}</span>
            </div>
            <p className="m-0 text-xs text-muted-foreground">{metric.formula}</p>
            <p className="m-0 text-xs text-muted-foreground">{metric.direction === 'lower_better' ? copy.lowerBetter : copy.higherBetter}</p>
          </article>
        ))}
      </div>
    </section>
  )
}
