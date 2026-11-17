export type QualityDashboardSummary = {
  submitted: number
  resolved: number
  closed: number
  backlog: number
}

export type QualityDashboardPerson = QualityDashboardSummary & {
  userId: number
  userName: string
  avatarUrl?: string
}

export type QualityDashboardDimension = {
  key: string
  label: string
  count: number
}

export type QualityDashboard = {
  summary: QualityDashboardSummary
  byPerson: QualityDashboardPerson[]
  byStatus: QualityDashboardDimension[]
  bySeverity: QualityDashboardDimension[]
}

export type QualityDashboardState =
  | { status: 'loading' }
  | { status: 'unavailable'; reason: 'workspace_access' }
  | { status: 'error'; error: unknown; retry: () => Promise<void> }
  | {
      status: 'ready'
      data: QualityDashboard
      refresh: 'idle' | 'refreshing' | 'stale_error'
    }
