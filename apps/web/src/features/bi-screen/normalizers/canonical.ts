import { BI_SCREEN_COPY } from '@/shared/copy'

import type {
  DashboardData,
  OwnerBoardItem,
  OverviewMetrics,
  QualityOwnerLoadItem,
  QualityTrendPoint,
  RankingItem,
  ScopeProgressItem,
  TrendPoint,
  VersionQualityRiskItem,
} from '../types'
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
import { normalizeTeamData } from './team-scoring'

function progress(value: unknown): ScopeProgressItem[] {
  return rows(value).flatMap((item) => {
    const id = integer(item.id)
    if (!id) return []
    return [{
      id,
      label: text(item.label, BI_SCREEN_COPY.fallback.unnamedProject),
      total_tasks: count(item.total_tasks),
      done_tasks: count(item.done_tasks),
      overdue_tasks: count(item.overdue_tasks),
      done_rate_pct: percent(item.done_rate_pct),
    }]
  })
}

function overview(value: unknown): OverviewMetrics {
  const source = record(value)
  return {
    total_tasks: count(source.total_tasks),
    in_flight_tasks: count(source.in_flight_tasks),
    overdue_tasks: count(source.overdue_tasks),
    due_soon_tasks: count(source.due_soon_tasks),
    unassigned_tasks: count(source.unassigned_tasks),
    current_scope_count: count(source.current_scope_count),
    open_bugs: count(source.open_bugs),
    blocker_bugs: count(source.blocker_bugs),
    high_severity_bugs: count(source.high_severity_bugs),
    pending_review_bugs: count(source.pending_review_bugs),
    overdue_open_bugs: count(source.overdue_open_bugs),
    blocked_tasks: count(source.blocked_tasks),
    done_tasks: count(source.done_tasks),
    completion_rate_pct: percent(source.completion_rate_pct),
    milestone_progress_pct: percent(source.milestone_progress_pct),
  }
}

function trend(value: unknown): TrendPoint[] {
  return rows(value).map((item) => ({
    date: text(item.date),
    label: text(item.label, text(item.date)),
    created: count(item.created),
    completed: count(item.completed),
  }))
}

function ownerBoard(value: unknown): OwnerBoardItem[] {
  return rows(value).map((item) => ({
    owner_id: integer(item.owner_id) || undefined,
    owner_name: text(item.owner_name, BI_SCREEN_COPY.ownerTable.unassigned),
    owner_avatar_url: text(item.owner_avatar_url) || undefined,
    total_tasks: count(item.total_tasks),
    overdue_tasks: count(item.overdue_tasks),
    normal_tasks: count(item.normal_tasks),
    done_tasks: count(item.done_tasks),
  }))
}

function rankings(value: unknown): RankingItem[] {
  return rows(value).flatMap((item) => {
    const milestoneId = integer(item.milestone_id)
    if (!milestoneId) return []
    return [{
      milestone_id: milestoneId,
      milestone_kind: text(item.milestone_kind),
      title: text(item.title, BI_SCREEN_COPY.fallback.unnamedMilestone),
      team_id: integer(item.team_id),
      team_name: text(item.team_name) || undefined,
      project_id: integer(item.project_id),
      project_name: text(item.project_name) || undefined,
      due_at: typeof item.due_at === 'string' ? item.due_at : null,
      status: text(item.status),
      status_label: text(item.status_label) || undefined,
      total_tasks: count(item.total_tasks),
      done_tasks: count(item.done_tasks),
      overdue_tasks: count(item.overdue_tasks),
      open_bugs: count(item.open_bugs),
      done_rate_pct: percent(item.done_rate_pct),
    }]
  })
}

function qualityTrend(value: unknown): QualityTrendPoint[] {
  return rows(value).map((item) => ({
    date: text(item.date),
    label: text(item.label, text(item.date)),
    open_bugs: count(item.open_bugs),
    high_severity_bugs: count(item.high_severity_bugs),
    resolved_bugs: count(item.resolved_bugs),
  }))
}

function versionRisks(value: unknown): VersionQualityRiskItem[] {
  return rows(value).flatMap((item) => {
    const id = integer(item.version_id)
    if (!id) return []
    return [{
      version_id: id,
      version_name: text(item.version_name, BI_SCREEN_COPY.fallback.unnamedProject),
      open_bugs: count(item.open_bugs),
      high_severity_bugs: count(item.high_severity_bugs),
      risk_level: text(item.risk_level),
      risk_label: text(item.risk_label),
    }]
  })
}

function qualityOwners(value: unknown): QualityOwnerLoadItem[] {
  return rows(value).map((item) => ({
    owner_id: integer(item.owner_id) || undefined,
    owner_name: text(item.owner_name, BI_SCREEN_COPY.ownerTable.unassigned),
    owner_avatar_url: text(item.owner_avatar_url) || undefined,
    pending_bugs: count(item.pending_bugs),
    review_bugs: count(item.review_bugs),
    overdue_bugs: count(item.overdue_bugs),
    open_bugs: count(item.open_bugs),
    high_severity_bugs: count(item.high_severity_bugs),
    high_severity_pct: percent(item.high_severity_pct),
  }))
}

function ids(value: unknown): number[] | undefined {
  if (!Array.isArray(value)) return undefined
  const result = value.map(integer).filter(Boolean)
  return result.length ? result : undefined
}

export function normalizeCanonicalDashboard(value: UnknownRecord): DashboardData | null {
  const scope = record(value.scope)
  const metrics = record(value.overview_metrics)
  if (!Object.keys(scope).length || !Object.keys(metrics).length) return null
  return {
    period: text(value.period, 'week'),
    workspace_id: integer(value.workspace_id),
    workspace_name: text(value.workspace_name) || undefined,
    team_count: count(value.team_count),
    scope: {
      mode: text(scope.mode),
      label: text(scope.label, BI_SCREEN_COPY.fallback.globalScope),
      team_id: integer(scope.team_id) || undefined,
      team_name: text(scope.team_name) || undefined,
      project_id: integer(scope.project_id) || undefined,
      project_name: text(scope.project_name) || undefined,
      project_set_ids: ids(scope.project_set_ids),
      project_ids: ids(scope.project_ids),
      requirement_ids: ids(scope.requirement_ids),
      milestone_kind: text(scope.milestone_kind) || undefined,
      milestone_id: integer(scope.milestone_id) || undefined,
      milestone_title: text(scope.milestone_title) || undefined,
      global: scope.global === true,
      global_limited: scope.global_limited === true,
      global_team_count: count(scope.global_team_count) || undefined,
      period: text(scope.period, text(value.period, 'week')),
    },
    overview_metrics: overview(metrics),
    urgency_distribution: distribution(value.urgency_distribution),
    due_distribution: distribution(value.due_distribution),
    priority_distribution: distribution(value.priority_distribution),
    type_distribution: distribution(value.type_distribution),
    completion_trend: trend(value.completion_trend),
    version_progress: progress(value.version_progress),
    requirement_progress: progress(value.requirement_progress),
    version_rankings: rankings(value.version_rankings),
    owner_board: ownerBoard(value.owner_board),
    status_progress: distribution(value.status_progress),
    quality_severity_distribution: distribution(value.quality_severity_distribution),
    quality_trend: qualityTrend(value.quality_trend),
    version_quality_risks: versionRisks(value.version_quality_risks),
    quality_owner_load: qualityOwners(value.quality_owner_load),
    team_data: normalizeTeamData(value.team_data),
    generated_at: text(value.generated_at),
  }
}