import type { ProjectDetailTab, ProjectSearchTab } from '@/shared/router'
import { shouldFallbackModuleApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  PlatformUser,
  ScrumProjectAuditLogNew,
  ScrumProjectNew,
} from '../api'
import {
  auditDisplayRows,
  auditRowsForPlanUnit,
  clampProgress,
  formatPMDateRange,
  healthLabel,
  ownerPreview,
  projectFromSources,
  statusLabel,
} from './selectors'
import type { ProjectOverviewFacts } from './overview-query'
import type { ProjectRefreshState } from './refresh-state'
import type {
  ActivityState,
  PlanUnitTab,
  ProjectListState,
} from './view-state-types'

type ReadStatus = 'pending' | 'error' | 'success'
type RetryIntent = () => void

export function resolveProjectDetailTab(tab: ProjectSearchTab): ProjectDetailTab {
  switch (tab) {
    case 'overview':
    case 'items':
    case 'activity':
      return tab
    case 'requirements':
    case 'tasks':
    case 'delivery':
      return 'overview'
  }
}

export function resolvePlanUnitTab(tab: ProjectSearchTab): PlanUnitTab {
  switch (tab) {
    case 'activity':
      return 'activity'
    case 'requirements':
      return 'requirements'
    case 'tasks':
      return 'tasks'
    case 'delivery':
      return 'delivery'
    case 'overview':
    case 'items':
      return 'overview'
  }
}

export function resolveProjectListState(input: {
  status: ReadStatus
  projects: ScrumProjectNew[]
  total: number
  fetching: boolean
  search: string
  selectedProjectId?: number
  users: PlatformUser[]
  retry: RetryIntent
}): ProjectListState {
  const projectCopy = copy.projectManagement.projectList
  if (input.status === 'pending') {
    return { status: 'loading', summary: projectCopy.loading }
  }
  if (input.status === 'error') {
    return {
      status: 'error',
      summary: projectCopy.errorTitle,
      retry: input.retry,
    }
  }
  if (input.projects.length === 0) {
    return {
      status: 'empty',
      summary: projectCopy.count(0, input.total),
      reason: input.search.trim() ? 'filtered' : 'initial',
    }
  }

  return {
    status: 'ready',
    summary: input.fetching
      ? projectCopy.refreshing
      : projectCopy.count(input.projects.length, input.total),
    items: input.projects.map((project) => ({
      id: project.id,
      name: project.name,
      description: project.description || project.code || projectCopy.fallbackCode(project.id),
      owner: ownerPreview(project.owner_user_id, input.users, project.owner_name),
      status: statusLabel(project.status),
      statusKey: String(project.status || ''),
      health: healthLabel(project.health_status),
      healthReason: project.health_reason,
      planUnitCount: Math.max(0, Number(project.version_task_count || 0)),
      requirementCount: Math.max(0, Number(project.requirement_count || 0)),
      progress: clampProgress(project.progress),
      selected: project.id === input.selectedProjectId,
    })),
    total: input.total,
    refreshing: input.fetching,
  }
}

type ActivitySource<T> = {
  status: ReadStatus
  data?: T
  error?: unknown
  fetching: boolean
}

type ActivityReadInput = {
  audit: ActivitySource<ScrumProjectAuditLogNew[]>
  users: PlatformUser[]
  retry: RetryIntent
} & (
  | { scope: 'project' }
  | {
      scope: 'plan-unit'
      planUnitId: number
      facts: ActivitySource<ProjectOverviewFacts>
    }
)

type ActivitySourceStatus = 'loading' | 'error' | 'ready' | 'stale_error'

function resolveActivitySource<T>(source: ActivitySource<T>): ActivitySourceStatus {
  if (source.status === 'error') {
    return source.data && shouldFallbackModuleApi(source.error)
      ? 'stale_error'
      : 'error'
  }
  if (!source.data) return source.status === 'pending' ? 'loading' : 'error'
  return 'ready'
}

export function resolveActivityState(input: ActivityReadInput): ActivityState {
  const auditStatus = resolveActivitySource(input.audit)
  const factsStatus = input.scope === 'plan-unit'
    ? resolveActivitySource(input.facts)
    : 'ready'
  if (auditStatus === 'error' || factsStatus === 'error') {
    return { status: 'error', scope: input.scope, retry: input.retry }
  }
  if (auditStatus === 'loading' || factsStatus === 'loading') {
    return { status: 'loading', scope: input.scope }
  }
  const auditRows = input.audit.data ?? []
  const scopedRows = input.scope === 'plan-unit'
    ? auditRowsForPlanUnit(auditRows, input.planUnitId, input.facts.data ?? {
        requirements: [],
        tasks: [],
      })
    : auditRows
  const rows = auditDisplayRows(scopedRows, input.users)
  const refresh: ProjectRefreshState = (
    auditStatus === 'stale_error' || factsStatus === 'stale_error'
  )
    ? { status: 'stale_error', retry: input.retry }
    : input.audit.fetching || (
        input.scope === 'plan-unit' && input.facts.fetching
      )
      ? { status: 'refreshing' }
      : { status: 'idle' }
  if (rows.length === 0) {
    return { status: 'empty', scope: input.scope, refresh }
  }
  return {
    status: 'ready',
    scope: input.scope,
    rows,
    refresh,
  }
}

export { resolveProjectScreenState } from './view-state-screen'