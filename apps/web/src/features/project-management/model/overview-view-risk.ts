import { copy } from '@/shared/copy'
import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
  ScrumVersionTaskNew,
} from '../api'
import { clampProgress } from './selectors'
import type {
  ProjectRiskRequirement,
  ProjectRiskRequirementDimension,
} from './overview-view-types'
import { isDone, isPast, isTaskCompleted } from './overview-view-helpers'

export function riskRequirements(input: {
  requirements: readonly ScrumProductRequirementNew[]
  tasks: readonly ProjectWorkTask[]
  bugs: readonly { requirementId: number; count: number }[]
  units: readonly ScrumVersionTaskNew[]
  now: Date
}): ProjectRiskRequirement[] {
  const risk = copy.projectManagement.overview.riskRequirements
  const unitById = new Map(input.units.map((unit) => [unit.id, unit]))
  const tasksByRequirement = new Map<number, ProjectWorkTask[]>()
  for (const task of input.tasks) {
    const requirementId = Number(task.requirement_id || 0)
    if (!requirementId) continue
    const rows = tasksByRequirement.get(requirementId) ?? []
    rows.push(task)
    tasksByRequirement.set(requirementId, rows)
  }
  const openBugsByRequirement = new Map<number, number>()
  for (const bug of input.bugs) {
    openBugsByRequirement.set(
      bug.requirementId,
      (openBugsByRequirement.get(bug.requirementId) ?? 0) + bug.count,
    )
  }

  return input.requirements.flatMap((requirement) => {
    const unit = unitById.get(requirement.version_task_new_id)
    const tasks = tasksByRequirement.get(requirement.id) ?? []
    const completedFromTasks = tasks.filter((task) => isTaskCompleted(task.status)).length
    const totalTasks = Math.max(
      0,
      Number(requirement.total_task_count ?? requirement.task_count ?? 0),
      tasks.length,
    )
    const completedTasks = Math.min(
      totalTasks,
      Math.max(0, Number(requirement.completed_task_count ?? 0), completedFromTasks),
    )
    const overdueTasks = tasks.filter((task) => (
      !isDone(task.status) && isPast(task.due_at, input.now)
    )).length
    const unassignedTasks = tasks.filter((task) => (
      !isDone(task.status) && !Number(task.assignee_id || 0)
    )).length
    const openBugs = openBugsByRequirement.get(requirement.id) ?? 0
    const dimensions: ProjectRiskRequirementDimension[] = []
    if (overdueTasks > 0) dimensions.push('task_overdue')
    if (unassignedTasks > 0) dimensions.push('task_unassigned')
    if (openBugs > 0) dimensions.push('bug_open')
    if (dimensions.length === 0) return []

    const reasons = [
      ...(overdueTasks > 0 ? [risk.overdueTasks(overdueTasks)] : []),
      ...(unassignedTasks > 0 ? [risk.unassignedTasks(unassignedTasks)] : []),
      ...(openBugs > 0 ? [risk.openBugs(openBugs)] : []),
    ]
    const progress = totalTasks > 0
      ? clampProgress(Math.round((completedTasks / totalTasks) * 100))
      : clampProgress(requirement.progress)
    return [{
      id: requirement.id,
      planUnitId: requirement.version_task_new_id,
      planUnitTitle: unit?.title || risk.unknownPlanUnit,
      planUnitType: unit?.type,
      title: requirement.title,
      priority: String(requirement.priority || 'normal'),
      severity: overdueTasks > 0 || openBugs > 0 ? 'critical' : 'high',
      requirementStatus: String(requirement.status || ''),
      completedTasks,
      totalTasks,
      progress,
      overdueTasks,
      unassignedTasks,
      openBugs,
      dimensions,
      reasons,
    } satisfies ProjectRiskRequirement]
  }).sort((left, right) => {
    const severity = Number(right.severity === 'critical') - Number(left.severity === 'critical')
    if (severity !== 0) return severity
    if (right.openBugs !== left.openBugs) return right.openBugs - left.openBugs
    if (right.overdueTasks !== left.overdueTasks) return right.overdueTasks - left.overdueTasks
    if (right.unassignedTasks !== left.unassignedTasks) return right.unassignedTasks - left.unassignedTasks
    if (left.progress !== right.progress) return left.progress - right.progress
    const priority = Number(right.priority === 'urgent') - Number(left.priority === 'urgent')
    if (priority !== 0) return priority
    return left.id - right.id
  })
}
