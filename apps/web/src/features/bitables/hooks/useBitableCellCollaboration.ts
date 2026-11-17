import { useEffect, useRef, useState } from 'react'

import type { BitableCollaborationClientPort } from '../api/collaboration-port'
import {
  bitableCellKey,
  bitableCellLeaseProof,
  presentBitableCellCollaboration,
  reduceBitableCellCollaboration,
  type BitableCellCollaborationEvent,
  type BitableCellCollaborationState,
  type BitableCellDraft,
  type BitableCellLease,
  type BitableCollaborationRoom,
} from '../model/collaboration'
import type { CellAddress, CellValue } from '../model/types'
import { activeLease, collaborationMessage } from './collaboration-state'

const initialState: BitableCellCollaborationState = { phase: 'view' }

export function useBitableCellCollaboration(input: {
  client?: BitableCollaborationClientPort
  room?: BitableCollaborationRoom
  tableId: string
  connectionId: string
  editorInstanceId: string
  sessionRevision: number
  canEdit: boolean
  renewAfterMs?: number
  onCommitted?: () => void
}) {
  const [state, setState] = useState<BitableCellCollaborationState>(initialState)
  const stateRef = useRef(state)
  const generation = useRef(0)
  const mounted = useRef(true)
  const identity = input.room
    ? `${input.sessionRevision}:${input.room.workspaceId}:${input.room.resourceId}:${input.tableId}:${input.connectionId}:${input.editorInstanceId}`
    : `${input.sessionRevision}:unavailable:${input.tableId}:${input.connectionId}:${input.editorInstanceId}`
  const active = useRef({ identity, canEdit: input.canEdit })
  active.current = { identity, canEdit: input.canEdit }

  const transition = (event: BitableCellCollaborationEvent) => {
    const next = reduceBitableCellCollaboration(stateRef.current, event)
    stateRef.current = next
    if (mounted.current) setState(next)
  }
  const release = (lease: BitableCellLease | undefined) => {
    if (lease && input.client) void input.client.releaseCell(bitableCellLeaseProof(lease)).catch(() => undefined)
  }

  useEffect(() => {
    generation.current += 1
    release(activeLease(stateRef.current))
    transition({ type: 'reset' })
    return () => {
      generation.current += 1
      release(activeLease(stateRef.current))
    }
    // identity owns the lifecycle; the client/room fields are represented in it.
  }, [identity, input.client])

  useEffect(() => {
    if (input.canEdit) return
    const current = stateRef.current
    if (current.phase === 'view' || current.phase === 'occupied' || current.phase === 'frozen') return
    generation.current += 1
    release(activeLease(current))
    transition({ type: 'lease_lost' })
  }, [input.canEdit])

  useEffect(() => {
    const current = state
    if (!input.client || (current.phase !== 'editing' && current.phase !== 'saving')) return
    const owner = generation.current
    const timer = window.setTimeout(() => {
      void input.client?.renewCell(bitableCellLeaseProof(current.lease)).then((lease) => {
        if (owner !== generation.current || active.current.identity !== identity || !active.current.canEdit) return
        transition({ type: 'lease_renewed', lease })
      }).catch(() => {
        if (owner !== generation.current || active.current.identity !== identity) return
        generation.current += 1
        transition({ type: 'lease_lost' })
      })
    }, input.renewAfterMs ?? 10_000)
    return () => window.clearTimeout(timer)
  }, [identity, input.client, input.renewAfterMs, state])

  useEffect(() => {
    const onPageHide = () => {
      const lease = activeLease(stateRef.current)
      if (!lease) return
      generation.current += 1
      release(lease)
      transition({ type: 'lease_lost' })
    }
    window.addEventListener('pagehide', onPageHide)
    return () => window.removeEventListener('pagehide', onPageHide)
  }, [input.client])

  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false }
  }, [])

  const acquire = async (draft: BitableCellDraft, retry: boolean): Promise<boolean> => {
    if (!input.client || !input.room || !input.canEdit) return false
    const attemptId = crypto.randomUUID()
    const owner = generation.current
    transition(retry ? { type: 'retry', attemptId } : { type: 'begin', attemptId, draft })
    try {
      const result = await input.client.acquireCell({
        key: draft.key,
        connectionId: input.connectionId,
        editorInstanceId: input.editorInstanceId,
      })
      if (owner !== generation.current || active.current.identity !== identity || !active.current.canEdit) {
        if (result.status === 'acquired') release(result.lease)
        return false
      }
      if (result.status === 'held') {
        transition({ type: 'occupied', attemptId, holder: result.holder })
        return false
      }
      transition({ type: 'acquired', attemptId, lease: result.lease })
      return true
    } catch {
      if (owner === generation.current && active.current.identity === identity) {
        transition({ type: 'acquire_failed', attemptId })
      }
      return false
    }
  }

  const begin = (address: CellAddress, baseRevision: number, original: CellValue, initial = original) => {
    if (!input.room) return Promise.resolve(false)
    const current = stateRef.current
    if (current.phase === 'occupied' || current.phase === 'frozen') {
      const sameCell = current.draft.key.recordId === address.recordId && current.draft.key.fieldId === address.fieldId
      if (sameCell) return acquire(current.draft, true)
      // A denied acquire owns no draft editor or lease. Selecting another cell
      // must acquire that cell, not retry the previously denied address.
      if (current.phase === 'occupied') transition({ type: 'discard' })
      else return Promise.resolve(false) // Never silently discard a frozen draft.
    } else if (current.phase !== 'view') return Promise.resolve(false)
    return acquire({
      key: bitableCellKey(input.room, input.tableId, address),
      baseRevision,
      original,
      value: initial,
    }, false)
  }

  const save = async (): Promise<boolean> => {
    const current = stateRef.current
    if (!input.client || !input.canEdit || current.phase !== 'editing') return false
    const attemptId = crypto.randomUUID()
    const operationId = crypto.randomUUID()
    const owner = generation.current
    transition({ type: 'save_started', attemptId, operationId })
    try {
      const result = await input.client.saveCell({
        proof: bitableCellLeaseProof(current.lease),
        baseRevision: current.draft.baseRevision,
        baseValue: current.draft.original,
        value: current.draft.value,
        operationId,
      })
      if (owner !== generation.current || active.current.identity !== identity) return false
      if (result.status === 'committed' && result.operationId === operationId) {
        transition({ type: 'save_committed', attemptId })
        release(current.lease)
        input.onCommitted?.()
        return true
      }
      transition({
        type: 'save_frozen',
        attemptId,
        reason: result.status === 'committed' ? 'unknown' : result.status,
      })
      return false
    } catch {
      if (owner === generation.current && active.current.identity === identity) {
        transition({ type: 'save_frozen', attemptId, reason: 'unknown' })
      }
      return false
    }
  }

  const availability = !input.client || !input.room ? 'unavailable' : input.canEdit ? 'ready' : 'forbidden'
  const presented = presentBitableCellCollaboration(state)
  return {
    state,
    view: { ...presented, availability, message: collaborationMessage(state) },
    begin,
    change: (value: CellValue) => transition({ type: 'change', value }),
    save,
    retry: () => {
      const current = stateRef.current
      return current.phase === 'frozen' || current.phase === 'occupied'
        ? acquire(current.draft, true)
        : Promise.resolve(false)
    },
    discard: () => {
      generation.current += 1
      release(activeLease(stateRef.current))
      transition({ type: 'discard' })
    },
  }
}
