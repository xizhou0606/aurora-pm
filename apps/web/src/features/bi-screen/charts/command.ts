import { BI_SCREEN_COPY } from '@/shared/copy'

import type { CommandLamp, CommandOverview, DashboardData } from '../types'
import { safeCount } from './core'

const RECENT_WINDOW = 7

export function commandOverview(data: DashboardData): CommandOverview {
  const copy = BI_SCREEN_COPY.command
  const metrics = data.overview_metrics
  const trend = data.completion_trend
  const recent = trend.slice(-RECENT_WINDOW)
  const previous = trend.slice(-RECENT_WINDOW * 2, -RECENT_WINDOW)
  const recentCompleted = recent.reduce((total, item) => total + safeCount(item.completed), 0)
  const previousCompleted = previous.reduce((total, item) => total + safeCount(item.completed), 0)
  const lamps: readonly CommandLamp[] = [
    { id: 'blocker', label: copy.blocker, value: safeCount(metrics.blocker_bugs), hint: copy.lampHint(safeCount(metrics.blocker_bugs)), tone: 'negative' },
    { id: 'overdue', label: BI_SCREEN_COPY.metrics.overdueOpen, value: safeCount(metrics.overdue_open_bugs), hint: BI_SCREEN_COPY.metrics.overdueOpenHint, tone: 'negative' },
    { id: 'high-severity', label: BI_SCREEN_COPY.metrics.highSeverity, value: safeCount(metrics.high_severity_bugs), hint: copy.lampHint(safeCount(metrics.high_severity_bugs)), tone: 'warning' },
    { id: 'open-bugs', label: BI_SCREEN_COPY.metrics.openBugs, value: safeCount(metrics.open_bugs), hint: BI_SCREEN_COPY.metrics.openBugsHint, tone: 'warning' },
    { id: 'pending-review', label: BI_SCREEN_COPY.metrics.pendingReview, value: safeCount(metrics.pending_review_bugs), hint: BI_SCREEN_COPY.metrics.pendingReviewHint, tone: 'warning' },
  ]
  return {
    completionPct: Math.min(100, Math.max(0, Math.round(metrics.completion_rate_pct))),
    totalTasks: safeCount(metrics.total_tasks),
    doneTasks: safeCount(metrics.done_tasks),
    inFlightTasks: safeCount(metrics.in_flight_tasks),
    recentCompleted,
    delta: recent.length && previous.length
      ? {
          direction: recentCompleted > previousCompleted
            ? 'up'
            : recentCompleted < previousCompleted ? 'down' : 'flat',
          value: Math.abs(recentCompleted - previousCompleted),
        }
      : null,
    lamps,
  }
}
