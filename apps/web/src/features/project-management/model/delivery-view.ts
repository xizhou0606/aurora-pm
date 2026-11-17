import { copy } from '@/shared/copy'
import type {
  PlatformUser,
  ScrumProjectDeliveryNode,
  ScrumProjectDeliveryNodeLabel,
  ScrumVersionTaskNew,
} from '../api'
import {
  normalizeProjectDeliveryNodes,
  projectDeliveryNodeIdentity,
  projectDeliveryNodeLabel,
} from '@/features/project-management/utils/projectDeliveryNodes'
import {
  clampProgress,
  ownerPreview,
  statusLabel,
  type UserPreviewModel,
} from './selectors'

export type DeliveryNodeState =
  | 'done'
  | 'late_done'
  | 'current'
  | 'overdue'
  | 'upcoming'
  | 'unscheduled'

export type DeliveryNodeModel = {
  key: string
  label: string
  deleted: boolean
  type?: string
  dueAt: string | null
  actualCompletedAt: string | null
  ownerUserId: number | null
  owner: UserPreviewModel
  state: DeliveryNodeState
  stateLabel: string
  scheduleOutcome: string
}

export type DeliveryLaneModel = {
  id: number
  title: string
  status: string
  progress: number
  progressLabel: string
  canEdit: boolean
  nodes: DeliveryNodeModel[]
}

export type DeliveryRoadmapState =
  | { status: 'empty' }
  | { status: 'ready'; lanes: DeliveryLaneModel[]; nodeCount: number }

function time(value?: string | null): number {
  const result = new Date(String(value || '')).getTime()
  return Number.isFinite(result) ? result : Number.POSITIVE_INFINITY
}

function calendarDay(value?: string | Date | null): number | null {
  if (!value) return null
  const text = value instanceof Date ? value.toISOString() : String(value).trim()
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(text)
  if (!match) return null
  return Math.floor(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])) / 86_400_000)
}

function isComplete(status?: string): boolean {
  return ['done', 'released', 'archived'].includes(String(status || ''))
}

function currentNodeIndex(nodes: ScrumProjectDeliveryNode[], now: Date): number {
  const nowTime = now.getTime()
  const upcoming = nodes.findIndex((node) =>
    !node.actual_completed_at && (!node.due_at || time(node.due_at) >= nowTime),
  )
  return upcoming >= 0 ? upcoming : nodes.findIndex((node) => !node.actual_completed_at)
}

export function resolveDeliveryNodeState(input: {
  unitStatus?: string
  dueAt?: string | null
  actualCompletedAt?: string | null
  index: number
  currentIndex: number
  now: Date
}): DeliveryNodeState {
  if (isComplete(input.unitStatus)) return 'done'
  if (input.actualCompletedAt) {
    return input.dueAt && time(input.actualCompletedAt) > time(input.dueAt)
      ? 'late_done'
      : 'done'
  }
  if (input.dueAt && time(input.dueAt) < input.now.getTime()) return 'overdue'
  if (!input.dueAt) return 'unscheduled'
  if (input.index === input.currentIndex) return 'current'
  return 'upcoming'
}

export function deliveryScheduleOutcome(
  dueAt: string | null | undefined,
  actualCompletedAt: string | null | undefined,
  now: Date,
): string {
  const outcome = copy.projectManagement.delivery.outcomes
  const dueDay = calendarDay(dueAt)
  const actualDay = calendarDay(actualCompletedAt)
  if (actualDay !== null) {
    if (dueDay === null) return outcome.completedUnscheduled
    const difference = actualDay - dueDay
    if (difference === 0) return outcome.onTime
    return difference < 0 ? outcome.early(Math.abs(difference)) : outcome.late(difference)
  }
  if (dueDay === null) return outcome.unscheduled
  const today = calendarDay(now)
  if (today === null) return outcome.unscheduled
  const remaining = dueDay - today
  if (remaining === 0) return outcome.dueToday
  return remaining > 0 ? outcome.remaining(remaining) : outcome.overdue(Math.abs(remaining))
}

export function deliveryNodesForUnit(
  unit: ScrumVersionTaskNew,
  labels: readonly ScrumProjectDeliveryNodeLabel[] = [],
): ScrumProjectDeliveryNode[] {
  if (unit.delivery_nodes !== undefined) {
    return normalizeProjectDeliveryNodes(unit.delivery_nodes, labels)
  }
  return normalizeProjectDeliveryNodes([
    ...(unit.design_freeze_due_at
      ? [{ type: 'design_freeze' as const, due_at: unit.design_freeze_due_at, owner_user_id: null }]
      : []),
    ...(unit.development_complete_due_at
      ? [{ type: 'development_complete' as const, due_at: unit.development_complete_due_at, owner_user_id: null }]
      : []),
    ...(unit.test_acceptance_due_at
      ? [{ type: 'acceptance' as const, due_at: unit.test_acceptance_due_at, owner_user_id: null }]
      : []),
  ], labels)
}

export function deliveryRoadmapState(input: {
  units: readonly ScrumVersionTaskNew[]
  users: readonly PlatformUser[]
  labelsByPlanUnit?: ReadonlyMap<number, readonly ScrumProjectDeliveryNodeLabel[]>
  canEdit: (unit: ScrumVersionTaskNew) => boolean
  now: Date
}): DeliveryRoadmapState {
  if (input.units.length === 0) return { status: 'empty' }
  const states = copy.projectManagement.delivery.states
  const lanes = input.units.map((unit) => {
    const labels = input.labelsByPlanUnit?.get(unit.id) ?? []
    const nodes = deliveryNodesForUnit(unit, labels)
    const currentIndex = isComplete(unit.status) ? -1 : currentNodeIndex(nodes, input.now)
    const progress = clampProgress(unit.progress)
    return {
      id: unit.id,
      title: unit.title,
      status: statusLabel(unit.status),
      progress,
      progressLabel: copy.projectManagement.delivery.progress(unit.title, progress),
      canEdit: input.canEdit(unit),
      nodes: nodes.map((node, index) => {
        const state = resolveDeliveryNodeState({
          unitStatus: unit.status,
          dueAt: node.due_at,
          actualCompletedAt: node.actual_completed_at,
          index,
          currentIndex,
          now: input.now,
        })
        return {
          key: projectDeliveryNodeIdentity(node) || `${unit.id}:${index}`,
          label: projectDeliveryNodeLabel(node, labels),
          deleted: node.label_deleted === true,
          type: node.type,
          dueAt: node.due_at || null,
          actualCompletedAt: node.actual_completed_at || null,
          ownerUserId: node.owner_user_id || null,
          owner: ownerPreview(node.owner_user_id, input.users),
          state,
          stateLabel: states[state],
          scheduleOutcome: deliveryScheduleOutcome(
            node.due_at,
            node.actual_completed_at,
            input.now,
          ),
        }
      }),
    } satisfies DeliveryLaneModel
  })
  return {
    status: 'ready',
    lanes,
    nodeCount: lanes.reduce((sum, lane) => sum + lane.nodes.length, 0),
  }
}
