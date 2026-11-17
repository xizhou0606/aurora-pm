import type {
  BitableCellHolder,
  BitableCellLease,
  BitableCellKey,
  BitableCollaborationRoom,
} from '../model/collaboration'
import type {
  BitableCellAcquireResult,
  BitableCommittedEvent,
  BitablePresenceMember,
  BitablePresenceState,
} from './collaboration-port'
import * as contract from './contract'

export function collaborationRoom(raw: unknown): BitableCollaborationRoom {
  const value = contract.object(raw)
  const workspaceId = contract.integer(value.workspace_id)
  if (workspaceId === 0) return contract.invalid()
  return { workspaceId, resourceId: contract.resourceId(value.resource_id) }
}

export function collaborationCellKey(raw: unknown): BitableCellKey {
  const value = contract.object(raw)
  return {
    ...collaborationRoom(value.room),
    tableId: contract.id(value.table_id),
    recordId: contract.id(value.record_id),
    fieldId: contract.id(value.field_id),
  }
}

export function collaborationHolder(raw: unknown): BitableCellHolder {
  const value = contract.object(raw)
  const actorUserId = contract.integer(value.actor_user_id)
  if (actorUserId === 0) return contract.invalid()
  return {
    actorUserId,
    actorName: contract.text(value.actor_display_name),
    expiresAt: contract.text(value.expires_at),
  }
}

export function collaborationLease(raw: unknown): BitableCellLease {
  const value = contract.object(raw)
  return {
    ...collaborationHolder(value),
    key: collaborationCellKey(value.key),
    leaseId: contract.id(value.lease_id),
    fencingToken: contract.integer(value.fencing_token),
    connectionId: contract.id(value.connection_id),
    editorInstanceId: contract.id(value.editor_instance_id),
    policyRevision: contract.integer(value.policy_revision),
  }
}

export function collaborationAcquire(raw: unknown): BitableCellAcquireResult {
  const value = contract.object(raw)
  return { status: 'acquired', lease: collaborationLease(value.lease) }
}

export function collaborationPresence(raw: unknown): BitablePresenceMember {
  const value = contract.object(raw)
  const actorUserId = contract.integer(value.actor_user_id)
  if (actorUserId === 0) return contract.invalid()
  return {
    connectionId: contract.id(value.connection_id),
    actorUserId,
    actorName: contract.text(value.actor_display_name),
    state: collaborationPresenceState(value.state),
    expiresAt: contract.text(value.expires_at),
  }
}

function presenceCell(value: unknown): BitablePresenceState['activeCell'] {
  try {
    const source = contract.object(value)
    return { recordId: contract.id(source.record_id), fieldId: contract.id(source.field_id) }
  } catch {
    return undefined
  }
}

export function collaborationPresenceState(raw: unknown): BitablePresenceState {
  let value: Readonly<Record<string, unknown>>
  try {
    value = contract.object(raw)
  } catch {
    return {}
  }
  const tableId = typeof value.table_id === 'string' && value.table_id ? value.table_id : undefined
  const viewId = typeof value.view_id === 'string' && value.view_id ? value.view_id : undefined
  const activeCell = presenceCell(value.active_cell)
  let selection: BitablePresenceState['selection']
  try {
    const source = contract.object(value.selection)
    const anchor = presenceCell(source.anchor)
    const focus = presenceCell(source.focus)
    if (anchor && focus) selection = { anchor, focus }
  } catch {
    selection = undefined
  }
  return { ...(tableId ? { tableId } : {}), ...(viewId ? { viewId } : {}), ...(activeCell ? { activeCell } : {}), ...(selection ? { selection } : {}) }
}

export function collaborationEvent(raw: unknown): BitableCommittedEvent {
  const value = contract.object(raw)
  const actorUserId = contract.integer(value.actor_user_id)
  if (actorUserId === 0) return contract.invalid()
  return {
    room: collaborationRoom(value.room),
    sequence: contract.integer(value.sequence),
    // Event operation keys are opaque and may be namespaced (e.g. policy:<hash>).
    // They are not record IDs and must not use the narrower record-ID grammar.
    operationId: contract.resourceId(value.operation_id),
    actorUserId,
    type: contract.text(value.type),
    payload: contract.object(value.payload),
    policyRevision: contract.integer(value.policy_revision),
    committedAt: contract.text(value.committed_at),
  }
}
