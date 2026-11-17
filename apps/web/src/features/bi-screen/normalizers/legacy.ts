import { BI_SCREEN_COPY } from '@/shared/copy'

import type { DashboardData } from '../types'
import {
  count,
  distribution,
  integer,
  percent,
  record,
  rows,
  text,
  type UnknownRecord,
} from './helpers'

export function normalizeLegacyDashboard(value: UnknownRecord): DashboardData | null {
  const kpi = record(value.kpi)
  if (!Object.keys(kpi).length) return null
  const projects = rows(value.projects).flatMap((item) => {
    const id = integer(item.project_id)
    if (!id) return []
    const total = count(item.tasks_total)
    const rate = percent(item.done_rate_pct)
    return [{
      id,
      label: text(item.project_name, BI_SCREEN_COPY.fallback.unnamedProject),
      total_tasks: total,
      done_tasks: Math.round((total * rate) / 100),
      overdue_tasks: count(item.overdue),
      done_rate_pct: rate,
    }]
  })
  const totalTasks = projects.reduce((total, item) => total + item.total_tasks, 0)
  const doneTasks = projects.reduce((total, item) => total + item.done_tasks, 0)
  const inFlight = count(kpi.tasks_in_flight)
  const overdue = count(kpi.overdue_open)
  const dueSoon = count(kpi.tasks_due_soon)
  const high = count(kpi.p0_open)
  const completed = rows(value.sparklines).find((item) => item.key === 'completed')
  const labels = Array.isArray(completed?.weeks) ? completed.weeks : []
  const values = Array.isArray(completed?.values) ? completed.values : []
  return {
    period: text(value.period, 'week'),
    workspace_id: integer(value.workspace_id),
    workspace_name: text(value.workspace_name) || undefined,
    team_count: count(value.team_count),
    scope: {
      mode: 'global',
      label: text(value.workspace_name, BI_SCREEN_COPY.fallback.globalScope),
      global: true,
      global_limited: false,
      global_team_count: count(value.team_count),
      period: text(value.period, 'week'),
    },
    overview_metrics: {
      total_tasks: totalTasks,
      in_flight_tasks: inFlight,
      overdue_tasks: overdue,
      due_soon_tasks: dueSoon,
      unassigned_tasks: 0,
      current_scope_count: count(value.project_total) || projects.length,
      open_bugs: high,
      blocker_bugs: high,
      high_severity_bugs: high,
      pending_review_bugs: 0,
      overdue_open_bugs: 0,
      blocked_tasks: count(record(value.summary).risk_high),
      done_tasks: doneTasks,
      completion_rate_pct: totalTasks ? percent((doneTasks / totalTasks) * 100) : 0,
      milestone_progress_pct: 0,
    },
    urgency_distribution: [],
    due_distribution: distribution([
      { key: 'overdue', label: BI_SCREEN_COPY.fallback.overdue, count: overdue },
      { key: 'due_soon', label: BI_SCREEN_COPY.fallback.dueSoon, count: dueSoon },
      {
        key: 'scheduled',
        label: BI_SCREEN_COPY.fallback.scheduled,
        count: Math.max(0, inFlight - overdue - dueSoon),
      },
    ]),
    priority_distribution: [],
    type_distribution: [],
    completion_trend: values.map((item, index) => ({
      date: text(labels[index]),
      label: text(labels[index], BI_SCREEN_COPY.fallback.periodLabel(index + 1)),
      created: 0,
      completed: count(item),
    })),
    version_progress: projects,
    requirement_progress: [],
    version_rankings: [],
    owner_board: [],
    status_progress: distribution(value.status_mix),
    quality_severity_distribution: high
      ? [{ key: 'high', label: BI_SCREEN_COPY.fallback.highSeverity, count: high, percent: 100 }]
      : [],
    quality_trend: [],
    version_quality_risks: [],
    quality_owner_load: [],
    team_data: {
      dimension: 'version',
      dimension_options: [],
      summary_rows: [],
      member_groups: [],
		score_profile: {
			id: 0, version: 0, source: 'default', can_manage: false,
			total_weight_bps: 10_000, minimum_coverage_bps: 7_000,
			minimum_workload: 3, items: [],
		},
      metric_notes: [],
    },
    generated_at: text(value.generated_at),
  }
}
