import type { CellAddress, CellValue } from './types'

export type BitableCollaborationRoom = {
  workspaceId: number
  resourceId: string
}

export type BitableCellKey = BitableCollaborationRoom & CellAddress & {
  tableId: string
}

export type BitableCellHolder = {
  actorUserId: number
  actorName: string
  expiresAt: string
}

export type BitableCellLease = BitableCellHolder & {
  key: BitableCellKey
  leaseId: string
  fencingToken: number
  connectionId: string
  editorInstanceId: string
  policyRevision: number
}

export type BitableCellLeaseProof = Pick<
  BitableCellLease,
  'key' | 'leaseId' | 'fencingToken' | 'connectionId' | 'editorInstanceId'
>

export type BitableCellDraft = {
  key: BitableCellKey
  baseRevision: number
  original: CellValue
  value: CellValue
}

export type BitableCellFreezeReason =
  | 'lease_lost'
  | 'conflict'
  | 'unknown'
  | 'acquire_failed'

export type BitableCellCollaborationState =
  | { phase: 'view' }
  | { phase: 'acquiring'; attemptId: string; draft: BitableCellDraft }
  | { phase: 'occupied'; draft: BitableCellDraft; holder: BitableCellHolder }
  | { phase: 'editing'; draft: BitableCellDraft; lease: BitableCellLease }
  | {
    phase: 'saving'
    attemptId: string
    operationId: string
    draft: BitableCellDraft
    lease: BitableCellLease
  }
  | {
    phase: 'frozen'
    draft: BitableCellDraft
    reason: BitableCellFreezeReason
    lease?: BitableCellLease
  }

export type BitableCellCollaborationEvent =
  | { type: 'begin'; attemptId: string; draft: BitableCellDraft }
  | { type: 'acquired'; attemptId: string; lease: BitableCellLease }
  | { type: 'occupied'; attemptId: string; holder: BitableCellHolder }
  | { type: 'acquire_failed'; attemptId: string }
  | { type: 'change'; value: CellValue }
  | { type: 'save_started'; attemptId: string; operationId: string }
  | {
    type: 'save_frozen'
    attemptId: string
    reason: Extract<BitableCellFreezeReason, 'lease_lost' | 'conflict' | 'unknown'>
  }
  | { type: 'save_committed'; attemptId: string }
  | { type: 'lease_renewed'; lease: BitableCellLease }
  | { type: 'lease_lost' }
  | { type: 'retry'; attemptId: string }
  | { type: 'discard' | 'reset' }

export function bitableCellKey(
  room: BitableCollaborationRoom,
  tableId: string,
  address: CellAddress,
): BitableCellKey {
  return { ...room, tableId, ...address }
}

export function bitableCellLeaseProof(lease: BitableCellLease): BitableCellLeaseProof {
  const { key, leaseId, fencingToken, connectionId, editorInstanceId } = lease
  return { key, leaseId, fencingToken, connectionId, editorInstanceId }
}

function activeAttempt(
  state: BitableCellCollaborationState,
  attemptId: string,
): boolean {
  return (state.phase === 'acquiring' || state.phase === 'saving')
    && state.attemptId === attemptId
}

export function reduceBitableCellCollaboration(
  state: BitableCellCollaborationState,
  event: BitableCellCollaborationEvent,
): BitableCellCollaborationState {
  if (event.type === 'reset' || event.type === 'discard') return { phase: 'view' }
  if (event.type === 'begin') {
    return state.phase === 'view'
      ? { phase: 'acquiring', attemptId: event.attemptId, draft: event.draft }
      : state
  }
  if (event.type === 'retry') {
    return state.phase === 'frozen' || state.phase === 'occupied'
      ? { phase: 'acquiring', attemptId: event.attemptId, draft: state.draft }
      : state
  }
  if (event.type === 'acquired') {
    return state.phase === 'acquiring' && activeAttempt(state, event.attemptId)
      ? { phase: 'editing', draft: state.draft, lease: event.lease }
      : state
  }
  if (event.type === 'occupied') {
    return state.phase === 'acquiring' && activeAttempt(state, event.attemptId)
      ? { phase: 'occupied', draft: state.draft, holder: event.holder }
      : state
  }
  if (event.type === 'acquire_failed') {
    return state.phase === 'acquiring' && activeAttempt(state, event.attemptId)
      ? { phase: 'frozen', draft: state.draft, reason: 'acquire_failed' }
      : state
  }
  if (event.type === 'change') {
    return state.phase === 'editing'
      ? { ...state, draft: { ...state.draft, value: event.value } }
      : state
  }
  if (event.type === 'save_started') {
    return state.phase === 'editing'
      ? { ...state, phase: 'saving', attemptId: event.attemptId, operationId: event.operationId }
      : state
  }
  if (event.type === 'save_committed') {
    return activeAttempt(state, event.attemptId) && state.phase === 'saving'
      ? { phase: 'view' }
      : state
  }
  if (event.type === 'save_frozen') {
    return activeAttempt(state, event.attemptId) && state.phase === 'saving'
      ? { phase: 'frozen', draft: state.draft, lease: state.lease, reason: event.reason }
      : state
  }
  if (event.type === 'lease_renewed') {
    return (state.phase === 'editing' || state.phase === 'saving')
      && state.lease.leaseId === event.lease.leaseId
      ? { ...state, lease: event.lease }
      : state
  }
  if (event.type === 'lease_lost') {
    return state.phase === 'acquiring'
      ? { phase: 'frozen', draft: state.draft, reason: 'lease_lost' }
      : state.phase === 'editing' || state.phase === 'saving'
        ? { phase: 'frozen', draft: state.draft, lease: state.lease, reason: 'lease_lost' }
        : state
  }
  return state
}

export function presentBitableCellCollaboration(state: BitableCellCollaborationState) {
  return {
    phase: state.phase,
    draft: state.phase === 'view' ? undefined : state.draft,
    holder: state.phase === 'occupied' ? state.holder : undefined,
    freezeReason: state.phase === 'frozen' ? state.reason : undefined,
    canChange: state.phase === 'editing',
    canSave: state.phase === 'editing',
    isActive: state.phase !== 'view',
    isFrozen: state.phase === 'frozen',
  }
}
