import type { PMScopeOptions } from '@/shared/api'
import type { TeamData } from './team'

export type Period = 'week' | 'month'

export type ScreenTab = 'overview' | 'team'

export type Scope = {
  projectSetIds: readonly number[]
  projectIds: readonly number[]
  requirementIds: readonly number[]
}

export type RouteState = {
  scope: Scope
  period: Period
  tab: ScreenTab
  teamDimension: import('./team').TeamDimension
  teamDimensionKey?: string
  teamView: import('./team').TeamView
  invalidScope: boolean
}

export type ScopeState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'invalid'; reset: () => void }
  | { status: 'ready'; data: PMScopeOptions }

export type DashboardScope = {
  mode: string
  label: string
  team_id?: number
  team_name?: string
  project_id?: number
  project_name?: string
  project_set_ids?: number[]
  project_ids?: number[]
  requirement_ids?: number[]
  milestone_kind?: string
  milestone_id?: number
  milestone_title?: string
  global: boolean
  global_limited: boolean
  global_team_count?: number
  period: string
}

export type OverviewMetrics = {
  total_tasks: number
  in_flight_tasks: number
  overdue_tasks: number
  due_soon_tasks: number
  unassigned_tasks: number
  current_scope_count: number
  open_bugs: number
  blocker_bugs: number
  high_severity_bugs: number
  pending_review_bugs: number
  overdue_open_bugs: number
  blocked_tasks: number
  done_tasks: number
  completion_rate_pct: number
  milestone_progress_pct: number
}

export type TrendPoint = {
  date: string
  label: string
  created: number
  completed: number
}

export type ScopeProgressItem = {
  id: number
  label: string
  total_tasks: number
  done_tasks: number
  overdue_tasks: number
  done_rate_pct: number
}

export type DistributionItem = {
  key: string
  label: string
  count: number
  percent: number
  tone?: string
}

export type RankingItem = {
  milestone_id: number
  milestone_kind: string
  title: string
  team_id: number
  team_name?: string
  project_id: number
  project_name?: string
  due_at?: string | null
  status: string
  status_label?: string
  total_tasks: number
  done_tasks: number
  overdue_tasks: number
  open_bugs: number
  done_rate_pct: number
}

export type OwnerBoardItem = {
  owner_id?: number
  owner_name: string
  owner_avatar_url?: string
  total_tasks: number
  overdue_tasks: number
  normal_tasks: number
  done_tasks: number
}

export type QualityTrendPoint = {
  date: string
  label: string
  open_bugs: number
  high_severity_bugs: number
  resolved_bugs: number
}

export type VersionQualityRiskItem = {
  version_id: number
  version_name: string
  open_bugs: number
  high_severity_bugs: number
  risk_level: string
  risk_label: string
}

export type QualityOwnerLoadItem = {
  owner_id?: number
  owner_name: string
  owner_avatar_url?: string
  pending_bugs: number
  review_bugs: number
  overdue_bugs: number
  open_bugs: number
  high_severity_bugs: number
  high_severity_pct: number
}

export type DashboardData = {
  period: string
  workspace_id: number
  workspace_name?: string
  team_count: number
  scope: DashboardScope
  overview_metrics: OverviewMetrics
  urgency_distribution: DistributionItem[]
  due_distribution: DistributionItem[]
  priority_distribution: DistributionItem[]
  type_distribution: DistributionItem[]
  completion_trend: TrendPoint[]
  version_progress: ScopeProgressItem[]
  requirement_progress: ScopeProgressItem[]
  version_rankings: RankingItem[]
  owner_board: OwnerBoardItem[]
  status_progress: DistributionItem[]
  quality_severity_distribution: DistributionItem[]
  quality_trend: QualityTrendPoint[]
  version_quality_risks: VersionQualityRiskItem[]
  quality_owner_load: QualityOwnerLoadItem[]
  team_data: TeamData
  generated_at: string
}

export type ReadyRefresh = 'idle' | 'refreshing' | 'stale_error'

export type DashboardState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty'; retry: () => void }
  | { status: 'ready'; data: DashboardData; refresh: ReadyRefresh; retry: () => void }

export type Drilldown =
  | {
      domain: 'tasks'
      scope?: Partial<Scope>
      query?: Readonly<Record<string, string>>
    }
  | {
      domain: 'quality'
      scope?: Partial<Scope>
      query?: Readonly<Record<string, string>>
    }

export type ChartPoint = {
  label: string
  values: readonly { series: string; value: number }[]
  drilldown?: Drilldown
}

export type ChartModel = {
  id: string
  title: string
  description: string
  meta: string
  ariaLabel: string
  emptyTitle: string
  emptyDescription: string
  option: Readonly<Record<string, unknown>> | null
  summaryCaption: string
  summaryColumns: readonly string[]
  summaryRows: readonly (readonly (string | number)[])[]
  points: readonly ChartPoint[]
  size?: 'compact' | 'medium'
  layout?: 'standard' | 'wide'
}

export type VersionMatrixTone = 'positive' | 'info' | 'warning' | 'danger'

export type VersionMatrixModule = {
  id: number
  label: string
  pct: number
  done: number
  remaining: number
  overdue: number
  statusLabel: string
  /** Accent for the card top line and progress fill; follows completion. */
  tone: VersionMatrixTone
  /** Status pill tone; turns danger whenever the version has overdue tasks. */
  statusTone: VersionMatrixTone
  drilldown: Drilldown
}

export type VersionMatrixModel = {
  id: string
  title: string
  description: string
  meta: string
  ariaLabel: string
  emptyTitle: string
  emptyDescription: string
  modules: readonly VersionMatrixModule[]
}

export type CompositionGroup = {
  id: string
  label: string
  total: number
  items: readonly DistributionItem[]
  /** When set, segment clicks emit `{ domain: 'tasks', query: { [drilldownQuery]: item.key } }`. */
  drilldownQuery?: 'filter_kinds'
}

export type CompositionModel = {
  id: string
  title: string
  description: string
  meta: string
  ariaLabel: string
  emptyTitle: string
  emptyDescription: string
  groups: readonly CompositionGroup[]
  summaryCaption: string
  summaryColumns: readonly string[]
  summaryRows: readonly (readonly (string | number)[])[]
}

export type CommandLamp = {
  id: string
  label: string
  value: number
  hint: string
  tone: 'negative' | 'warning'
}

export type CommandOverview = {
  completionPct: number
  totalTasks: number
  doneTasks: number
  inFlightTasks: number
  recentCompleted: number
  delta: { direction: 'up' | 'down' | 'flat'; value: number } | null
  lamps: readonly CommandLamp[]
}