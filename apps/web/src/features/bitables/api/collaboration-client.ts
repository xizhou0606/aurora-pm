import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

import type {
  BitableCellKey,
  BitableCellLease,
  BitableCollaborationRoom,
} from '../model/collaboration'
import {
  collaborationAcquire,
  collaborationHolder,
  collaborationLease,
  collaborationPresence,
} from './collaboration-contract'
import { consumeCollaborationSse } from './collaboration-sse'
import type {
  BitableCollaborationClientPort,
  BitableGuardedCellSaver,
} from './collaboration-port'
import * as contract from './contract'

const base = (resourceId: string) => `/document-resources/${contract.resourceId(resourceId)}/collaboration`
const CONNECTION_HEADER = 'X-PM-Collaboration-Connection'
const cleanupTransport = { adapter: 'fetch' as const, fetchOptions: { keepalive: true } }
const CANONICAL_UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
export const bitableCollaborationEndpoints = {
  events: (resourceId: string) => `${base(resourceId)}/events`,
  acquire: (resourceId: string) => `${base(resourceId)}/cell-leases/acquire`,
  renew: (resourceId: string) => `${base(resourceId)}/cell-leases/renew`,
  release: (resourceId: string) => `${base(resourceId)}/cell-leases/release`,
  disconnect: (resourceId: string) => `${base(resourceId)}/cell-leases/disconnect`,
  presenceHeartbeat: (resourceId: string) => `${base(resourceId)}/presence/heartbeat`,
  presence: (resourceId: string) => `${base(resourceId)}/presence`,
} as const

export function createBitableCollaborationConnectionId(): string {
  return crypto.randomUUID()
}

async function request<T>(call: () => Promise<unknown>, normalize: (value: unknown) => T): Promise<T> {
  try {
    return normalize(unwrapApiData(await call()))
  } catch (error) {
    throw toSafeApiError(error, 'BITABLE_COLLABORATION_REQUEST_FAILED')
  }
}

function sameKey(left: BitableCellKey, right: BitableCellKey): boolean {
  return left.workspaceId === right.workspaceId
    && left.resourceId === right.resourceId
    && left.tableId === right.tableId
    && left.recordId === right.recordId
    && left.fieldId === right.fieldId
}

function scopedLease(
  lease: BitableCellLease,
  key: BitableCellKey,
  connectionId: string,
  editorInstanceId: string,
): BitableCellLease {
  if (!sameKey(lease.key, key)
    || lease.connectionId !== connectionId
    || lease.editorInstanceId !== editorInstanceId) {
    throw new Error('Collaboration lease scope mismatch')
  }
  return lease
}

function heldLease(error: unknown) {
  const response = (error as { response?: { status?: number; data?: unknown } }).response
  if (response?.status !== 409) return undefined
  try {
    const payload = contract.object(unwrapApiData(response))
    return payload.holder
      ? { status: 'held' as const, holder: collaborationHolder(payload.holder) }
      : undefined
  } catch {
    return undefined
  }
}

function waitForRetry(signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const finish = () => {
      window.clearTimeout(timer)
      signal.removeEventListener('abort', finish)
      resolve()
    }
    const timer = window.setTimeout(finish, 1_000)
    signal.addEventListener('abort', finish, { once: true })
  })
}

function presenceState(state: Parameters<BitableCollaborationClientPort['heartbeatPresence']>[0]['state']) {
  const cell = (value: { recordId: string; fieldId: string }) => ({
    record_id: contract.id(value.recordId),
    field_id: contract.id(value.fieldId),
  })
  return {
    ...(state.tableId ? { table_id: contract.id(state.tableId) } : {}),
    ...(state.viewId ? { view_id: contract.id(state.viewId) } : {}),
    ...(state.activeCell ? { active_cell: cell(state.activeCell) } : {}),
    ...(state.selection ? { selection: {
      anchor: cell(state.selection.anchor),
      focus: cell(state.selection.focus),
    } } : {}),
  }
}

async function openEventStream(input: {
  room: BitableCollaborationRoom
  connectionId: string
  afterSequence?: number
  signal: AbortSignal
  onSignal: Parameters<BitableCollaborationClientPort['subscribe']>[0]['onSignal']
}) {
  let cursor = input.afterSequence
  while (!input.signal.aborted) {
    try {
      const response = await platformApi.get(bitableCollaborationEndpoints.events(input.room.resourceId), {
        adapter: 'fetch',
        responseType: 'stream',
        signal: input.signal,
        headers: {
          [CONNECTION_HEADER]: input.connectionId,
          ...(cursor ? { 'Last-Event-ID': String(cursor) } : {}),
        },
      })
      const stream = response.data
      if (!(stream instanceof ReadableStream)) throw new Error('Collaboration stream unavailable')
      input.onSignal({ kind: 'connected' })
      const result = await consumeCollaborationSse({ stream, room: input.room, onSignal: (signal) => {
        if (signal.kind === 'committed') cursor = signal.event.sequence
        if (signal.kind === 'reset_required') cursor = undefined
        input.onSignal(signal)
      } })
      if (result.revoked || input.signal.aborted) return
      cursor = result.reset ? undefined : result.cursor ?? cursor
      input.onSignal({ kind: 'error' })
    } catch {
      if (input.signal.aborted) return
      input.onSignal({ kind: 'error' })
    }
    await waitForRetry(input.signal)
  }
}

export function createBitableCollaborationClient(
  connectionId: string,
  saveCell: BitableGuardedCellSaver,
): BitableCollaborationClientPort {
  if (!CANONICAL_UUID.test(connectionId)) {
    throw new Error('Collaboration connection id must be a canonical lowercase UUID')
  }
  const headersFor = (candidate: string) => {
    if (candidate !== connectionId) throw new Error('Collaboration connection scope mismatch')
    return { [CONNECTION_HEADER]: connectionId }
  }
  return {
    acquireCell(input) {
      return platformApi.post(bitableCollaborationEndpoints.acquire(input.key.resourceId), {
        table_id: input.key.tableId,
        record_id: input.key.recordId,
        field_id: input.key.fieldId,
        editor_instance_id: input.editorInstanceId,
      }, { headers: headersFor(input.connectionId) }).then((response) => {
        const result = collaborationAcquire(unwrapApiData(response))
        if (result.status !== 'acquired') return contract.invalid()
        scopedLease(result.lease, input.key, input.connectionId, input.editorInstanceId)
        return result
      }).catch((error: unknown) => {
        const held = heldLease(error)
        if (held) return held
        throw toSafeApiError(error, 'BITABLE_COLLABORATION_REQUEST_FAILED')
      })
    },
    renewCell(proof) {
      return request(
        () => platformApi.post(bitableCollaborationEndpoints.renew(proof.key.resourceId), {
          table_id: proof.key.tableId,
          record_id: proof.key.recordId,
          field_id: proof.key.fieldId,
          editor_instance_id: proof.editorInstanceId,
          lease_id: proof.leaseId,
          fencing_token: proof.fencingToken,
        }, { headers: headersFor(proof.connectionId) }),
        (value) => scopedLease(
          collaborationLease(contract.object(value).lease),
          proof.key,
          proof.connectionId,
          proof.editorInstanceId,
        ),
      )
    },
    releaseCell(proof) {
      return request(
        () => platformApi.post(bitableCollaborationEndpoints.release(proof.key.resourceId), {
          table_id: proof.key.tableId,
          record_id: proof.key.recordId,
          field_id: proof.key.fieldId,
          editor_instance_id: proof.editorInstanceId,
          lease_id: proof.leaseId,
          fencing_token: proof.fencingToken,
        }, { ...cleanupTransport, headers: headersFor(proof.connectionId) }),
        () => undefined,
      )
    },
    disconnect(input) {
      return request(
        () => platformApi.post(
          bitableCollaborationEndpoints.disconnect(input.room.resourceId),
          undefined,
          { ...cleanupTransport, headers: headersFor(input.connectionId) },
        ),
        () => undefined,
      )
    },
    heartbeatPresence(input) {
      return request(
        () => platformApi.post(
          bitableCollaborationEndpoints.presenceHeartbeat(input.room.resourceId),
          { state: presenceState(input.state) },
          { headers: headersFor(input.connectionId) },
        ),
        (value) => collaborationPresence(contract.object(value).presence),
      )
    },
    leavePresence(input) {
      return request(
        () => platformApi.delete(
          bitableCollaborationEndpoints.presence(input.room.resourceId),
          { ...cleanupTransport, headers: headersFor(input.connectionId) },
        ),
        () => undefined,
      )
    },
    saveCell,
    subscribe(input) {
      headersFor(input.connectionId)
      const controller = new AbortController()
      void openEventStream({ ...input, signal: controller.signal })
      return () => controller.abort()
    },
  }
}
