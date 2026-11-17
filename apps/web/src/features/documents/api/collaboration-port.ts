import type { RichTextCollaborationAdapter } from '@/shared/ui'

export type DocumentCollaborationTicket = {
  ticket: string
  websocketUrl: string
  expiresAt: string
}

export type DocumentCollaborationMember = {
  clientId: number
  actorId: string
  actorName: string
  hasCursor: boolean
}

export type DocumentCollaborationEvent =
  | { kind: 'connected' }
  | { kind: 'local_update' }
  | { kind: 'reconnecting' }
  | { kind: 'durable_commit'; durableRevision: number; storedAt: string }
  | { kind: 'access_changed'; level: 'viewer' | 'editor' }
  | { kind: 'awareness'; members: readonly DocumentCollaborationMember[] }
  | { kind: 'access_revoked' }
  | { kind: 'error' }
  | { kind: 'edit_conflict' }

export type DocumentCollaborationSession = {
  adapter: RichTextCollaborationAdapter
  destroy: () => void
}

export type DocumentCollaborationSessionFactory = {
  open: (input: {
    workspaceId: number
    resourceId: string
    connectionId: string
    /** Authentication token: memory-only and never appended to the WebSocket URL. */
    ticket: string
    websocketUrl: string
    canEdit: boolean
    onEvent: (event: DocumentCollaborationEvent) => void
  }) => DocumentCollaborationSession
}
