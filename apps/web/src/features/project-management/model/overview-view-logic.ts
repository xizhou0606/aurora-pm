import { shouldFallbackModuleApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import type { ProjectOverviewFacts } from './overview-query'
import {
  clampProgress,
  formatPMDateRange,
  healthLabel,
  projectItemTypeLabel,
  riskLabel,
  statusLabel,
} from './selectors'
import type {
  ProjectOverviewAggregateModel,
  ProjectOverviewFactsState,
} from './overview-view-types'
import {
  attentionItems,
  deliverySummary,
  isTaskCompleted,
  riskRequirements,
  scopedNodes,
  workBreakdown,
} from './overview-view-helpers'

type RetryIntent = () => void

export function resolveProjectOverviewFactsState(input: {
  status: 'pending' | 'error' | 'success'
  facts?: ProjectOverviewFacts
  error?: unknown
  fetching: boolean
  retry: RetryIntent
}): ProjectOverviewFactsState {
  if (input.status === 'pending' && !input.facts) return { status: 'loading' }
  if (input.status === 'error' && (
    !input.facts || !shouldFallbackModuleApi(input.error)
  )) {
    return { status: 'error', retry: input.retry }
  }
  if (!input.facts) return { status: 'error', retry: input.retry }
  const settled = {
    facts: input.facts,
    refreshing: input.fetching,
    ...(input.status === 'error'
      ? { staleError: { retry: input.retry } }
      : {}),
  }
  return input.facts.requirements.length === 0 && input.facts.tasks.length === 0
    ? { status: 'empty', ...settled }
    : { status: 'ready', ...settled }
}

export function projectOverviewAggregateModel(input: {
  project: ScrumProjectNew
  unit?: ScrumVersionTaskNew
  units: readonly ScrumVersionTaskNew[]
  facts: ProjectOverviewFacts
  now?: Date
}): ProjectOverviewAggregateModel {
  const overview = copy.projectManagement.overview
  const scope = input.unit ?? input.project
  const now = input.now ?? new Date()
  const nodes = scopedNodes(input.units)
  const totalTasks = Math.max(0, Number(scope.total_task_count ?? input.facts.tasks.length))
  const completedTasks = Math.min(
    totalTasks,
    Math.max(0, Number(
      scope.completed_task_count ?? input.facts.tasks.filter((task) => isTaskCompleted(task.status)).length,
    )),
  )
  return {
    metrics: [
      { id: 'status', label: overview.metricStatus, value: statusLabel(scope.status) },
      {
        id: 'progress',
        label: overview.metricProgress,
        value: `${clampProgress(scope.progress)}%`,
        description: overview.metricTaskProgress(completedTasks, totalTasks),
      },
      {
        id: 'health',
        label: overview.metricHealth,
        value: healthLabel(scope.health_status),
        description: scope.health_reason || overview.healthReasonUnset,
      },
      { id: 'risk', label: overview.metricRisk, value: riskLabel(scope.risk_level) },
      {
        id: 'dates',
        label: overview.metricDates,
        value: formatPMDateRange(scope.starts_at, scope.due_at),
      },
    ],
    attention: attentionItems({
      scope,
      scopeLabel: input.unit ? projectItemTypeLabel(input.unit.type) : copy.projectManagement.projects,
      requirements: input.facts.requirements,
      tasks: input.facts.tasks,
      nodes,
      now,
    }),
    riskRequirements: riskRequirements({
      requirements: input.facts.requirements,
      tasks: input.facts.tasks,
      bugs: input.facts.bugs ?? [],
      units: input.units,
      now,
    }),
    evidence: {
      requirements: input.facts.requirements.length,
      tasks: input.facts.tasks.length,
      bugs: (input.facts.bugs ?? []).reduce((total, bug) => total + bug.count, 0),
    },
    delivery: deliverySummary({ scope, nodes, now }),
    work: workBreakdown(input.facts.requirements, input.facts.tasks, input.facts.bugs ?? []),
  }
}
