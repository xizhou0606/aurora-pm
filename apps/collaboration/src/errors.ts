export class CollaborationError extends Error {
  constructor(
    message: string,
    readonly code: number,
    readonly reason: string,
  ) {
    super(message)
    this.name = 'CollaborationError'
  }
}

export const deny = (reason = 'permission-denied') =>
  new CollaborationError('collaboration request denied', 4403, reason)

export const rejectPayload = (reason = 'invalid-collaboration-payload') =>
  new CollaborationError('collaboration payload rejected', 4400, reason)

export const unavailable = (reason = 'authority-unavailable') =>
  new CollaborationError('collaboration authority unavailable', 1013, reason)
