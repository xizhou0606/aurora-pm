import { useEffect, useRef } from 'react'

import type {
  BitableCollaborationClientPort,
  BitablePresenceState,
} from '../api/collaboration-port'
import type { BitableCollaborationRoom } from '../model/collaboration'

const DEFAULT_HEARTBEAT_MS = 10_000

export function useBitablePresence(input: {
  room?: BitableCollaborationRoom
  client?: BitableCollaborationClientPort
  connectionId: string
  canPublish: boolean
  getState?: () => BitablePresenceState
  heartbeatEveryMs?: number
}) {
  const getStateRef = useRef(input.getState)
  getStateRef.current = input.getState

  useEffect(() => {
    if (!input.room || !input.client || !input.canPublish) return
    const room = input.room
    const client = input.client
    let active = true
    let left = false
    let timer: number | undefined
    let lastState = ''
    let lastSentAt = 0
    const heartbeatEveryMs = input.heartbeatEveryMs ?? DEFAULT_HEARTBEAT_MS

    const leave = () => {
      if (left) return
      left = true
      active = false
      if (timer !== undefined) window.clearTimeout(timer)
      void client.leavePresence({ room, connectionId: input.connectionId }).catch(() => undefined)
    }
    const heartbeat = async () => {
      const state = getStateRef.current?.() ?? {}
      const fingerprint = JSON.stringify(state)
      try {
        if (fingerprint !== lastState || Date.now() - lastSentAt >= heartbeatEveryMs) {
          await client.heartbeatPresence({ room, connectionId: input.connectionId, state })
          if (!active) {
            // A heartbeat already in flight can arrive after pagehide cleanup.
            // Repeat idempotent leave after it settles so it cannot resurrect us.
            await client.leavePresence({ room, connectionId: input.connectionId })
            return
          }
          lastState = fingerprint
          lastSentAt = Date.now()
        }
      } catch {
        // The realtime stream remains authoritative; heartbeat failures are retried.
        lastSentAt = Date.now()
        lastState = fingerprint
      }
      // Poll only local state cheaply; send on a changed selection/edit state,
      // otherwise retain the normal heartbeat interval. Requests stay serial.
      if (active) timer = window.setTimeout(heartbeat, Math.min(250, heartbeatEveryMs))
    }

    window.addEventListener('pagehide', leave)
    void heartbeat()
    return () => {
      active = false
      window.removeEventListener('pagehide', leave)
      leave()
    }
  }, [
    input.canPublish,
    input.client,
    input.connectionId,
    input.heartbeatEveryMs,
    input.room?.resourceId,
    input.room?.workspaceId,
  ])
}
