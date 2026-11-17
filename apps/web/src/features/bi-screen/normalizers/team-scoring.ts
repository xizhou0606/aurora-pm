import { BI_SCREEN_COPY } from '@/shared/copy'

import type {
  TeamData,
  TeamMemberMetric,
  TeamMetricRow,
  TeamScoreBreakdown,
  TeamScoreMetric,
  TeamScoreProfile,
} from '../types'
import {
  count,
  integer,
  percent,
  record,
  rows,
  text,
  type UnknownRecord,
} from './helpers'

function roleKey(value: unknown): TeamMetricRow['role_key'] | undefined {
  return value === 'product' || value === 'development' || value === 'testing'
    ? value
    : undefined
}

function optionalNumber(value: unknown): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function scoreBreakdown(value: unknown): TeamScoreBreakdown[] {
  return rows(value).flatMap((item) => {
    const metricKey = text(item.metric_key)
    if (!metricKey) return []
    return [{
      metric_key: metricKey,
      raw_value: optionalNumber(item.raw_value),
      normalized_score: optionalNumber(item.normalized_score),
      configured_weight_pct: percent(item.configured_weight_pct),
      effective_weight_pct: percent(item.effective_weight_pct),
      sample_size: count(item.sample_size),
      eligible: item.eligible === true,
    }]
  })
}

function teamMetric(item: UnknownRecord): Omit<TeamMetricRow, 'role_key' | 'role_label'> {
  return {
    member_count: count(item.member_count),
    workload: count(item.workload),
    completed_workload: count(item.completed_workload),
    avg_response_hours: Math.round(count(item.avg_response_hours) * 100) / 100,
    response_sample_size: count(item.response_sample_size),
    on_time_rate_pct: percent(item.on_time_rate_pct),
    on_time_sample_size: count(item.on_time_sample_size),
    quality_risk_rate_pct: percent(item.quality_risk_rate_pct),
    quality_risk_sample_size: count(item.quality_risk_sample_size),
    rework_count: count(item.rework_count),
    rework_rate_pct: percent(item.rework_rate_pct),
    rework_sample_size: count(item.rework_sample_size),
    first_pass_rate_pct: percent(item.first_pass_rate_pct),
    first_pass_sample_size: count(item.first_pass_sample_size),
    requirement_count: count(item.requirement_count),
    review_pass_rate_pct: optionalNumber(item.review_pass_rate_pct),
    review_pass_sample_size: count(item.review_pass_sample_size),
    bug_count: count(item.bug_count),
    bug_close_avg_hours: optionalNumber(item.bug_close_avg_hours),
    bug_close_sample_size: count(item.bug_close_sample_size),
    invalid_bug_rate_pct: optionalNumber(item.invalid_bug_rate_pct),
    invalid_bug_sample_size: count(item.invalid_bug_sample_size),
    task_count: count(item.task_count),
    overdue_rate_pct: optionalNumber(item.overdue_rate_pct),
    overdue_sample_size: count(item.overdue_sample_size),
    bug_task_rate_pct: optionalNumber(item.bug_task_rate_pct),
    bug_task_sample_size: count(item.bug_task_sample_size),
    p0_bug_count: count(item.p0_bug_count),
    team_p0_share_pct: optionalNumber(item.team_p0_share_pct),
    p1_bug_count: count(item.p1_bug_count),
    team_p1_share_pct: optionalNumber(item.team_p1_share_pct),
    bug_response_avg_hours: optionalNumber(item.bug_response_avg_hours),
    completion_rate_pct: percent(item.completion_rate_pct),
    workload_share_pct: percent(item.workload_share_pct),
    bug_response_rate_pct: percent(item.bug_response_rate_pct),
    bug_response_sample_size: count(item.bug_response_sample_size),
    composite_score: optionalNumber(item.composite_score),
    score_eligible: item.score_eligible === true,
    score_coverage_pct: percent(item.score_coverage_pct),
    score_breakdown: scoreBreakdown(item.score_breakdown),
    score_ineligible_reasons: Array.isArray(item.score_ineligible_reasons)
      ? item.score_ineligible_reasons.filter((reason): reason is 'workload_below_minimum' | 'coverage_below_minimum' | 'required_metric_missing' => (
        reason === 'workload_below_minimum' || reason === 'coverage_below_minimum' || reason === 'required_metric_missing'
      ))
      : [],
    score_missing_metric_keys: Array.isArray(item.score_missing_metric_keys)
      ? item.score_missing_metric_keys.map((key) => text(key)).filter(Boolean)
      : [],
  }
}

function scoreMetric(item: UnknownRecord, metricRole: TeamMetricRow['role_key']): TeamScoreMetric | null {
  const metricKey = text(item.metric_key)
  if (!metricKey) return null
  const unit = item.unit === 'hours' || item.unit === 'count' ? item.unit : 'percent'
  const direction = item.direction === 'lower_better' ? 'lower_better' : 'higher_better'
  const applicableRoles = Array.isArray(item.applicable_roles)
    ? item.applicable_roles.map(roleKey).filter((role): role is TeamMetricRow['role_key'] => Boolean(role))
    : []
  return {
    metric_key: metricKey,
    role_key: metricRole,
    label: text(item.label, metricKey),
    description: text(item.description),
    formula: text(item.formula, text(item.description)),
    unit,
    direction,
    normalization: text(item.normalization),
    applicable_roles: applicableRoles.length ? applicableRoles : [metricRole],
    definition_version: count(item.definition_version),
    status: item.status === 'deprecated' ? 'deprecated' : 'active',
    enabled: item.enabled === true,
    weight_bps: count(item.weight_bps),
    weight_pct: percent(item.weight_pct),
    missing_policy: item.missing_policy === 'ineligible' ? 'ineligible' : 'renormalize',
    minimum_samples: count(item.minimum_samples),
    sort_order: count(item.sort_order),
  }
}

function scoreProfile(value: unknown): TeamScoreProfile {
  const source = record(value)
  const items = rows(source.items).flatMap((item) => {
    const explicitRole = roleKey(item.role_key)
    const applicableRoles = Array.isArray(item.applicable_roles)
      ? item.applicable_roles.map(roleKey).filter((role): role is TeamMetricRow['role_key'] => Boolean(role))
      : []
    const roles = explicitRole ? [explicitRole] : applicableRoles
    return roles.map((role) => scoreMetric(item, role)).filter((metric): metric is TeamScoreMetric => Boolean(metric))
  })
  return {
    id: integer(source.id),
    version: count(source.version),
    source: source.source === 'workspace' ? 'workspace' : 'default',
    can_manage: source.can_manage === true,
    total_weight_bps: count(source.total_weight_bps),
    minimum_coverage_bps: count(source.minimum_coverage_bps),
    minimum_workload: count(source.minimum_workload),
    items,
  }
}

function teamMember(item: UnknownRecord): TeamMemberMetric | null {
  const userId = integer(item.user_id)
  if (!userId) return null
  const metric = teamMetric(item)
  const { member_count: _memberCount, ...memberMetric } = metric
  return {
    ...memberMetric,
    user_id: userId,
    user_name: text(item.user_name, BI_SCREEN_COPY.fallback.user(userId)),
    user_avatar_url: text(item.user_avatar_url) || undefined,
  }
}

export function normalizeTeamData(value: unknown): TeamData {
  const source = record(value)
  const dimension = source.dimension === 'month' || source.dimension === 'week'
    ? source.dimension
    : 'version'
  return {
    dimension,
    dimension_key: text(source.dimension_key) || undefined,
    dimension_options: rows(source.dimension_options).flatMap((item) => {
      const key = text(item.key)
      return key ? [{ key, label: text(item.label, key) }] : []
    }),
    summary_rows: rows(source.summary_rows).flatMap((item) => {
      const key = roleKey(item.role_key)
      return key ? [{
        role_key: key,
        role_label: text(item.role_label, key),
        ...teamMetric(item),
      }] : []
    }),
    member_groups: rows(source.member_groups).flatMap((item) => {
      const key = roleKey(item.role_key)
      if (!key) return []
      return [{
        role_key: key,
        role_label: text(item.role_label, key),
        members: rows(item.members).map(teamMember).filter((member): member is TeamMemberMetric => Boolean(member)),
      }]
    }),
    score_profile: scoreProfile(source.score_profile),
    metric_notes: Array.isArray(source.metric_notes)
      ? source.metric_notes.map((item) => text(item)).filter(Boolean)
      : [],
  }
}
