import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'

import type { BitableCellLeaseProof } from '../model/collaboration'
import {
  bitableCollaborationEndpoints,
  createBitableCollaborationClient,
  createBitableCollaborationConnectionId,
} from './collaboration-client'

const originalAdapter = platformApi.defaults.adapter
let testInterceptor: number | undefined
const connectionId = 'a1111111-1111-4111-8111-111111111111'
const differentConnectionId = '22222222-2222-4222-8222-222222222222'
const key = {
  workspaceId: 7,
  resourceId: 'unified-resource',
  tableId: 'table-1',
  recordId: 'record-1',
  fieldId: 'field-1',
}
const leaseDTO = {
  key: {
    room: { workspace_id: 7, resource_id: 'unified-resource' },
    table_id: 'table-1',
    record_id: 'record-1',
    field_id: 'field-1',
  },
  lease_id: 'lease-1',
  fencing_token: 4,
  connection_id: connectionId,
  editor_instance_id: 'editor-1',
  actor_user_id: 9,
  actor_display_name: 'Editor',
  policy_revision: 8,
  expires_at: '2026-09-08T12:00:00Z',
}

afterEach(() => {
  if (testInterceptor !== undefined) platformApi.interceptors.request.eject(testInterceptor)
  testInterceptor = undefined
  vi.useRealTimers()
  platformApi.defaults.adapter = originalAdapter
  vi.restoreAllMocks()
})

describe('bitable collaboration HTTP adapter', () => {
  it('closes a malformed response and reconnects after the last already delivered commit', async () => {
    vi.useFakeTimers()
    const cancel = vi.fn()
    const event = { event: { room: { workspace_id: 7, resource_id: 'unified-resource' }, sequence: 11,
      operation_id: 'operation-11', actor_user_id: 12, type: 'bitable.cell.updated', payload: {}, policy_revision: 8, committed_at: 'now' } }
    const first = new ReadableStream<Uint8Array>({ start(controller) {
      controller.enqueue(new TextEncoder().encode(`event: committed\ndata: ${JSON.stringify(event)}\n\nevent: presence\ndata: broken\n\n`))
    }, cancel })
    const second = new ReadableStream<Uint8Array>({ start(controller) { controller.close() } })
    const get = vi.spyOn(platformApi, 'get').mockResolvedValueOnce({ data: first }).mockResolvedValue({ data: second })
    let reportError!: () => void
    const streamFailed = new Promise<void>(resolve => { reportError = resolve })
    const close = createBitableCollaborationClient(connectionId, vi.fn()).subscribe({
      room: { workspaceId: 7, resourceId: 'unified-resource' }, connectionId,
      onSignal: signal => { if (signal.kind === 'error') reportError() },
    })
    try {
      // The reader and its cancellation settle in microtasks before retry is scheduled.
      await streamFailed
      expect(get).toHaveBeenCalledTimes(1)
      vi.advanceTimersByTime(1_000)
      await Promise.resolve()
      expect(cancel).toHaveBeenCalledTimes(1)
      expect(get).toHaveBeenCalledTimes(2)
      expect(get).toHaveBeenLastCalledWith(expect.any(String), expect.objectContaining({ headers: expect.objectContaining({ 'Last-Event-ID': '11' }) }))
    } finally { close() }
  })
  it('creates a high-entropy connection id for one bound client lifecycle', () => {
    expect(createBitableCollaborationConnectionId()).toMatch(/^[0-9a-f-]{36}$/)
    expect(() => createBitableCollaborationClient(connectionId.toUpperCase(), vi.fn()))
      .toThrow('canonical lowercase UUID')
  })

  it('uses the frozen resource paths and snake-case lease DTOs', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    let keepaliveRequests = 0
    testInterceptor = platformApi.interceptors.request.use(config => {
      if (config.adapter === 'fetch') {
        expect(config.fetchOptions).toMatchObject({ keepalive: true })
        keepaliveRequests += 1
        config.adapter = platformApi.defaults.adapter
      }
      return config
    })
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      const data = config.url?.endsWith('/acquire')
        ? { lease: leaseDTO }
        : config.url?.endsWith('/renew')
          ? { lease: leaseDTO }
          : config.url?.endsWith('/presence/heartbeat')
            ? { presence: {
                connection_id: 'public-connection', actor_user_id: 9,
                actor_display_name: 'Editor', state: {
                  table_id: 'table-1', view_id: 'view-1',
                  active_cell: { record_id: 'record-1', field_id: 'field-1' },
                }, expires_at: '2026-09-08T12:00:00Z',
              } }
          : {}
      return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const client = createBitableCollaborationClient(connectionId, vi.fn())
    const acquired = await client.acquireCell({
      key, connectionId, editorInstanceId: 'editor-1',
    })
    if (acquired.status !== 'acquired') throw new Error('expected lease')
    const proof: BitableCellLeaseProof = {
      key, leaseId: 'lease-1', fencingToken: 4,
      connectionId, editorInstanceId: 'editor-1',
    }
    await client.renewCell(proof)
    await client.releaseCell(proof)
    await client.disconnect({
      room: { workspaceId: 7, resourceId: 'unified-resource' },
      connectionId,
    })
    await client.heartbeatPresence({
      room: { workspaceId: 7, resourceId: 'unified-resource' },
      connectionId,
      state: {
        tableId: 'table-1', viewId: 'view-1', activeCell: { recordId: 'record-1', fieldId: 'field-1' },
      },
    })
    await client.leavePresence({
      room: { workspaceId: 7, resourceId: 'unified-resource' }, connectionId,
    })

    expect(requests.map(({ url }) => url)).toEqual([
      bitableCollaborationEndpoints.acquire('unified-resource'),
      bitableCollaborationEndpoints.renew('unified-resource'),
      bitableCollaborationEndpoints.release('unified-resource'),
      bitableCollaborationEndpoints.disconnect('unified-resource'),
      bitableCollaborationEndpoints.presenceHeartbeat('unified-resource'),
      bitableCollaborationEndpoints.presence('unified-resource'),
    ])
    expect(JSON.parse(String(requests[0]?.data))).toEqual({
      table_id: 'table-1', record_id: 'record-1', field_id: 'field-1',
      editor_instance_id: 'editor-1',
    })
    expect(JSON.parse(String(requests[1]?.data))).toEqual({
      table_id: 'table-1', record_id: 'record-1', field_id: 'field-1',
      lease_id: 'lease-1', fencing_token: 4, editor_instance_id: 'editor-1',
    })
    expect(requests[3]?.data).toBeUndefined()
    expect(JSON.parse(String(requests[4]?.data))).toEqual({ state: {
      table_id: 'table-1', view_id: 'view-1',
      active_cell: { record_id: 'record-1', field_id: 'field-1' },
    } })
    expect(requests[5]?.method).toBe('delete')
    expect(keepaliveRequests).toBe(3)
    for (const request of requests) {
      expect(request.headers['X-PM-Collaboration-Connection']).toBe(connectionId)
    }
  })

  it('authenticates the SSE request through platformApi and maps committed events', async () => {
    const event = {
      kind: 'committed',
      event: {
        room: { workspace_id: 7, resource_id: 'unified-resource' },
        sequence: 11,
        operation_id: 'operation-1',
        actor_user_id: 12,
        type: 'bitable.cell.updated',
        payload: {},
        policy_revision: 8,
        committed_at: '2026-09-08T12:00:00Z',
      },
    }
    const bytes = new TextEncoder().encode(`event: committed\ndata: ${JSON.stringify(event)}\n\n`)
    const stream = new ReadableStream<Uint8Array>({
      start(controller) { controller.enqueue(bytes); controller.close() },
    })
    const get = vi.spyOn(platformApi, 'get').mockResolvedValue({ data: stream })
    const client = createBitableCollaborationClient(connectionId, vi.fn())
    let close: () => void = () => undefined
    const committed = new Promise<number>((resolve) => {
      close = client.subscribe({
        room: { workspaceId: 7, resourceId: 'unified-resource' },
        connectionId,
        afterSequence: 10,
        onSignal: (signal) => {
          if (signal.kind !== 'committed') return
          resolve(signal.event.sequence)
          close()
        },
      })
    })
    await expect(committed).resolves.toBe(11)
    expect(get).toHaveBeenCalledWith(
      bitableCollaborationEndpoints.events('unified-resource'),
      expect.objectContaining({
        adapter: 'fetch', responseType: 'stream', headers: {
          'Last-Event-ID': '10',
          'X-PM-Collaboration-Connection': connectionId,
        },
      }),
    )
  })

  it('projects a server-owned holder from the acquire conflict response', async () => {
    platformApi.defaults.adapter = async (config) => Promise.reject({
      config,
      response: {
        status: 409,
        data: { error: { code: 'lease_held' }, holder: leaseDTO },
        headers: {},
        config,
      },
    })
    const client = createBitableCollaborationClient(connectionId, vi.fn())
    await expect(client.acquireCell({
      key, connectionId, editorInstanceId: 'editor-2',
    })).resolves.toEqual({
      status: 'held',
      holder: {
        actorUserId: 9,
        actorName: 'Editor',
        expiresAt: '2026-09-08T12:00:00Z',
      },
    })
  })

  it('rejects connection drift before issuing lease or stream traffic', () => {
    const post = vi.spyOn(platformApi, 'post')
    const get = vi.spyOn(platformApi, 'get')
    const client = createBitableCollaborationClient(connectionId, vi.fn())
    expect(() => client.acquireCell({
      key, connectionId: differentConnectionId, editorInstanceId: 'editor-1',
    })).toThrow('connection scope mismatch')
    expect(() => client.subscribe({
      room: { workspaceId: 7, resourceId: 'unified-resource' },
      connectionId: differentConnectionId,
      onSignal: vi.fn(),
    })).toThrow('connection scope mismatch')
    expect(post).not.toHaveBeenCalled()
    expect(get).not.toHaveBeenCalled()
  })
})
