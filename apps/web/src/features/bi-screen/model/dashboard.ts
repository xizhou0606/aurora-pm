import type { DashboardData, DashboardState } from '../types'

export function dashboardIsEmpty(data: DashboardData): boolean {
  const metrics = data.overview_metrics
  return metrics.total_tasks === 0
    && metrics.open_bugs === 0
    && data.completion_trend.length === 0
    && data.version_progress.length === 0
    && data.requirement_progress.length === 0
    && data.owner_board.length === 0
    && data.quality_trend.length === 0
    && data.version_quality_risks.length === 0
    && data.quality_owner_load.length === 0
}

export function dashboardState(input: {
  data: DashboardData | undefined
  query: 'pending' | 'failed' | 'settled'
  refresh: 'idle' | 'refreshing' | 'failed'
  retry: () => void
}): DashboardState {
  if (!input.data) {
    if (input.query === 'pending') return { status: 'loading' }
    return { status: 'error', retry: input.retry }
  }
  if (dashboardIsEmpty(input.data)) return { status: 'empty', retry: input.retry }
  return {
    status: 'ready',
    data: input.data,
    refresh: input.refresh === 'failed'
      ? 'stale_error'
      : input.refresh === 'refreshing'
        ? 'refreshing'
        : 'idle',
    retry: input.retry,
  }
}
