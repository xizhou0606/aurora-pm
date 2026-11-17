import { copy } from '@/shared/copy'
import type {
  PMContentPriority,
  PMRequirementPriority,
  ProjectWorkTaskStatus,
  RequirementStatus,
} from '../api'
import {
  CONTENT_PRIORITIES,
  REQUIREMENT_PRIORITIES,
  REQUIREMENT_STATUSES,
  TASK_STATUSES,
} from './content-forms'
import type { WriteOption } from './write-options'

const contentCopy = copy.projectManagement.content

const priorityLabels = {
  low: contentCopy.priority.low,
  normal: contentCopy.priority.normal,
  high: contentCopy.priority.high,
  urgent: contentCopy.priority.urgent,
} satisfies Record<PMContentPriority, string>

const requirementPriorityLabels = {
  p0: contentCopy.priority.p0,
  p1: contentCopy.priority.p1,
  p2: contentCopy.priority.p2,
  p3: contentCopy.priority.p3,
} satisfies Record<PMRequirementPriority, string>

const requirementStatusLabels = {
  backlog: contentCopy.requirement.status.backlog,
  grooming: contentCopy.requirement.status.grooming,
  development: contentCopy.requirement.status.development,
  acceptance: contentCopy.requirement.status.acceptance,
  done: contentCopy.requirement.status.done,
} satisfies Record<RequirementStatus, string>

const taskStatusLabels = {
  draft: contentCopy.task.status.draft,
  in_dev: contentCopy.task.status.in_dev,
  pending_test: contentCopy.task.status.pending_test,
  completed: contentCopy.task.status.completed,
  cancelled: contentCopy.task.status.cancelled,
} satisfies Record<ProjectWorkTaskStatus, string>

function options<TValue extends string>(
  values: readonly TValue[],
  labels: Record<TValue, string>,
): WriteOption<TValue>[] {
  return values.map((value) => ({ value, label: labels[value] }))
}

export const CONTENT_PRIORITY_OPTIONS = options(CONTENT_PRIORITIES, priorityLabels)
export const REQUIREMENT_PRIORITY_OPTIONS = options(REQUIREMENT_PRIORITIES, requirementPriorityLabels)
export const REQUIREMENT_STATUS_OPTIONS = options(REQUIREMENT_STATUSES, requirementStatusLabels)
export const TASK_STATUS_OPTIONS = options(TASK_STATUSES, taskStatusLabels)

export function contentPriorityLabel(value: unknown): string {
  return priorityLabels[value as PMContentPriority] ?? contentCopy.common.unknown
}

export function requirementPriorityLabel(value: unknown): string {
  return requirementPriorityLabels[value as PMRequirementPriority] ?? contentCopy.common.unknown
}

export function requirementStatusLabel(value: unknown): string {
  return requirementStatusLabels[value as RequirementStatus] ?? contentCopy.common.unknown
}

export function taskStatusLabel(value: unknown): string {
  return taskStatusLabels[value as ProjectWorkTaskStatus] ?? contentCopy.common.unknown
}
