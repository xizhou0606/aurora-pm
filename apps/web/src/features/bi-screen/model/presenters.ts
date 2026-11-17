import { BI_SCREEN_COPY } from '@/shared/copy'

import type { TeamScoreMetric } from '../types'

const formatter = new Intl.NumberFormat('zh-CN')

export function presentNumber(value: number): string {
  return formatter.format(Number(value) || 0)
}

export function presentGeneratedTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return BI_SCREEN_COPY.page.generatedUnknown
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

export function presentScopeSummary(input: {
  count: number
  label?: string
}): string {
  if (input.count > 1) return BI_SCREEN_COPY.page.selectedCount(input.count)
  return input.label || (input.count ? BI_SCREEN_COPY.page.selectedScope : BI_SCREEN_COPY.page.allProjects)
}

export function presentTeamScoreFormulaTerm(metric: TeamScoreMetric): string {
  const copy = BI_SCREEN_COPY.team
  const weight = metric.weight_pct.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
  switch (metric.normalization) {
    case 'inverse_percent':
      return copy.scoreFormulaInverseTerm(metric.label, weight)
    case 'role_average_share':
      return copy.scoreFormulaShareTerm(metric.label, weight)
    case 'role_response_target':
      return copy.scoreFormulaResponseTerm(metric.label, weight)
    default:
      return copy.scoreFormulaTerm(metric.label, weight)
  }
}

export function presentTeamScoreFormula(metrics: readonly TeamScoreMetric[]): string {
  if (!metrics.length) return BI_SCREEN_COPY.team.scoreFormulaEmpty
  return metrics.map(presentTeamScoreFormulaTerm).join(' + ')
}
