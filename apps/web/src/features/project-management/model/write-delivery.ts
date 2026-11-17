import type {
  PatchScrumProjectDeliveryNodePayload,
  ScrumProjectDeliveryNode,
  ScrumProjectDeliveryNodeLabel,
  ScrumVersionTaskNew,
} from '../api'
import {
  availableProjectDeliveryNodeLabels,
  nextProjectDeliveryNodeLabelClientKey,
  normalizeProjectDeliveryNodeLabels,
  normalizeProjectDeliveryNodes,
  projectDeliveryNodeIdentity,
  projectDeliveryNodeLabelIdentity,
} from '@/features/project-management/utils/projectDeliveryNodes'
import { dateTimePayload } from './form-utils'

export type PlanUnitDeliveryDraft = {
  deliveryNodeLabels: ScrumProjectDeliveryNodeLabel[]
  deliveryNodes: ScrumProjectDeliveryNode[]
}

export function planUnitDeliveryFromCreate(): PlanUnitDeliveryDraft {
  return {
    deliveryNodeLabels: [],
    deliveryNodes: [],
  }
}

export function planUnitDeliveryFromUnit(
  unit: Pick<ScrumVersionTaskNew, 'delivery_nodes'>,
  labels: readonly ScrumProjectDeliveryNodeLabel[] = [],
): PlanUnitDeliveryDraft {
  const deliveryNodeLabels = normalizeProjectDeliveryNodeLabels(labels)
  return {
    deliveryNodeLabels,
    deliveryNodes: normalizeProjectDeliveryNodes(unit.delivery_nodes, deliveryNodeLabels),
  }
}

export function replacePlanUnitDeliveryLabels<T extends PlanUnitDeliveryDraft>(
  draft: T,
  labels: readonly ScrumProjectDeliveryNodeLabel[],
  preserveRemoved: boolean,
): T {
  const previous = new Map(
    draft.deliveryNodeLabels.map((label) => [projectDeliveryNodeLabelIdentity(label), label]),
  )
  const deliveryNodeLabels = normalizeProjectDeliveryNodeLabels(labels).filter((label) => !label.deleted)
  const active = new Set(deliveryNodeLabels.map(projectDeliveryNodeLabelIdentity))
  const deliveryNodes = normalizeProjectDeliveryNodes(
    draft.deliveryNodes.flatMap((node) => {
      const identity = projectDeliveryNodeIdentity(node)
      const oldLabel = previous.get(identity)
      if (!identity || !oldLabel || active.has(identity)) return [node]
      return preserveRemoved
        ? [{ ...node, label_name: node.label_name || oldLabel.name, label_deleted: true }]
        : []
    }),
    deliveryNodeLabels,
  )
  return { ...draft, deliveryNodeLabels, deliveryNodes }
}

export function createPlanUnitDeliveryLabel<T extends PlanUnitDeliveryDraft>(
  draft: T,
  name: string,
): T {
  const labels = normalizeProjectDeliveryNodeLabels(draft.deliveryNodeLabels)
  const next: ScrumProjectDeliveryNodeLabel = {
    client_key: nextProjectDeliveryNodeLabelClientKey(labels),
    name: name.trim(),
    system_key: null,
    sort_order: Math.max(0, ...labels.map((label) => Number(label.sort_order || 0))) + 10,
    deleted: false,
  }
  return replacePlanUnitDeliveryLabels(draft, [...labels, next], false)
}

export function renamePlanUnitDeliveryLabel<T extends PlanUnitDeliveryDraft>(
  draft: T,
  identity: string,
  name: string,
): T {
  return replacePlanUnitDeliveryLabels(
    draft,
    draft.deliveryNodeLabels.map((label) => (
      projectDeliveryNodeLabelIdentity(label) === identity
        ? { ...label, name: name.trim() }
        : label
    )),
    draft.deliveryNodes.some((node) => node.label_deleted === true),
  )
}

export function removePlanUnitDeliveryLabel<T extends PlanUnitDeliveryDraft>(
  draft: T,
  identity: string,
  preserveRemoved: boolean,
): T {
  return replacePlanUnitDeliveryLabels(
    draft,
    draft.deliveryNodeLabels.filter((label) => projectDeliveryNodeLabelIdentity(label) !== identity),
    preserveRemoved,
  )
}

export function availablePlanUnitDeliveryLabels(draft: PlanUnitDeliveryDraft) {
  return availableProjectDeliveryNodeLabels(draft.deliveryNodeLabels, draft.deliveryNodes)
}

export function addPlanUnitDeliveryNode<T extends PlanUnitDeliveryDraft>(
  draft: T,
  label: ScrumProjectDeliveryNodeLabel,
): T {
  const node: ScrumProjectDeliveryNode = Number(label.id || 0) > 0
    ? { label_id: Number(label.id) }
    : { label_client_key: String(label.client_key || '') }
  return {
    ...draft,
    deliveryNodes: normalizeProjectDeliveryNodes(
      [...draft.deliveryNodes, node],
      draft.deliveryNodeLabels,
    ),
  }
}

export function removePlanUnitDeliveryNode<T extends PlanUnitDeliveryDraft>(
  draft: T,
  identity: string,
): T {
  return {
    ...draft,
    deliveryNodes: draft.deliveryNodes.filter((node) => projectDeliveryNodeIdentity(node) !== identity),
  }
}

export function updatePlanUnitDeliveryNode<T extends PlanUnitDeliveryDraft>(
  draft: T,
  identity: string,
  patch: Partial<Pick<ScrumProjectDeliveryNode, 'due_at' | 'actual_completed_at' | 'owner_user_id'>>,
): T {
  return {
    ...draft,
    deliveryNodes: draft.deliveryNodes.map((node) => (
      projectDeliveryNodeIdentity(node) === identity ? { ...node, ...patch } : node
    )),
  }
}

export function patchPlanUnitDeliveryPayload(
  draft: PlanUnitDeliveryDraft,
): PatchScrumProjectDeliveryNodePayload[] {
  return normalizeProjectDeliveryNodes(draft.deliveryNodes, draft.deliveryNodeLabels).flatMap((node) => {
    const labelId = Number(node.label_id || 0)
    return labelId > 0 ? [{
      label_id: labelId,
      due_at: dateTimePayload(node.due_at || ''),
      actual_completed_at: dateTimePayload(node.actual_completed_at || ''),
      owner_user_id: node.owner_user_id || null,
    }] : []
  })
}
