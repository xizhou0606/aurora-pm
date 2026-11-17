import { isSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import {
  normalizeProjectDeliveryNodeLabels,
} from '@/features/project-management/utils/projectDeliveryNodes'
import type {
  ScrumProjectDeliveryNodeLabel,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'

export const deliveryCopy = copy.projectManagement.delivery

export type DeliveryFeedbackState =
  | { status: 'idle' }
  | { status: 'success'; message: string }
  | { status: 'error'; message: string }

export type DeliveryLabelCatalogState =
  | { status: 'loading'; writable: false }
  | { status: 'error'; retry: () => void; writable: false }
  | {
      status: 'empty'
      refreshing: boolean
      staleError?: { retry: () => void }
      writable: boolean
    }
  | {
      status: 'ready'
      items: readonly ScrumProjectDeliveryNodeLabel[]
      refreshing: boolean
      staleError?: { retry: () => void }
      writable: boolean
    }

export type DeliveryEditorTarget = {
  projectId: number
  planUnitId: number
  title: string
}

export type DeliveryQuickEditTarget = DeliveryEditorTarget & {
  nodeKey: string
  label: string
}

export function errorCode(error: unknown, fallback: string): string {
  return isSafeApiError(error) ? error.code : fallback
}

export function activeLabels(
  labels: readonly ScrumProjectDeliveryNodeLabel[],
): ScrumProjectDeliveryNodeLabel[] {
  return normalizeProjectDeliveryNodeLabels(labels).filter((label) => !label.deleted)
}

export function hasDuplicateLabel(
  labels: readonly ScrumProjectDeliveryNodeLabel[],
  name: string,
  excludedId?: number,
): boolean {
  const normalized = name.trim().toLocaleLowerCase()
  return activeLabels(labels).some((label) => (
    label.id !== excludedId && label.name.trim().toLocaleLowerCase() === normalized
  ))
}

export function historicalLabels(unit?: ScrumVersionTaskNew): ScrumProjectDeliveryNodeLabel[] {
  const history = new Map<string, ScrumProjectDeliveryNodeLabel>()
  for (const node of unit?.delivery_nodes ?? []) {
    if (node.label_deleted !== true) continue
    const id = Number(node.label_id || 0)
    const clientKey = String(node.label_client_key || '').trim()
    const name = String(node.label_name || '').trim()
    const key = id > 0 ? `id:${id}` : clientKey ? `client:${clientKey}` : ''
    if (!key || !name || history.has(key)) continue
    history.set(key, {
      ...(id > 0 ? { id } : {}),
      ...(id <= 0 && clientKey ? { client_key: clientKey } : {}),
      name,
      system_key: node.type || null,
      sort_order: Number.MAX_SAFE_INTEGER,
      deleted: true,
    })
  }
  return [...history.values()]
}

export function canEditDelivery(
  project: Pick<ScrumProjectNew, 'id'> | undefined,
  unit: Pick<ScrumVersionTaskNew, 'project_new_id'> | undefined,
): boolean {
  return Boolean(project && unit && unit.project_new_id === project.id)
}