import * as Y from 'yjs'
import type { DurableAcknowledgement } from './contract.js'

export type CollaborationSaveState = 'memory_synced' | 'durably_saved'

export function parseDurableAcknowledgement(payload: string): DurableAcknowledgement | undefined {
  let value: unknown
  try {
    value = JSON.parse(payload)
  } catch {
    return undefined
  }
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return undefined
  const candidate = value as Record<string, unknown>
  if (
    candidate.type !== 'durable_commit' ||
    typeof candidate.roomName !== 'string' ||
    typeof candidate.stateVectorBase64 !== 'string' ||
    typeof candidate.commitId !== 'string' ||
    !Number.isSafeInteger(candidate.durableRevision) ||
    typeof candidate.snapshotHash !== 'string' ||
    typeof candidate.projectionHash !== 'string' ||
    typeof candidate.storedAt !== 'string'
  ) return undefined
  return candidate as unknown as DurableAcknowledgement
}

export function saveStateFor(document: Y.Doc, acknowledgement?: DurableAcknowledgement): CollaborationSaveState {
  if (!acknowledgement) return 'memory_synced'
  let durableVector: Map<number, number>
  try {
    durableVector = Y.decodeStateVector(Buffer.from(acknowledgement.stateVectorBase64, 'base64'))
  } catch {
    return 'memory_synced'
  }
  const localVector = Y.decodeStateVector(Y.encodeStateVector(document))
  for (const [clientId, clock] of localVector) {
    if ((durableVector.get(clientId) ?? 0) < clock) return 'memory_synced'
  }
  return 'durably_saved'
}
