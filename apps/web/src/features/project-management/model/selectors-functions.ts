import { copy } from '@/shared/copy'
import {
  createDefaultProjectDeliveryNodeLabels,
  normalizeProjectDeliveryNodes,
} from '@/features/project-management/utils/projectDeliveryNodes'
import type {
  ProjectWorkTask,
  ScrumBoardNew,
  ScrumProjectAuditLogNew,
  ScrumProjectNew,
  ScrumProductRequirementNew,
  ScrumVersionTaskNew,
  PlatformUser,
} from '../api'
import type {
  AuditDisplayRow,
  PlanUnitAggregateSchedule,
  PlanUnitFilters,
  PlanUnitOverviewModel,
  ProjectOverviewModel,
} from './selectors-types'
import {
  clampProgress,
  formatPMDateRange,
  healthLabel,
  ownerLabel,
  projectItemTypeLabel,
  statusLabel,
} from './selectors-labels'

export function projectFromSources(
  projectId: number,
  board?: ScrumBoardNew,
  projects: ScrumProjectNew[] = [],
): ScrumProjectNew | undefined {
  if (board) return board.project?.id === projectId ? board.project : undefined
  return projects.find((project) => project.id === projectId)
}

export function planUnitFromBoard(
  board: ScrumBoardNew | undefined,
  planUnitId: number,
  projectId?: number,
): ScrumVersionTaskNew | undefined {
  return board?.version_tasks.find((unit) => (
    unit.id === planUnitId &&
    (!projectId || unit.project_new_id === projectId)
  ))
}

export function filterPlanUnits(
  rows: ScrumVersionTaskNew[],
  filters: PlanUnitFilters,
  users: PlatformUser[] = [],
): ScrumVersionTaskNew[] {
  const query = filters.search.trim().toLocaleLowerCase()
  return rows.filter((row) => {
    if (filters.status !== 'all' && String(row.status || '') !== filters.status) return false
    const health = String(row.health_status || 'not_assessed')
    if (filters.health !== 'all' && health !== filters.health) return false
    if (!query) return true
    return [
      row.title,
      row.goal,
      projectItemTypeLabel(row.type),
      ownerLabel(row.owner_user_id, users, row.owner_name),
    ]
      .some((value) => String(value ?? '').toLocaleLowerCase().includes(query))
  })
}

export function uniquePlanUnitOptions(
  rows: ScrumVersionTaskNew[],
  field: 'status' | 'health_status',
): string[] {
  return [...new Set(rows.map((row) => String(row[field] || (
    field === 'health_status' ? 'not_assessed' : ''
  ))).filter(Boolean))]
}

export function projectItemDeliveryNodes(unit: ScrumVersionTaskNew) {
  const legacyNodes = [
    ...(unit.design_freeze_due_at
      ? [{ type: 'design_freeze' as const, due_at: unit.design_freeze_due_at }]
      : []),
    ...(unit.development_complete_due_at
      ? [{ type: 'development_complete' as const, due_at: unit.development_complete_due_at }]
      : []),
    ...(unit.test_acceptance_due_at
      ? [{ type: 'acceptance' as const, due_at: unit.test_acceptance_due_at }]
      : []),
  ]
  return normalizeProjectDeliveryNodes(
    unit.delivery_nodes === undefined ? legacyNodes : unit.delivery_nodes,
    createDefaultProjectDeliveryNodeLabels(),
  )
}

export function aggregatePlanUnitSchedule(
  units: readonly Pick<ScrumVersionTaskNew, 'starts_at' | 'due_at'>[],
): PlanUnitAggregateSchedule {
  const valid = (value?: string | null) => value && Number.isFinite(Date.parse(value)) ? value : undefined
  const starts = units.flatMap((unit) => {
    const value = valid(unit.starts_at)
    return value ? [value] : []
  })
  const ends = units.flatMap((unit) => {
    const value = valid(unit.due_at)
    return value ? [value] : []
  })
  const contributingUnits = units.filter((unit) => valid(unit.starts_at) || valid(unit.due_at)).length
  return {
    startsAt: starts.sort((left, right) => Date.parse(left) - Date.parse(right))[0],
    dueAt: ends.sort((left, right) => Date.parse(right) - Date.parse(left))[0],
    contributingUnits,
  }
}

export function auditRowsForPlanUnit(
  rows: ScrumProjectAuditLogNew[],
  planUnitId: number,
  facts: {
    requirements: readonly ScrumProductRequirementNew[]
    tasks: readonly ProjectWorkTask[]
  },
): ScrumProjectAuditLogNew[] {
  const requirementIds = new Set(
    facts.requirements
      .filter((requirement) => requirement.version_task_new_id === planUnitId)
      .map((requirement) => requirement.id),
  )
  return rows.filter((row) => {
    const resourceId = Number(row.resource_id || 0)
    if (!Number.isInteger(resourceId) || resourceId <= 0) return false
    switch (row.resource) {
      case 'version':
        return resourceId === planUnitId
      case 'requirement':
        return requirementIds.has(resourceId)
      case 'task': {
        const task = facts.tasks.find((candidate) => candidate.id === resourceId)
        return Boolean(task && (
          task.project_id === planUnitId || requirementIds.has(task.requirement_id)
        ))
      }
      default:
        return false
    }
  })
}

export function projectOverviewModel(
  project: ScrumProjectNew,
  users: PlatformUser[] = [],
): ProjectOverviewModel {
  const overview = copy.projectManagement.overview
  return {
    metrics: [
      { label: overview.metricPlanUnits, value: Number(project.version_task_count || 0) },
      { label: overview.metricRequirements, value: Number(project.requirement_count || 0) },
      { label: overview.metricTasks, value: Number(project.scrum_task_count || 0) },
      { label: overview.metricProgress, value: `${clampProgress(project.progress)}%` },
    ],
    details: [
      { label: overview.currentPlanUnit, value: project.current_plan_unit_title || overview.unset },
      { label: overview.dateRange, value: formatPMDateRange(project.starts_at, project.due_at) },
      { label: overview.owner, value: ownerLabel(project.owner_user_id, users, project.owner_name) },
      {
        label: copy.projectManagement.planUnitDetail.status,
        value: overview.statusAndHealth(
          statusLabel(project.status),
          healthLabel(project.health_status),
        ),
      },
      {
        label: overview.descriptionLabel,
        value: project.description || overview.unset,
        wide: true,
      },
    ],
  }
}

export function planUnitOverviewModel(
  unit: ScrumVersionTaskNew,
  users: PlatformUser[] = [],
): PlanUnitOverviewModel {
  const detail = copy.projectManagement.planUnitDetail
  return {
    details: [
      { label: detail.goal, value: unit.goal || copy.projectManagement.overview.unset },
      { label: detail.owner, value: ownerLabel(unit.owner_user_id, users, unit.owner_name) },
      { label: detail.status, value: statusLabel(unit.status) },
      { label: detail.health, value: healthLabel(unit.health_status) },
      { label: detail.progress, value: `${clampProgress(unit.progress)}%` },
      { label: detail.dateRange, value: formatPMDateRange(unit.starts_at, unit.due_at) },
      {
        label: detail.counts,
        value: detail.countsValue(
          Number(unit.requirement_count || 0),
          Number(unit.scrum_task_count || 0),
        ),
      },
    ],
  }
}

export function auditDisplayRows(
  rows: ScrumProjectAuditLogNew[],
  users: PlatformUser[] = [],
): AuditDisplayRow[] {
  const activity = copy.projectManagement.activity
  return rows.map((row) => {
    const user = row.user_id
      ? users.find((candidate) => candidate.id === row.user_id)
      : undefined
    const fallbackName = row.username || (
      row.user_id
        ? activity.user(row.user_id, user?.display_name)
        : activity.unknownUser
    )
    return {
      ...row,
      actor: {
        ...(row.user_id ? { userId: row.user_id } : {}),
        name: user?.display_name?.trim() || fallbackName,
        ...(user?.username && user.username !== fallbackName
          ? { secondary: user.username }
          : {}),
        ...(user?.avatar_url ? { avatarUrl: user.avatar_url } : {}),
      },
      actionLabel: activity.actions[
        row.action as keyof typeof activity.actions
      ] ?? activity.actions.unknown,
      resourceLabel: activity.resources[
        row.resource as keyof typeof activity.resources
      ] ?? activity.resources.unknown,
      resultLabel: Number(row.status_code || 200) < 400
        ? activity.success
        : activity.failed,
    }
  })
}