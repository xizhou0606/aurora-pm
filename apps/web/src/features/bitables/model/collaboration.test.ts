import { describe, expect, it } from 'bun:test'

import {
  bitableCellKey,
  reduceBitableCellCollaboration,
  type BitableCellCollaborationState,
  type BitableCellDraft,
  type BitableCellLease,
} from './collaboration'

const room = { workspaceId: 7, resourceId: 'unified-resource' }
const draft: BitableCellDraft = {
  key: bitableCellKey(room, 'table', { recordId: 'record', fieldId: 'field' }),
  baseRevision: 3,
  original: 'before',
  value: 'after',
}
const lease: BitableCellLease = {
  key: draft.key,
  leaseId: 'lease-1',
  fencingToken: 4,
  connectionId: 'connection-1',
  editorInstanceId: 'editor-1',
  actorUserId: 9,
  actorName: 'Editor',
  policyRevision: 2,
  expiresAt: '2026-09-08T12:00:00Z',
}

describe('bitable cell collaboration state', () => {
  it('moves through acquire, edit, save and committed view', () => {
    let state: BitableCellCollaborationState = { phase: 'view' }
    state = reduceBitableCellCollaboration(state, { type: 'begin', attemptId: 'acquire', draft })
    expect(state.phase).toBe('acquiring')
    state = reduceBitableCellCollaboration(state, { type: 'acquired', attemptId: 'acquire', lease })
    state = reduceBitableCellCollaboration(state, { type: 'change', value: 'new draft' })
    expect(state).toMatchObject({ phase: 'editing', draft: { value: 'new draft' } })
    state = reduceBitableCellCollaboration(state, {
      type: 'save_started', attemptId: 'save', operationId: 'operation',
    })
    expect(state.phase).toBe('saving')
    state = reduceBitableCellCollaboration(state, { type: 'save_committed', attemptId: 'save' })
    expect(state).toEqual({ phase: 'view' })
  })

  it('freezes and retains the draft when the lease is lost during save', () => {
    let state: BitableCellCollaborationState = {
      phase: 'editing', draft: { ...draft, value: 'local draft' }, lease,
    }
    state = reduceBitableCellCollaboration(state, {
      type: 'save_started', attemptId: 'save', operationId: 'operation',
    })
    state = reduceBitableCellCollaboration(state, { type: 'lease_lost' })
    expect(state).toMatchObject({ phase: 'frozen', reason: 'lease_lost', draft: { value: 'local draft' } })
    expect(reduceBitableCellCollaboration(state, {
      type: 'save_committed', attemptId: 'save',
    })).toEqual(state)
  })

  it('ignores an old acquire result after an explicit retry', () => {
    const occupied = reduceBitableCellCollaboration(
      { phase: 'acquiring', attemptId: 'first', draft },
      {
        type: 'occupied', attemptId: 'first',
        holder: { actorUserId: 10, actorName: 'Other', expiresAt: 'later' },
      },
    )
    const retrying = reduceBitableCellCollaboration(occupied, { type: 'retry', attemptId: 'second' })
    const late = reduceBitableCellCollaboration(retrying, {
      type: 'acquired', attemptId: 'first', lease,
    })
    expect(late).toEqual(retrying)
  })

  it('uses the same authoritative lock key across standalone and referenced surfaces', () => {
    const address = { recordId: 'record', fieldId: 'field' }
    expect(bitableCellKey(room, 'table', address)).toEqual(bitableCellKey(room, 'table', address))
    expect(bitableCellKey(room, 'table', address)).not.toHaveProperty('surfaceId')
    expect(bitableCellKey(room, 'table', address).resourceId).toBe('unified-resource')
  })
})
