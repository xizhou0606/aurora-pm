import { describe, expect, it, vi } from 'bun:test'
import { consumeCollaborationSse } from './collaboration-sse'

const room = { workspaceId: 2, resourceId: 'resource-1' }
const bytes = (frame: string) => new TextEncoder().encode(frame)

describe('collaboration SSE lifetime', () => {
  it('delivers a namespaced policy event and the subsequent record and presence events', async () => {
    const onSignal = vi.fn()
    const dtoRoom = { workspace_id: 2, resource_id: 'resource-1' }
    const event = (sequence: number, operation_id: string, type: string) => ({
      room: dtoRoom, sequence, operation_id, type, actor_user_id: 110, payload: {}, policy_revision: 2, committed_at: '2026-09-08T15:00:00Z',
    })
    const frames = [
      `event: committed\ndata: ${JSON.stringify({ event: event(4, `policy:${'a'.repeat(64)}`, 'resource.policy.committed') })}`,
      `event: committed\ndata: ${JSON.stringify({ event: event(5, 'record-operation', 'bitable.records.committed') })}`,
      `event: presence\ndata: ${JSON.stringify({ presence: [{ room: dtoRoom, connection_id: 'presence-123', actor_user_id: 129, actor_display_name: 'Tester', state: { table_id: 'table-1' }, expires_at: '2026-09-08T15:01:00Z' }] })}`,
    ]
    const stream = new ReadableStream<Uint8Array>({ start(controller) {
      controller.enqueue(bytes(`${frames.join('\n\n')}\n\n`))
      controller.close()
    } })
    await expect(consumeCollaborationSse({ stream, room, onSignal })).resolves.toMatchObject({ cursor: 5 })
    expect(onSignal.mock.calls.map(([signal]) => signal.kind)).toEqual(['committed', 'committed', 'presence'])
    expect(onSignal.mock.calls[2][0].members[0].actorUserId).toBe(129)
  })
  it('accepts the Go envelope with an omitted empty presence list', async () => {
    const onSignal = vi.fn()
    const stream = new ReadableStream<Uint8Array>({ start(controller) {
      controller.enqueue(bytes('event: presence\ndata: {"kind":"presence"}\n\n'))
      controller.close()
    } })
    await consumeCollaborationSse({ stream, room, onSignal })
    expect(onSignal).toHaveBeenCalledWith({ kind: 'presence', room, members: [] })
  })

  it('cancels a live response on parsing failure instead of leaking a connection', async () => {
    const cancel = vi.fn()
    const stream = new ReadableStream<Uint8Array>({ start(controller) {
      controller.enqueue(bytes('event: presence\ndata: invalid json\n\n'))
    }, cancel })
    await expect(consumeCollaborationSse({ stream, room, onSignal: vi.fn() })).rejects.toThrow()
    expect(cancel).toHaveBeenCalledTimes(1)
    expect(stream.locked).toBe(false)
  })

  it('closes immediately when access is revoked even if the server leaves the stream open', async () => {
    const cancel = vi.fn(), onSignal = vi.fn()
    const stream = new ReadableStream<Uint8Array>({ start(controller) {
      controller.enqueue(bytes('event: access_revoked\ndata: {}\n\n'))
    }, cancel })
    await expect(consumeCollaborationSse({ stream, room, onSignal })).resolves.toMatchObject({ revoked: true })
    expect(cancel).toHaveBeenCalledTimes(1)
    expect(onSignal).toHaveBeenCalledWith({ kind: 'access_revoked', room })
  })
})
