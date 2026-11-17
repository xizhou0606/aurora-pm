import { isSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import { projectDeliveryNodeLabelIdentity } from '@/features/project-management/utils/projectDeliveryNodes'
import type { ScrumProjectDeliveryNodeLabel } from '../api'
import type { PlanUnitDraft } from '../model'

export const deliveryCopy = copy.projectManagement.delivery

export function safeErrorCode(error: unknown, fallback: string): string {
  return isSafeApiError(error) ? error.code : fallback
}

export function duplicateLabel(
  labels: readonly ScrumProjectDeliveryNodeLabel[],
  name: string,
  excluded?: string,
): boolean {
  const normalized = name.trim().toLocaleLowerCase()
  return labels.some((label) => (
    projectDeliveryNodeLabelIdentity(label) !== excluded &&
    label.name.trim().toLocaleLowerCase() === normalized
  ))
}

export function replaceLabel<T extends PlanUnitDraft>(
  draft: T,
  identity: string,
  label: ScrumProjectDeliveryNodeLabel,
): T {
  return {
    ...draft,
    deliveryNodeLabels: draft.deliveryNodeLabels.map((item) => (
      projectDeliveryNodeLabelIdentity(item) === identity ? label : item
    )),
  }
}