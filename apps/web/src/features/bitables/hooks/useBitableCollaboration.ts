import { useEffect, useSyncExternalStore } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { clearResourceAccessCredential } from '@/features/resource-access'
import type { BitableCollaborationClientPort } from '../api/collaboration-port'
import type { TableScope } from '../api/client'
import { bitableCellKey } from '../model/collaboration'
import type { CellAddress } from '../model/types'
import { bitableKey, type BitableEnvironment } from './environment'
import { useBitableCellCollaboration } from './useBitableCellCollaboration'
import { useBitableRealtimeEvents } from './useBitableRealtimeEvents'
import { useBitablePresence } from './useBitablePresence'
import type { BitablePresenceState } from '../api/collaboration-port'

const subscribeVisibility = (listener: () => void) => {
  document.addEventListener('visibilitychange', listener)
  return () => document.removeEventListener('visibilitychange', listener)
}
const isVisible = () => document.visibilityState !== 'hidden'

export function useBitableCollaboration(input: {
  environment: BitableEnvironment
  scope: TableScope
  resourceId?: string
  client?: BitableCollaborationClientPort
  connectionId: string
  editorInstanceId: string
  canRead: boolean
  canEdit: boolean
  renewAfterMs?: number
  heartbeatEveryMs?: number
  getPresenceState?: () => BitablePresenceState
}) {
  const queryClient = useQueryClient()
  const visible = useSyncExternalStore(subscribeVisibility, isVisible, () => true)
  const room = input.resourceId
    ? { workspaceId: input.environment.workspaceId, resourceId: input.resourceId }
    : undefined
  const realtime = useBitableRealtimeEvents({
    environment: input.environment,
    scope: input.scope,
    resourceId: input.resourceId,
    client: input.client,
    connectionId: input.connectionId,
    canRead: input.canRead,
  })
  const cell = useBitableCellCollaboration({
    client: input.client,
    room,
    tableId: input.scope.tableId,
    connectionId: input.connectionId,
    editorInstanceId: input.editorInstanceId,
    sessionRevision: input.environment.sessionRevision,
    canEdit: visible && input.canRead && input.canEdit && realtime.status !== 'revoked',
    renewAfterMs: input.renewAfterMs,
    onCommitted: () => void queryClient.invalidateQueries({
      queryKey: bitableKey(input.environment, input.scope),
    }),
  })
  useBitablePresence({
    room,
    client: input.client,
    connectionId: input.connectionId,
    canPublish: visible && input.canRead && input.canEdit && realtime.status !== 'revoked',
    getState: input.getPresenceState,
    heartbeatEveryMs: input.heartbeatEveryMs,
  })
  useEffect(() => {
    if (realtime.status === 'revoked') clearResourceAccessCredential('bitable')
  }, [realtime.status])
  return {
    availability: !input.client || !room
      ? 'unavailable' as const
      : !input.canRead || realtime.status === 'revoked'
        ? 'forbidden' as const
        : 'ready' as const,
    realtime,
    cell,
    lockKey: (address: CellAddress) => room
      ? bitableCellKey(room, input.scope.tableId, address)
      : undefined,
  }
}
