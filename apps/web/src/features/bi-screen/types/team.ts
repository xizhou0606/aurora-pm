export type TeamDimension = 'version' | 'month' | 'week'

export type TeamView = 'data' | 'chart'

export type TeamDimensionOption = {
  key: string
  label: string
}

export type TeamDimensionControl = {
  value: TeamDimension
  loading: boolean
  onChange: (value: TeamDimension) => void
}

export type TeamSliceControl = {
  value?: string
  options: readonly TeamDimensionOption[]
  loading: boolean
  onChange: (value?: string) => void
}

export type TeamMetricRow = {
  role_key: 'product' | 'development' | 'testing'
  role_label: string
  member_count: number
  workload: number
  completed_workload: number
  avg_response_hours: number
  response_sample_size: number
  on_time_rate_pct: number
  on_time_sample_size: number
  quality_risk_rate_pct: number
  quality_risk_sample_size: number
  rework_count: number
  rework_rate_pct: number
  rework_sample_size: number
  first_pass_rate_pct: number
  first_pass_sample_size?: number
  requirement_count?: number
  review_pass_rate_pct?: number
  review_pass_sample_size?: number
  bug_count?: number
  bug_close_avg_hours?: number
  bug_close_sample_size?: number
  invalid_bug_rate_pct?: number
  invalid_bug_sample_size?: number
  task_count?: number
  overdue_rate_pct?: number
  overdue_sample_size?: number
  bug_task_rate_pct?: number
  bug_task_sample_size?: number
  p0_bug_count?: number
  team_p0_share_pct?: number
  p1_bug_count?: number
  team_p1_share_pct?: number
  bug_response_avg_hours?: number
  completion_rate_pct: number
  workload_share_pct: number
  bug_response_rate_pct: number
  bug_response_sample_size: number
  composite_score?: number
  score_eligible: boolean
  score_coverage_pct: number
  score_breakdown: TeamScoreBreakdown[]
  score_ineligible_reasons: Array<'workload_below_minimum' | 'coverage_below_minimum' | 'required_metric_missing'>
  score_missing_metric_keys: string[]
}

export type TeamMemberMetric = Omit<TeamMetricRow, 'role_key' | 'role_label' | 'member_count'> & {
  user_id: number
  user_name: string
  user_avatar_url?: string
}

export type TeamMemberGroup = {
  role_key: TeamMetricRow['role_key']
  role_label: string
  members: TeamMemberMetric[]
}

export type TeamMemberItemType = 'requirement' | 'task' | 'bug'

export type TeamMemberDrilldownItem = {
  id: number
  key?: string
  title: string
  status: string
  priority?: string
  severity?: string
  project_id?: number
  requirement_id?: number
  created_at: string
  due_at?: string
  effective_due_at?: string
  due_at_source?: 'self' | 'parent'
  due_at_source_task_id?: number
  delivery_at?: string
  completed_at?: string
  overdue?: boolean
}

export type TeamMemberDrilldownData = {
  role: TeamMetricRow['role_key']
  item_type: TeamMemberItemType
  user_id: number
  dimension: TeamDimension
  dimension_key?: string
  total: number
  items: TeamMemberDrilldownItem[]
}

export type TeamScoreBreakdown = {
  metric_key: string
  raw_value?: number
  normalized_score?: number
  configured_weight_pct: number
  effective_weight_pct: number
  sample_size: number
  eligible: boolean
}

export type TeamScoreMetric = {
  metric_key: string
  role_key: TeamMetricRow['role_key']
  label: string
  description: string
  formula: string
  unit: 'percent' | 'hours' | 'count'
  direction: 'higher_better' | 'lower_better'
  normalization: string
  applicable_roles: TeamMetricRow['role_key'][]
  definition_version: number
  status: 'active' | 'deprecated'
  enabled: boolean
  weight_bps: number
  weight_pct: number
  missing_policy: 'renormalize' | 'ineligible'
  minimum_samples: number
  sort_order: number
}

export type TeamScoreProfile = {
  id: number
  version: number
  source: 'default' | 'workspace'
  can_manage: boolean
  total_weight_bps: number
  minimum_coverage_bps: number
  minimum_workload: number
  items: TeamScoreMetric[]
}

export type TeamScoreProfileUpdate = {
  expected_version: number
  reset_to_default?: boolean
  items: Array<Pick<
    TeamScoreMetric,
    'metric_key' | 'role_key' | 'enabled' | 'weight_bps' | 'missing_policy' | 'minimum_samples'
  >>
}

export type TeamData = {
  dimension: TeamDimension
  dimension_key?: string
  dimension_options: TeamDimensionOption[]
  summary_rows: TeamMetricRow[]
  member_groups: TeamMemberGroup[]
  score_profile: TeamScoreProfile
  metric_notes: string[]
}

export type TeamMetricApprovalNodeOption = {
  node_id: string
  node_name: string
}

export type TeamMetricWorkflowOption = {
  template_id: number
  template_name: string
  published_version: number
  approval_nodes: TeamMetricApprovalNodeOption[]
}

export type TeamMetricSettings = {
  version: number
  can_manage: boolean
  configured: boolean
  valid: boolean
  requirement_workflow_template_id?: number
  requirement_workflow_template_name?: string
  requirement_workflow_published_version?: number
  approval_node_id?: string
  approval_node_name?: string
  updated_by_user_id?: number
  options: TeamMetricWorkflowOption[]
}

export type TeamMetricSettingsUpdate = {
  expected_version: number
  requirement_workflow_template_id: number
  approval_node_id: string
}
