import { canRetainStaleApiData } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  PlatformUser,
  ScrumBoardNew,
  ScrumProjectNew,
} from '../api'
import {
  formatPMDateRange,
  healthLabel,
  ownerPreview,
  planUnitFromBoard,
  projectItemTypeLabel,
  projectFromSources,
  statusLabel,
} from './selectors'
import type { ProjectRefreshState } from './refresh-state'
import type {
  ProjectContentState,
  ProjectScreenState,
  ProjectWorkspaceRoute,
} from './view-state-types'

type ReadStatus = 'pending' | 'error' | 'success'
type RetryIntent = () => void

export function resolveProjectScreenState(input: {
  route: ProjectWorkspaceRoute
  status: ReadStatus
  board?: ScrumBoardNew
  error?: unknown
  fetching: boolean
  users: PlatformUser[]
  retry: RetryIntent
}): ProjectScreenState {
  const projectId = input.route.projectId
  if (!projectId) return { status: 'select' }
  if (input.status === 'pending' && !input.board) return { status: 'loading' }
  if (input.status === 'error' && input.fetching && !input.board) {
    return { status: 'loading' }
  }
  if (
    input.status === 'error' &&
    (!input.board || !canRetainStaleApiData(input.error))
  ) {
    return { status: 'error', retry: input.retry }
  }

  const refresh: ProjectRefreshState = input.status === 'error'
    ? { status: 'stale_error', retry: input.retry }
    : input.fetching
      ? { status: 'refreshing' }
      : { status: 'idle' }

  const project = projectFromSources(projectId, input.board)
  if (!project || !input.board) return { status: 'project-not-found' }

  if (input.route.kind === 'plan-unit') {
    const unit = planUnitFromBoard(
      input.board,
      input.route.planUnitId,
      input.route.projectId,
    )
    if (!unit) {
      return {
        status: 'item-not-found',
        projectId: input.route.projectId,
        planUnitId: input.route.planUnitId,
      }
    }
    return {
      status: 'ready',
      refresh,
      resource: {
        kind: 'plan-unit',
        projectId: input.route.projectId,
        planUnitId: unit.id,
        route: input.route,
        project,
        unit,
        planUnits: input.board.version_tasks,
        users: input.users,
        header: {
          title: unit.title,
          description: unit.goal || copy.projectManagement.planUnitList.goalUnset,
          meta: [
            `${copy.projectManagement.projects}：${project.name}`,
            `${copy.projectManagement.planUnitDetail.status}：${statusLabel(unit.status)}`,
            `${copy.projectManagement.planUnitDetail.dateRange}：${formatPMDateRange(unit.starts_at, unit.due_at)}`,
          ],
          owner: {
            label: copy.projectManagement.planUnitDetail.owner,
            preview: ownerPreview(unit.owner_user_id, input.users, unit.owner_name),
          },
        },
        content: (() => {
          switch (input.route.tab) {
            case 'activity':
              return {
                kind: 'activity' as const,
                projectId: input.route.projectId,
                planUnitId: unit.id,
              }
            case 'requirements':
              return { kind: 'requirements' as const }
            case 'tasks':
              return { kind: 'tasks' as const }
            case 'delivery':
              return { kind: 'delivery' as const }
            case 'overview':
              return { kind: 'overview' as const }
          }
        })(),
      },
    }
  }

  let content: ProjectContentState
  const tab = input.route.kind === 'list' ? 'items' : input.route.tab
  switch (tab) {
    case 'overview':
      content = {
        kind: 'overview',
        units: input.board.version_tasks,
        users: input.users,
      }
      break
    case 'items':
      content = {
        kind: 'items',
        rows: input.board.version_tasks,
        users: input.users,
      }
      break
    case 'activity':
      content = { kind: 'activity', projectId }
      break
  }

  return {
    status: 'ready',
    refresh,
    resource: {
      kind: 'project',
      projectId,
      project,
      planUnits: input.board.version_tasks,
      header: {
        title: project.name,
        description: project.description || project.code || copy.projectManagement.projectList.fallbackCode(project.id),
        meta: [
          `${copy.projectManagement.planUnitDetail.status}：${statusLabel(project.status)}`,
          `${copy.projectManagement.planUnitDetail.dateRange}：${formatPMDateRange(project.starts_at, project.due_at)}`,
        ],
        owner: {
          label: copy.projectManagement.overview.owner,
          preview: ownerPreview(project.owner_user_id, input.users, project.owner_name),
        },
      },
      fromItemId: input.route.kind === 'project' ? input.route.fromItemId : undefined,
      content,
    },
  }
}
