import type { BitableRealtimeSignal } from './collaboration-port'
import type { BitableCollaborationRoom } from '../model/collaboration'
import {
  collaborationEvent,
  collaborationPresence,
  collaborationRoom,
} from './collaboration-contract'
import * as contract from './contract'

type ParsedEvent = { name: string; data: string }

function parseFrame(frame: string): ParsedEvent | undefined {
  let name = 'message'
  const data: string[] = []
  for (const line of frame.split(/\r?\n/)) {
    if (!line || line.startsWith(':')) continue
    const colon = line.indexOf(':')
    const field = colon < 0 ? line : line.slice(0, colon)
    const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '')
    if (field === 'event') name = value
    if (field === 'data') data.push(value)
  }
  return data.length ? { name, data: data.join('\n') } : undefined
}

function signalFor(
  event: ParsedEvent,
  room: BitableCollaborationRoom,
): BitableRealtimeSignal | undefined {
  const payload = contract.object(JSON.parse(event.data))
  if (event.name === 'committed') {
    const committed = collaborationEvent(payload.event)
    if (committed.room.workspaceId !== room.workspaceId
      || committed.room.resourceId !== room.resourceId) return contract.invalid()
    return { kind: 'committed', event: committed }
  }
  if (event.name === 'presence') {
    // Go omits an empty presence slice from the SSE envelope.
    const items = contract.list(payload.presence ?? [], (raw) => {
      const value = contract.object(raw)
      const actual = collaborationRoom(value.room)
      if (actual.workspaceId !== room.workspaceId || actual.resourceId !== room.resourceId) {
        return contract.invalid()
      }
      return collaborationPresence(value)
    })
    return { kind: 'presence', room, members: items }
  }
  if (event.name === 'resync_required') {
    const replay = contract.object(payload.replay)
    return {
      kind: 'reset_required',
      room,
      reason: contract.text(replay.reset_reason),
    }
  }
  if (event.name === 'access_revoked') return undefined
  return undefined
}

export async function consumeCollaborationSse(input: {
  stream: ReadableStream<Uint8Array>
  room: BitableCollaborationRoom
  onSignal: (signal: BitableRealtimeSignal) => void
}): Promise<{ cursor?: number; revoked: boolean; reset: boolean }> {
  const reader = input.stream.getReader()
  const decoder = new TextDecoder()
  let buffer = '', cursor: number | undefined, revoked = false, reset = false
  try {
    while (true) {
      const chunk = await reader.read()
      buffer += decoder.decode(chunk.value, { stream: !chunk.done })
      const frames = buffer.split(/\r?\n\r?\n/)
      buffer = frames.pop() ?? ''
      for (const frame of frames) {
        const event = parseFrame(frame)
        if (!event) continue
        if (event.name === 'access_revoked') {
          revoked = true
          input.onSignal({ kind: 'access_revoked', room: input.room })
          return { cursor, revoked, reset }
        }
        const signal = signalFor(event, input.room)
        if (!signal) continue
        if (signal.kind === 'committed') cursor = signal.event.sequence
        if (signal.kind === 'reset_required') { cursor = undefined; reset = true }
        input.onSignal(signal)
      }
      if (chunk.done) break
    }
  } finally {
    // Releasing the reader lock alone leaves an infinite HTTP response alive.
    // Close it before any reconnect, including malformed frames and revocation.
    await reader.cancel().catch(() => undefined)
    reader.releaseLock()
  }
  return { cursor, revoked, reset }
}
