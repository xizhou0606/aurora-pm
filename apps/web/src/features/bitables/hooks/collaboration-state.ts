import { BITABLE_COLLABORATION_COPY as copy } from '@/shared/copy'

import type {
  BitableCellCollaborationState,
  BitableCellLease,
} from '../model/collaboration'

export function activeLease(state: BitableCellCollaborationState): BitableCellLease | undefined {
  if (state.phase === 'editing' || state.phase === 'saving' || state.phase === 'frozen') {
    return state.lease
  }
  return undefined
}

export function collaborationMessage(state: BitableCellCollaborationState): string {
  if (state.phase === 'acquiring') return copy.acquiring
  if (state.phase === 'occupied') return copy.occupied(state.holder.actorName)
  if (state.phase !== 'frozen') return ''
  if (state.reason === 'lease_lost') return copy.leaseLost
  if (state.reason === 'conflict') return copy.conflict
  if (state.reason === 'unknown') return copy.unknown
  return copy.acquireFailed
}
