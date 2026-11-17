import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { BITABLE_COLLABORATION_COPY as copy } from '@/shared/copy'

import type {
  BitableCollaborationClientPort,
  BitablePresenceMember,
} from '../api/collaboration-port'
import type { TableScope } from '../api/client'
import type { BitableCollaborationRoom } from '../model/collaboration'
import { bitableKey, type BitableEnvironment } from './environment'

export type BitableRealtimeState = {
  status: 'unavailable' | 'connecting' | 'live' | 'error' | 'revoked'
  members: readonly BitablePresenceMember[]
  lastSequence?: number
}

function sameRoom(left: BitableCollaborationRoom, right: BitableCollaborationRoom) {
  return left.workspaceId === right.workspaceId && left.resourceId === right.resourceId
}

export function useBitableRealtimeEvents(input: {
  environment: BitableEnvironment
  scope: TableScope
  resourceId?: string
  client?: BitableCollaborationClientPort
  connectionId: string
  canRead: boolean
}) {
  const queryClient = useQueryClient()
  const room = input.resourceId
    ? { workspaceId: input.environment.workspaceId, resourceId: input.resourceId }
    : undefined
  const identity = room
    ? `${input.environment.sessionRevision}:${room.workspaceId}:${room.resourceId}:${input.scope.resourceId}:${input.scope.tableId}`
    : `${input.environment.sessionRevision}:unavailable:${input.scope.resourceId}:${input.scope.tableId}`
  const [state, setState] = useState<BitableRealtimeState>({
    status: room && input.client && input.canRead ? 'connecting' : 'unavailable',
    members: [],
  })
  const cursor = useRef<number | undefined>(undefined)

  useEffect(() => {
    cursor.current = undefined
    if (!room || !input.client || !input.canRead) {
      setState({ status: 'unavailable', members: [] })
      return
    }
    let active = true
    let refreshing = false
    let dirty = false
    let timer: ReturnType<typeof setTimeout> | undefined
    setState({ status: 'connecting', members: [] })
    const invalidate = () => {
      dirty = true
      if (refreshing || timer !== undefined) return
      timer = setTimeout(async () => {
        timer = undefined
        if (!active) return
        dirty = false
        refreshing = true
        try {
          // Never cancel an in-flight read for every event in a replay/burst.
          await queryClient.invalidateQueries({
            queryKey: bitableKey(input.environment, input.scope),
          }, { cancelRefetch: false })
        } finally {
          refreshing = false
          // A commit received during the read needs one trailing refresh.
          if (active && dirty) invalidate()
        }
      }, 100)
    }
    const close = input.client.subscribe({
      room,
      connectionId: input.connectionId,
      afterSequence: cursor.current,
      onSignal: (signal) => {
        if (!active) return
        if (signal.kind === 'connected') {
          setState((current) => ({ ...current, status: 'live' }))
          return
        }
        if (signal.kind === 'error') {
          setState((current) => ({ ...current, status: 'error' }))
          return
        }
        if (!sameRoom(signal.kind === 'committed' ? signal.event.room : signal.room, room)) return
        if (signal.kind === 'committed') {
          if (cursor.current !== undefined && signal.event.sequence <= cursor.current) return
          cursor.current = signal.event.sequence
          setState((current) => ({ ...current, status: 'live', lastSequence: signal.event.sequence }))
          invalidate()
        } else if (signal.kind === 'presence') {
          setState((current) => ({ ...current, status: 'live', members: signal.members }))
        } else if (signal.kind === 'reset_required') {
          cursor.current = undefined
          invalidate()
        } else if (signal.kind === 'access_revoked') {
          active = false
          if (timer !== undefined) clearTimeout(timer)
          setState({ status: 'revoked', members: [] })
          queryClient.removeQueries({ queryKey: bitableKey(input.environment, input.scope) })
        }
      },
    })
    return () => { active = false; if (timer !== undefined) clearTimeout(timer); close() }
  }, [identity, input.canRead, input.client, input.connectionId, queryClient])

  const message = state.status === 'unavailable'
    ? copy.unavailable
    : state.status === 'error'
      ? copy.reconnecting
      : state.status === 'revoked'
        ? copy.accessRevoked
        : ''
  return { ...state, message }
}
