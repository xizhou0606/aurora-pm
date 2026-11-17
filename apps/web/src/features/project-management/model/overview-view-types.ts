import type { ProjectOverviewFacts } from './overview-query'

type RetryIntent = () => void

export type ProjectOverviewFactsState =
  | { status: 'loading' }
  | { status: 'error'; retry: RetryIntent }
  | {
      status: 'empty' | 'ready'
      facts: ProjectOverviewFacts
      refreshing: boolean
      staleError?: { retry: RetryIntent }
    }

export type ProjectAttentionTarget =
  | 'plan'
  | 'requirements'
  | 'tasks'
  | 'delivery'
  | 'overview'

export type ProjectAttentionItem = {
  key: string
  title: string
  detail: string
  target: ProjectAttentionTarget
}

export type ProjectOverviewMetric = {
  id: string
  label: string
  value: string
  description?: string
}

export type ProjectDeliverySummary = {
  verdict: string
  currentNode: string
  nextDue: string
  completedNodes: number
  totalNodes: number
  delayedNodes: number
}

export type ProjectWorkBreakdown = {
  requirements: {
    total: number
    done: number
    active: number
    backlog: number
    highPriority: number
  }
  tasks: {
    total: number
    done: number
    active: number
    todo: number
    cancelled: number
    unassigned: number
  }
  bugs: {
    open: number
    critical: number
    normal: number
    minor: number
  }
}

export type ProjectRiskRequirementSeverity = 'critical' | 'high'

export type ProjectRiskRequirementDimension =
  | 'task_overdue'
  | 'task_unassigned'
  | 'bug_open'

export type ProjectRiskRequirement = {
  id: number
  planUnitId: number
  planUnitTitle: string
  planUnitType?: string
  title: string
  priority: string
  severity: ProjectRiskRequirementSeverity
  requirementStatus: string
  completedTasks: number
  totalTasks: number
  progress: number
  overdueTasks: number
  unassignedTasks: number
  openBugs: number
  dimensions: ProjectRiskRequirementDimension[]
  reasons: string[]
}

export type ProjectOverviewAggregateModel = {
  metrics: ProjectOverviewMetric[]
  attention: ProjectAttentionItem[]
  riskRequirements: ProjectRiskRequirement[]
  evidence: {
    requirements: number
    tasks: number
    bugs: number
  }
  delivery: ProjectDeliverySummary
  work: ProjectWorkBreakdown
}