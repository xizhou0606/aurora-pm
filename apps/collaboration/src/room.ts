import type { CollaborationRoom } from './contract.js'
import { rejectPayload } from './errors.js'

const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/
const ROOM_PREFIX = 'v1'

function assertIdentifier(value: string): void {
  if (!ID_PATTERN.test(value)) {
    throw rejectPayload('invalid-room-identifier')
  }
}

export function encodeRoomName(room: CollaborationRoom): string {
  assertIdentifier(room.workspaceId)
  assertIdentifier(room.resourceId)
  return `${ROOM_PREFIX}.${Buffer.from(room.workspaceId).toString('base64url')}.${Buffer.from(room.resourceId).toString('base64url')}`
}

export function decodeRoomName(name: string): CollaborationRoom {
  const parts = name.split('.')
  if (parts.length !== 3 || parts[0] !== ROOM_PREFIX) {
    throw rejectPayload('invalid-room-name')
  }

  const room = {
    workspaceId: Buffer.from(parts[1] ?? '', 'base64url').toString('utf8'),
    resourceId: Buffer.from(parts[2] ?? '', 'base64url').toString('utf8'),
  }
  assertIdentifier(room.workspaceId)
  assertIdentifier(room.resourceId)
  if (encodeRoomName(room) !== name) {
    throw rejectPayload('non-canonical-room-name')
  }
  return room
}
