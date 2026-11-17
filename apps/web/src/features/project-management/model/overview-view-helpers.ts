import { copy } from '@/shared/copy'
import { projectDeliveryNodeLabel } from '@/features/project-management/utils/projectDeliveryNodes'
import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
  ScrumProjectDeliveryNode,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { deliveryNodesForUnit } from './delivery-view'
import {
  formatPMDate,
  healthLabel,
} from './selectors'
import type {
  ProjectAttentionItem,
  ProjectDeliverySummary,
  ProjectWorkBreakdown,
} from './overview-view-types'

const doneStatuses = new Set([
  'released', 'archived', 'done', 'completed', 'closed',
  'cancelled', 'canceled', 'duplicate',
])
const activeStatuses = new Set([
  'grooming', 'development', 'in_dev', 'pending_test',
  'in_progress', 'doing', 'blocked', 'review', 'acceptance',
])
const completedTaskStatuses = new Set([
  'released', 'archived', 'closed', 'done', 'completed',
])
const cancelledTaskStatuses = new Set(['cancelled', 'canceled', 'duplicate'])

export function isDone(status?: string): boolean {
  return doneStatuses.has(String(status || '').toLowerCase())
}

export function isActive(status?: string): boolean {
  return activeStatuses.has(String(status || '').toLowerCase())
}

export function isTaskCompleted(status?: string): boolean {
  return completedTaskStatuses.has(String(status || '').toLowerCase())
}

export function isTaskCancelled(status?: string): boolean {
  return cancelledTaskStatuses.has(String(status || '').toLowerCase())
}

export function isPast(value: string | null | undefined, now: Date): boolean {
  if (!value) return false
  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) && timestamp < now.getTime()
}

function listPreview(values: readonly string[], kind: string): string {
  const attention = copy.projectManagement.overview.attention
  const unique = [...new Set(values.map((value) => value.trim()).filter(Boolean))]
  if (unique.length === 0) return attention.listFallback(kind)
  if (unique.length <= 2) return unique.join('、')
  return attention.listMany(unique.slice(0, 2).join('、'), unique.length, kind)
}

export function scopedNodes(units: readonly ScrumVersionTaskNew[]): ScrumProjectDeliveryNode[] {
  return units.flatMap((unit) => deliveryNodesForUnit(unit))
    .filter((node) => node.label_deleted !== true)
}

export function attentionItems(input: {
  scope: ScrumProjectNew | ScrumVersionTaskNew
  scopeLabel: string
  requirements: readonly ScrumProductRequirementNew[]
  tasks: readonly ProjectWorkTask[]
  nodes: readonly ScrumProjectDeliveryNode[]
  now: Date
}): ProjectAttentionItem[] {
  const attention = copy.projectManagement.overview.attention
  const items: ProjectAttentionItem[] = []
  const actualCompleted = 'actual_completed_at' in input.scope
    ? input.scope.actual_completed_at
    : undefined
  if (!actualCompleted && !isDone(input.scope.status) && isPast(input.scope.due_at, input.now)) {
    items.push({
      key: 'scope-overdue',
      title: attention.scopeOverdueTitle(input.scopeLabel),
      detail: attention.scopeOverdueDetail(formatPMDate(input.scope.due_at)),
      target: 'plan',
    })
  }
  const overdueTasks = input.tasks.filter((task) => (
    !isDone(task.status) && isPast(task.due_at, input.now)
  ))
  const overdueRequirementIds = new Set(
    overdueTasks.map((task) => Number(task.requirement_id || 0)).filter(Boolean),
  )
  const overdueRequirements = input.requirements.filter((requirement) => (
    overdueRequirementIds.has(requirement.id) && !isDone(requirement.status)
  ))
  if (overdueRequirements.length > 0) {
    items.push({
      key: 'requirements-overdue',
      title: attention.requirementsOverdue,
      detail: listPreview(overdueRequirements.map((item) => item.title), '需求'),
      target: 'requirements',
    })
  }
  if (overdueTasks.length > 0) {
    items.push({
      key: 'tasks-overdue',
      title: attention.tasksOverdue,
      detail: listPreview(overdueTasks.map((item) => item.title), '任务'),
      target: 'tasks',
    })
  }
  const delayedNodes = input.nodes.filter((node) => (
    !node.actual_completed_at && isPast(node.due_at, input.now)
  ))
  if (delayedNodes.length > 0) {
    const first = delayedNodes[0]
    const label = projectDeliveryNodeLabel(first)
    items.push({
      key: 'delivery-nodes-delayed',
      title: attention.deliveryDelayed,
      detail: delayedNodes.length === 1
        ? attention.deliverySingle(label, formatPMDate(first.due_at))
        : attention.deliveryMany(label, delayedNodes.length),
      target: 'delivery',
    })
  }
  const healthStatus = String(input.scope.health_status || '')
  if (healthStatus === 'at_risk' || healthStatus === 'off_track') {
    items.push({
      key: 'health',
      title: healthStatus === 'off_track'
        ? attention.healthOffTrack
        : attention.healthAtRisk,
      detail: input.scope.health_reason || healthLabel(healthStatus),
      target: 'overview',
    })
  }
  const unassignedTasks = input.tasks.filter((task) => (
    !isDone(task.status) && !Number(task.assignee_id || 0)
  ))
  if (unassignedTasks.length > 0) {
    items.push({
      key: 'tasks-unassigned',
      title: attention.tasksUnassigned,
      detail: attention.tasksUnassignedDetail(unassignedTasks.length),
      target: 'tasks',
    })
  }
  const highPriority = input.requirements.filter((requirement) => (
    ['high', 'urgent'].includes(String(requirement.priority || '')) &&
    !isDone(requirement.status)
  ))
  if (highPriority.length > 0) {
    items.push({
      key: 'requirements-high-priority',
      title: attention.requirementsHighPriority,
      detail: listPreview(highPriority.map((item) => item.title), '需求'),
      target: 'requirements',
    })
  }
  return items.slice(0, 8)
}

export function deliverySummary(input: {
  scope: ScrumProjectNew | ScrumVersionTaskNew
  nodes: readonly ScrumProjectDeliveryNode[]
  now: Date
}): ProjectDeliverySummary {
  const summary = copy.projectManagement.overview.deliverySummary
  const completed = input.nodes.filter((node) => Boolean(node.actual_completed_at))
  const open = input.nodes.filter((node) => !node.actual_completed_at)
  const current = open[0] ?? completed[completed.length - 1]
  const nextDue = open.find((node) => Boolean(node.due_at))
  const delayed = open.filter((node) => isPast(node.due_at, input.now)).length
  const hasHealthRisk = ['at_risk', 'off_track'].includes(
    String(input.scope.health_status || ''),
  )
  const hasRisk = ['medium', 'high', 'urgent'].includes(
    String(input.scope.risk_level || ''),
  )
  const verdict = input.nodes.length > 0 && completed.length === input.nodes.length
    ? summary.completed
    : delayed > 0 || hasHealthRisk || hasRisk
      ? summary.delayed
      : input.nodes.length > 0
        ? summary.onTrack
        : summary.unconfigured
  return {
    verdict,
    currentNode: current ? projectDeliveryNodeLabel(current) : summary.noNode,
    nextDue: nextDue ? formatPMDate(nextDue.due_at) : copy.projectManagement.common.unknownValue,
    completedNodes: completed.length,
    totalNodes: input.nodes.length,
    delayedNodes: delayed,
  }
}

export function workBreakdown(
  requirements: readonly ScrumProductRequirementNew[],
  tasks: readonly ProjectWorkTask[],
  bugs: readonly { requirementId: number; severity?: 'critical' | 'normal' | 'minor'; count: number }[],
): ProjectWorkBreakdown {
  const requirementDone = requirements.filter((item) => isDone(item.status)).length
  const requirementActive = requirements.filter((item) => isActive(item.status)).length
  const taskDone = tasks.filter((item) => isTaskCompleted(item.status)).length
  const taskActive = tasks.filter((item) => isActive(item.status)).length
  const taskCancelled = tasks.filter((item) => isTaskCancelled(item.status)).length
  return {
    requirements: {
      total: requirements.length,
      done: requirementDone,
      active: requirementActive,
      backlog: Math.max(0, requirements.length - requirementDone - requirementActive),
      highPriority: requirements.filter((item) => (
        ['high', 'urgent'].includes(String(item.priority || '')) && !isDone(item.status)
      )).length,
    },
    tasks: {
      total: tasks.length,
      done: taskDone,
      active: taskActive,
      todo: Math.max(0, tasks.length - taskDone - taskActive - taskCancelled),
      cancelled: taskCancelled,
      unassigned: tasks.filter((item) => (
        !isDone(item.status) && !Number(item.assignee_id || 0)
      )).length,
    },
    bugs: {
      open: bugs.reduce((total, bug) => total + bug.count, 0),
      critical: bugs.filter((bug) => bug.severity === 'critical').reduce((total, bug) => total + bug.count, 0),
      normal: bugs.filter((bug) => (bug.severity || 'normal') === 'normal').reduce((total, bug) => total + bug.count, 0),
      minor: bugs.filter((bug) => bug.severity === 'minor').reduce((total, bug) => total + bug.count, 0),
    },
  }
}

export { riskRequirements } from './overview-view-risk'
