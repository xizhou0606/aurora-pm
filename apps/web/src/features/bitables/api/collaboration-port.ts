import type {
  BitableCellHolder,
  BitableCellLease,
  BitableCellLeaseProof,
  BitableCellKey,
  BitableCollaborationRoom,
} from '../model/collaboration'
import type { CellValue } from '../model/types'

export type BitableCellAcquireResult =
  | { status: 'acquired'; lease: BitableCellLease }
  | { status: 'held'; holder: BitableCellHolder }

export type BitableCellSaveResult =
  | { status: 'committed'; operationId: string }
  | { status: 'lease_lost' | 'conflict' | 'unknown' }

export type BitablePresenceMember = {
  connectionId: string
  actorUserId: number
  actorName: string
  state: BitablePresenceState
  expiresAt: string
}

export type BitablePresenceState = {
  tableId?: string
  viewId?: string
  activeCell?: { recordId: string; fieldId: string }
  selection?: {
    anchor: { recordId: string; fieldId: string }
    focus: { recordId: string; fieldId: string }
  }
}

export type BitableCommittedEvent = {
  room: BitableCollaborationRoom
  sequence: number
  operationId: string
  actorUserId: number
  type: string
  payload: Readonly<Record<string, unknown>>
  policyRevision: number
  committedAt: string
}

export type BitableRealtimeSignal =
  | { kind: 'connected' }
  | { kind: 'committed'; event: BitableCommittedEvent }
  | { kind: 'presence'; room: BitableCollaborationRoom; members: readonly BitablePresenceMember[] }
  | { kind: 'reset_required'; room: BitableCollaborationRoom; reason: string }
  | { kind: 'access_revoked'; room: BitableCollaborationRoom }
  | { kind: 'error' }

export type BitableCollaborationClientPort = {
  acquireCell(input: {
    key: BitableCellKey
    connectionId: string
    editorInstanceId: string
  }): Promise<BitableCellAcquireResult>
  renewCell(proof: BitableCellLeaseProof): Promise<BitableCellLease>
  releaseCell(proof: BitableCellLeaseProof): Promise<void>
  disconnect(input: { room: BitableCollaborationRoom; connectionId: string }): Promise<void>
  heartbeatPresence(input: {
    room: BitableCollaborationRoom
    connectionId: string
    state: BitablePresenceState
  }): Promise<BitablePresenceMember>
  leavePresence(input: { room: BitableCollaborationRoom; connectionId: string }): Promise<void>
  saveCell(input: {
    proof: BitableCellLeaseProof
    baseRevision: number
    baseValue: CellValue
    value: CellValue
    operationId: string
  }): Promise<BitableCellSaveResult>
  subscribe(input: {
    room: BitableCollaborationRoom
    connectionId: string
    afterSequence?: number
    onSignal: (signal: BitableRealtimeSignal) => void
  }): () => void
}

export type BitableGuardedCellSaver = BitableCollaborationClientPort['saveCell']
