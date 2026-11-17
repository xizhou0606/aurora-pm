import type {
  TeamDimension,
  TeamDimensionOption,
  TeamMemberMetric,
  TeamMetricRow,
} from '../types'
import { BI_SCREEN_COPY } from '@/shared/copy'

export type TeamRoleKey = TeamMetricRow['role_key']

export type TeamDisplayMetricKey =
  | 'requirement_count'
  | 'bug_count'
  | 'task_count'
  | 'review_pass_rate'
  | 'first_pass_rate'
  | 'bug_close_hours'
  | 'invalid_bug_rate'
  | 'overdue_rate'
  | 'bug_task_rate'
  | 'p0_bug_count'
  | 'p1_bug_count'
  | 'bug_response_hours'

export type TeamDisplayMember = TeamMemberMetric
export type TeamDisplaySummary = TeamMetricRow

export type TeamDisplayMetric = {
  key: TeamDisplayMetricKey
  label: string
  unit: 'count' | 'percent' | 'hours'
  defaultDescending: boolean
}

export type TeamMetricValue = {
  value?: number
  samples?: number
  supportingValue?: number
}

export const TEAM_ROLE_ORDER: readonly TeamRoleKey[] = ['product', 'testing', 'development']

export const TEAM_ROLE_METRICS: Readonly<Record<TeamRoleKey, readonly TeamDisplayMetric[]>> = {
  product: [
    { key: 'requirement_count', label: BI_SCREEN_COPY.team.metricLabels.requirementCount, unit: 'count', defaultDescending: true },
    { key: 'review_pass_rate', label: BI_SCREEN_COPY.team.metricLabels.reviewPassRate, unit: 'percent', defaultDescending: true },
    { key: 'first_pass_rate', label: BI_SCREEN_COPY.team.metricLabels.firstPassRate, unit: 'percent', defaultDescending: true },
  ],
  testing: [
    { key: 'bug_count', label: BI_SCREEN_COPY.team.metricLabels.bugCount, unit: 'count', defaultDescending: true },
    { key: 'bug_close_hours', label: BI_SCREEN_COPY.team.metricLabels.bugCloseHours, unit: 'hours', defaultDescending: false },
    { key: 'invalid_bug_rate', label: BI_SCREEN_COPY.team.metricLabels.invalidBugRate, unit: 'percent', defaultDescending: false },
  ],
  development: [
    { key: 'task_count', label: BI_SCREEN_COPY.team.metricLabels.taskCount, unit: 'count', defaultDescending: true },
    { key: 'overdue_rate', label: BI_SCREEN_COPY.team.metricLabels.overdueRate, unit: 'percent', defaultDescending: false },
    { key: 'p0_bug_count', label: BI_SCREEN_COPY.team.metricLabels.p0BugCount, unit: 'count', defaultDescending: true },
    { key: 'p1_bug_count', label: BI_SCREEN_COPY.team.metricLabels.p1BugCount, unit: 'count', defaultDescending: true },
    { key: 'bug_response_hours', label: BI_SCREEN_COPY.team.metricLabels.bugResponseHours, unit: 'hours', defaultDescending: false },
  ],
}

function isoWeekStart(key: string): Date | undefined {
  const match = key.match(/^(\d{4})-W(\d{2})$/)
  if (!match) return undefined
  const year = Number(match[1])
  const week = Number(match[2])
  if (year < 1 || week < 1 || week > 53) return undefined
  const january4 = new Date(Date.UTC(year, 0, 4))
  const weekday = january4.getUTCDay() || 7
  january4.setUTCDate(january4.getUTCDate() - weekday + 1 + (week - 1) * 7)
  const [actualYear, actualWeek] = (() => {
    const thursday = new Date(january4)
    thursday.setUTCDate(thursday.getUTCDate() + 3)
    const isoYear = thursday.getUTCFullYear()
    const firstThursday = new Date(Date.UTC(isoYear, 0, 4))
    const firstWeekday = firstThursday.getUTCDay() || 7
    firstThursday.setUTCDate(firstThursday.getUTCDate() - firstWeekday + 4)
    return [isoYear, 1 + Math.round((thursday.getTime() - firstThursday.getTime()) / 604_800_000)]
  })()
  return actualYear === year && actualWeek === week ? january4 : undefined
}

function shortDate(value: Date): string {
  return `${String(value.getUTCMonth() + 1).padStart(2, '0')}.${String(value.getUTCDate()).padStart(2, '0')}`
}

export function teamDimensionOptionLabel(
  dimension: TeamDimension,
  option: TeamDimensionOption,
): string {
  if (dimension !== 'week') return option.label
  const start = isoWeekStart(option.key)
  if (!start) return option.label
  const end = new Date(start)
  end.setUTCDate(end.getUTCDate() + 6)
  return BI_SCREEN_COPY.team.weekOptionLabel(option.label, shortDate(start), shortDate(end))
}

function optionalNumber(value: number | undefined, samples?: number): TeamMetricValue {
  return typeof value === 'number' && Number.isFinite(value) && (samples === undefined || samples > 0)
    ? { value, samples }
    : { samples }
}

function countMetric(value: number | undefined): TeamMetricValue {
	return typeof value === 'number' && Number.isFinite(value) ? { value, samples: value } : {}
}

export function teamMetricValue(
  row: TeamDisplayMember | TeamDisplaySummary,
  metric: TeamDisplayMetric,
): TeamMetricValue {
  switch (metric.key) {
    case 'requirement_count':
      return countMetric(row.requirement_count)
    case 'bug_count':
      return countMetric(row.bug_count)
    case 'task_count':
      return countMetric(row.task_count)
    case 'review_pass_rate':
      return optionalNumber(row.review_pass_rate_pct, row.review_pass_sample_size)
    case 'first_pass_rate':
      return optionalNumber(row.first_pass_rate_pct, row.first_pass_sample_size)
    case 'bug_close_hours':
      return optionalNumber(row.bug_close_avg_hours, row.bug_close_sample_size)
    case 'invalid_bug_rate':
      return optionalNumber(
        row.invalid_bug_rate_pct,
        row.invalid_bug_sample_size,
      )
    case 'overdue_rate':
      return optionalNumber(
        row.overdue_rate_pct,
        row.overdue_sample_size,
      )
    case 'bug_task_rate':
      return optionalNumber(
        row.bug_task_rate_pct,
        row.bug_task_sample_size,
      )
    case 'p0_bug_count': {
      const result = optionalNumber(row.p0_bug_count)
      return { ...result, supportingValue: row.team_p0_share_pct }
    }
    case 'p1_bug_count': {
      const result = optionalNumber(row.p1_bug_count)
      return { ...result, supportingValue: row.team_p1_share_pct }
    }
    case 'bug_response_hours':
      return optionalNumber(row.bug_response_avg_hours, row.bug_response_sample_size)
  }
}

export function isSeverityShareMetric(metric: TeamDisplayMetric): boolean {
  return metric.key === 'p0_bug_count' || metric.key === 'p1_bug_count'
}

export function sortTeamMembers(
  members: readonly TeamDisplayMember[],
  metric: TeamDisplayMetric,
  descending: boolean,
): TeamDisplayMember[] {
  return [...members].sort((left, right) => {
    const leftValue = teamMetricValue(left, metric).value
    const rightValue = teamMetricValue(right, metric).value
    const leftMissing = leftValue === undefined
    const rightMissing = rightValue === undefined
    if (leftMissing !== rightMissing) return leftMissing ? 1 : -1
    if (leftValue !== rightValue && leftValue !== undefined && rightValue !== undefined) {
      return (leftValue - rightValue) * (descending ? -1 : 1)
    }
    return left.user_name.localeCompare(right.user_name, 'zh-CN') || left.user_id - right.user_id
  })
}
