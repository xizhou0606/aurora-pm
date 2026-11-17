import type { ProjectDetailTab, ProjectSearchTab } from '@/shared/router'
import type {
  PlatformUser,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import type {
  AuditDisplayRow,
  UserPreviewModel,
} from './selectors'
import type { ProjectRefreshState } from './refresh-state'

type RetryIntent = () => void

export type ProjectListItem = {
  id: number
  name: string
  description: string
  owner: UserPreviewModel
  status: string
  statusKey: string
  health: string
  healthReason?: string
  planUnitCount: number
  requirementCount: number
  progress: number
  selected: boolean
}

export type ProjectListState =
  | { status: 'loading'; summary: string }
  | { status: 'error'; summary: string; retry: RetryIntent }
  | { status: 'empty'; summary: string; reason: 'initial' | 'filtered' }
  | {
      status: 'ready'
      summary: string
      items: ProjectListItem[]
      total: number
      refreshing: boolean
    }

export type ActivityScope = 'project' | 'plan-unit'

export type ActivityRefreshState = ProjectRefreshState

export type ActivityState =
  | { status: 'loading'; scope: ActivityScope }
  | { status: 'error'; scope: ActivityScope; retry: RetryIntent }
  | {
      status: 'empty'
      scope: ActivityScope
      refresh: ActivityRefreshState
    }
  | {
      status: 'ready'
      scope: ActivityScope
      rows: AuditDisplayRow[]
      refresh: ActivityRefreshState
    }

export type ResourceHeaderModel = {
  title: string
  description: string
  meta: string[]
  owner?: {
    label: string
    preview: UserPreviewModel
  }
}

export type ProjectContentState =
  | {
      kind: 'overview'
      units: ScrumVersionTaskNew[]
      users: PlatformUser[]
    }
  | { kind: 'items'; rows: ScrumVersionTaskNew[]; users: PlatformUser[] }
  | { kind: 'activity'; projectId: number }

export type PlanUnitTab = 'overview' | 'requirements' | 'tasks' | 'delivery' | 'activity'

export type PlanUnitContentSearch = {
  requirement: {
    q: string
    status?: any
    priority?: any
    page: number
    pageSize: number
  }
  task: {
    requirementId?: number
    taskId?: number
    page: number
    pageSize: number
  }
}

export type PlanUnitContentState =
  | { kind: 'overview' }
  | { kind: 'delivery' }
  | { kind: 'requirements' }
  | { kind: 'tasks' }
  | { kind: 'activity'; projectId: number; planUnitId: number }

export type ProjectReadyModel = {
  kind: 'project'
  projectId: number
  project: ScrumProjectNew
  planUnits: ScrumVersionTaskNew[]
  header: ResourceHeaderModel
  fromItemId?: number
  content: ProjectContentState
}

export type PlanUnitReadyModel = {
  kind: 'plan-unit'
  projectId: number
  planUnitId: number
  route: Extract<ProjectWorkspaceRoute, { kind: 'plan-unit' }>
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  planUnits: ScrumVersionTaskNew[]
  users: PlatformUser[]
  header: ResourceHeaderModel
  content: PlanUnitContentState
}

export type ProjectScreenState =
  | { status: 'select' }
  | { status: 'loading' }
  | { status: 'error'; retry: RetryIntent }
  | { status: 'project-not-found' }
  | { status: 'item-not-found'; projectId: number; planUnitId: number }
  | {
      status: 'ready'
      resource: ProjectReadyModel | PlanUnitReadyModel
      refresh: ProjectRefreshState
    }

export type ProjectWorkspaceRoute =
  | { kind: 'list'; projectId?: number }
  | {
      kind: 'project'
      projectId: number
      tab: ProjectDetailTab
      fromItemId?: number
    }
  | {
      kind: 'plan-unit'
      projectId: number
      planUnitId: number
      tab: PlanUnitTab
      search?: PlanUnitContentSearch
    }
