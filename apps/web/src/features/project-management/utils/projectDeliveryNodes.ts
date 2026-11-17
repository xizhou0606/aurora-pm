import type {
  ScrumProjectDeliveryNode,
  ScrumProjectDeliveryNodeLabel,
  ScrumProjectDeliveryNodeType,
} from '@/features/project-management/api'
import { copy } from '@/shared/copy'

const deliveryCopy = copy.projectManagement.delivery

export const LEGACY_PROJECT_DELIVERY_NODE_OPTIONS: ReadonlyArray<{
  type: ScrumProjectDeliveryNodeType
  label: string
}> = [
  { type: 'product_review', label: deliveryCopy.systemLabels.product_review },
  { type: 'design_freeze', label: deliveryCopy.systemLabels.design_freeze },
  { type: 'development_complete', label: deliveryCopy.systemLabels.development_complete },
  { type: 'acceptance', label: deliveryCopy.systemLabels.acceptance },
]

const LEGACY_LABEL_BY_TYPE = new Map(
  LEGACY_PROJECT_DELIVERY_NODE_OPTIONS.map((item) => [item.type, item.label]),
)

export function createDefaultProjectDeliveryNodeLabels(): ScrumProjectDeliveryNodeLabel[] {
  return LEGACY_PROJECT_DELIVERY_NODE_OPTIONS.map((item, index) => ({
    client_key: item.type,
    name: item.label,
    system_key: item.type,
    sort_order: (index + 1) * 10,
    deleted: false,
  }))
}

export function projectDeliveryNodeLabelIdentity(label?: ScrumProjectDeliveryNodeLabel | null) {
  const id = Number(label?.id || 0)
  if (Number.isFinite(id) && id > 0) return `id:${id}`
  const clientKey = String(label?.client_key || '').trim()
  return clientKey ? `client:${clientKey}` : ''
}

export function projectDeliveryNodeIdentity(node?: ScrumProjectDeliveryNode | null) {
  const labelId = Number(node?.label_id || 0)
  if (Number.isFinite(labelId) && labelId > 0) return `id:${labelId}`
  const clientKey = String(node?.label_client_key || '').trim()
  return clientKey ? `client:${clientKey}` : ''
}

export function projectDeliveryNodeLabelDomKey(label?: ScrumProjectDeliveryNodeLabel | null) {
  const id = Number(label?.id || 0)
  return Number.isFinite(id) && id > 0 ? String(id) : String(label?.client_key || '').trim()
}

export function nextProjectDeliveryNodeLabelClientKey(
  labels?: ReadonlyArray<ScrumProjectDeliveryNodeLabel> | null,
) {
  let max = 0
  for (const label of labels || []) {
    const match = /^custom_(\d+)$/.exec(String(label.client_key || '').trim())
    if (match) max = Math.max(max, Number(match[1]))
  }
  return `custom_${max + 1}`
}

export function normalizeProjectDeliveryNodeLabels(
  labels?: ReadonlyArray<ScrumProjectDeliveryNodeLabel> | null,
): ScrumProjectDeliveryNodeLabel[] {
  const byIdentity = new Map<string, ScrumProjectDeliveryNodeLabel>()
  for (const item of labels || []) {
    const id = Number(item?.id || 0)
    const clientKey = String(item?.client_key || '').trim()
    const name = String(item?.name || '').trim()
    const normalizedId = Number.isFinite(id) && id > 0 ? id : undefined
    const identity = normalizedId ? `id:${normalizedId}` : clientKey ? `client:${clientKey}` : ''
    if (!identity || !name || byIdentity.has(identity)) continue
    byIdentity.set(identity, {
      ...(normalizedId ? { id: normalizedId } : {}),
      ...(clientKey ? { client_key: clientKey } : {}),
      name,
      system_key: item.system_key ? String(item.system_key) : null,
      sort_order: Number.isFinite(Number(item.sort_order)) ? Number(item.sort_order) : 0,
      deleted: Boolean(item.deleted),
    })
  }
  return [...byIdentity.values()].sort((left, right) => {
    if (left.sort_order !== right.sort_order) return left.sort_order - right.sort_order
    return projectDeliveryNodeLabelIdentity(left).localeCompare(projectDeliveryNodeLabelIdentity(right))
  })
}

export function projectDeliveryNodeLabel(
  nodeOrType?: ScrumProjectDeliveryNode | ScrumProjectDeliveryNodeType | string | null,
  labels?: ReadonlyArray<ScrumProjectDeliveryNodeLabel> | null,
) {
  if (typeof nodeOrType === 'string') {
    return LEGACY_LABEL_BY_TYPE.get(nodeOrType as ScrumProjectDeliveryNodeType) || nodeOrType
  }
  if (!nodeOrType) return deliveryCopy.unnamedNode
  const identity = projectDeliveryNodeIdentity(nodeOrType)
  const catalogLabel = normalizeProjectDeliveryNodeLabels(labels)
    .find((item) => projectDeliveryNodeLabelIdentity(item) === identity)
  return catalogLabel?.name || String(nodeOrType.label_name || '').trim() ||
    LEGACY_LABEL_BY_TYPE.get(String(nodeOrType.type || '') as ScrumProjectDeliveryNodeType) ||
    deliveryCopy.unnamedNode
}

export function normalizeProjectDeliveryNodes(
  nodes?: ReadonlyArray<ScrumProjectDeliveryNode> | null,
  labels?: ReadonlyArray<ScrumProjectDeliveryNodeLabel> | null,
): ScrumProjectDeliveryNode[] {
  const catalog = normalizeProjectDeliveryNodeLabels(labels)
  const labelById = new Map(catalog.filter((item) => Number(item.id || 0) > 0).map((item) => [Number(item.id), item]))
  const labelByClientKey = new Map(catalog.filter((item) => item.client_key).map((item) => [String(item.client_key), item]))
  const labelBySystemKey = new Map(
    catalog.filter((item) => item.system_key).map((item) => [String(item.system_key), item]),
  )
  const byIdentity = new Map<string, ScrumProjectDeliveryNode>()

  for (const item of nodes || []) {
    const legacyType = String(item?.type || '').trim()
    const requestedLabelId = Number(item?.label_id || 0)
    const requestedClientKey = String(item?.label_client_key || '').trim()
    const catalogLabel = requestedLabelId > 0
      ? labelById.get(requestedLabelId)
      : requestedClientKey
        ? labelByClientKey.get(requestedClientKey)
        : labelBySystemKey.get(legacyType)
    const labelId = Number(catalogLabel?.id || requestedLabelId || 0)
    const labelClientKey = String(catalogLabel?.client_key || requestedClientKey || '').trim()
    const identity = labelId > 0 ? `id:${labelId}` : labelClientKey ? `client:${labelClientKey}` : ''
    if (!identity || byIdentity.has(identity)) continue
    const ownerId = Number(item.owner_user_id || 0)
    const nodeType = legacyType || String(catalogLabel?.system_key || `custom:${labelId > 0 ? labelId : labelClientKey}`)
    byIdentity.set(identity, {
      ...(labelId > 0 ? { label_id: labelId } : {}),
      ...(labelId <= 0 && labelClientKey ? { label_client_key: labelClientKey } : {}),
      label_name: catalogLabel?.name || String(item.label_name || '').trim() ||
        LEGACY_LABEL_BY_TYPE.get(nodeType as ScrumProjectDeliveryNodeType) || deliveryCopy.fallbackNode(labelId || labelClientKey),
      label_deleted: item.label_deleted === undefined
        ? Boolean(catalogLabel?.deleted)
        : Boolean(item.label_deleted),
      type: nodeType,
      due_at: item.due_at ? String(item.due_at) : null,
      actual_completed_at: item.actual_completed_at ? String(item.actual_completed_at) : null,
      owner_user_id: Number.isFinite(ownerId) && ownerId > 0 ? ownerId : null,
    })
  }

  return [...byIdentity.values()].sort((left, right) => {
    const leftLabel = catalog.find((item) => projectDeliveryNodeLabelIdentity(item) === projectDeliveryNodeIdentity(left))
    const rightLabel = catalog.find((item) => projectDeliveryNodeLabelIdentity(item) === projectDeliveryNodeIdentity(right))
    const leftOrder = leftLabel?.sort_order ?? Number.MAX_SAFE_INTEGER
    const rightOrder = rightLabel?.sort_order ?? Number.MAX_SAFE_INTEGER
    if (leftOrder !== rightOrder) return leftOrder - rightOrder
    return projectDeliveryNodeIdentity(left).localeCompare(projectDeliveryNodeIdentity(right))
  })
}

export function availableProjectDeliveryNodeLabels(
  labels?: ReadonlyArray<ScrumProjectDeliveryNodeLabel> | null,
  nodes?: ReadonlyArray<ScrumProjectDeliveryNode> | null,
) {
  const selected = new Set(normalizeProjectDeliveryNodes(nodes, labels).map(projectDeliveryNodeIdentity))
  return normalizeProjectDeliveryNodeLabels(labels)
    .filter((item) => !item.deleted && !selected.has(projectDeliveryNodeLabelIdentity(item)))
}
