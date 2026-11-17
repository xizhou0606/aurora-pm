import { HocuspocusProvider, type HocuspocusProviderConfiguration } from '@hocuspocus/provider'
import Collaboration from '@tiptap/extension-collaboration'
import * as Y from 'yjs'
import { collaborationCursor } from './collaboration-cursor'
import { collaborationBlockLocks } from './collaboration-block-locks'

import type {
  DocumentCollaborationEvent,
  DocumentCollaborationMember,
  DocumentCollaborationSessionFactory,
} from './collaboration-port'

const ROOM_ID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/

function base64Url(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  bytes.forEach((byte) => { binary += String.fromCharCode(byte) })
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function roomName(workspaceId: number, resourceId: string): string {
  const workspace = String(workspaceId)
  if (!ROOM_ID.test(workspace) || !ROOM_ID.test(resourceId)) throw new Error('invalid_collaboration_room')
  return `v1.${base64Url(workspace)}.${base64Url(resourceId)}`
}

function bytesFromBase64(value: string): Uint8Array | undefined {
  if (!value || value.length > 16_384 || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)) {
    return undefined
  }
  try {
    const binary = atob(value)
    const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0))
    return btoa(binary) === value ? bytes : undefined
  } catch {
    return undefined
  }
}

function coversLocalState(document: Y.Doc, stateVectorBase64: string): boolean {
  const bytes = bytesFromBase64(stateVectorBase64)
  if (!bytes) return false
  try {
    const durable = Y.decodeStateVector(bytes)
    const local = Y.decodeStateVector(Y.encodeStateVector(document))
    for (const [clientId, clock] of local) {
      if ((durable.get(clientId) ?? 0) < clock) return false
    }
    return true
  } catch {
    return false
  }
}

function statelessEvent(
  payload: string,
  expectedRoom: string,
  document: Y.Doc,
): DocumentCollaborationEvent | undefined {
  let value: unknown
  try {
    value = JSON.parse(payload)
  } catch {
    return undefined
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined
  const source = value as Record<string, unknown>
  if (source.type === 'edit_conflict') return { kind: 'edit_conflict' }
  if (source.type === 'access_revoked') return { kind: 'access_revoked' }
  // A failed durable commit invalidates the whole in-memory room on the Node
  // runtime. Treat it as terminal for this Y.Doc so the provider cannot
  // reconnect and re-propagate rejected local state into the reloaded room.
  if (source.type === 'durable_commit_failed') return { kind: 'error' }
  if (source.type === 'access_changed' && (source.level === 'viewer' || source.level === 'editor')) {
    return { kind: 'access_changed', level: source.level }
  }
  if (
    source.type !== 'durable_commit'
    || source.roomName !== expectedRoom
    || !Number.isSafeInteger(source.durableRevision)
    || Number(source.durableRevision) <= 0
    || typeof source.storedAt !== 'string'
    || !Number.isFinite(Date.parse(source.storedAt))
    || typeof source.stateVectorBase64 !== 'string'
    || !coversLocalState(document, source.stateVectorBase64)
  ) return undefined
  return {
    kind: 'durable_commit',
    durableRevision: Number(source.durableRevision),
    storedAt: source.storedAt,
  }
}

function projectedAwarenessMembers(states: readonly Record<string, unknown>[]): readonly DocumentCollaborationMember[] {
  return states.flatMap((state) => {
    if (state.active === false) return []
    if (!Number.isSafeInteger(state.clientId) || Number(state.clientId) < 0) return []
    if (!state.user || typeof state.user !== 'object' || Array.isArray(state.user)) return []
    const user = state.user as Record<string, unknown>
    if (typeof user.id !== 'string' || !user.id || user.id.length > 128) return []
    if (typeof user.displayName !== 'string' || !user.displayName || user.displayName.length > 220) return []
    return [{
      clientId: Number(state.clientId),
      actorId: user.id,
      actorName: user.displayName,
      hasCursor: Boolean((state.cursor && typeof state.cursor === 'object')
        || (state.selection && typeof state.selection === 'object')),
    }]
  })
}

export const productionDocumentCollaborationFactory: DocumentCollaborationSessionFactory = {
  open(input) {
    const document = new Y.Doc()
    const name = roomName(input.workspaceId, input.resourceId)
    let destroyed = false
    let members: readonly DocumentCollaborationMember[] = []
    const emit = (event: DocumentCollaborationEvent) => {
      if (!destroyed) input.onEvent(event)
    }
    const configuration = {
      url: input.websocketUrl,
      name,
      document,
      token: input.ticket,
      sessionAwareness: true,
      onAuthenticated: ({ scope }: { scope: 'read-write' | 'readonly' }) => {
        emit({ kind: 'access_changed', level: scope === 'read-write' ? 'editor' : 'viewer' })
      },
      onSynced: ({ state }: { state: boolean }) => {
        if (state) emit({ kind: 'connected' })
      },
      onStatus: ({ status }: { status: unknown }) => {
        const value = String(status).toLowerCase()
        if (value === 'connecting' || value === 'disconnected') emit({ kind: 'reconnecting' })
      },
      // Actor identity is accepted only from the awareness state after the
      // collaboration authority has projected it. No browser actor fields are set.
      onAwarenessChange: ({ states }: { states: readonly Record<string, unknown>[] }) => {
        const next = [...projectedAwarenessMembers(states)].sort((a, b) => a.clientId - b.clientId)
        if (JSON.stringify(next) === JSON.stringify(members)) return
        members = next
        emit({ kind: 'awareness', members })
      },
      onAuthenticationFailed: () => emit({ kind: 'access_revoked' }),
      onStateless: ({ payload }: { payload: string }) => {
        const event = statelessEvent(payload, name, document)
        if (event?.kind === 'edit_conflict') {
          // Never reconnect a rejected local Y.Doc and replay its conflicting
          // update. Keep the editor snapshot available for copying the draft.
          provider.disconnect()
        }
        if (event) emit(event)
      },
    } as unknown as HocuspocusProviderConfiguration
    const provider = new HocuspocusProvider(configuration)
    // Provider counters include the initial sync handshake, not just edits.
    // Selection/awareness and reconnects must never dirty the document.
    const onLocalUpdate = (_update: Uint8Array, origin: unknown, _doc: Y.Doc, transaction: Y.Transaction) => {
      if (transaction.local && origin !== provider) emit({ kind: 'local_update' })
    }
    document.on('update', onLocalUpdate)
    const publishVisibility = () => {
      const awareness = provider.awareness
      if (!awareness || destroyed) return
      const visible = globalThis.document.visibilityState !== 'hidden' && globalThis.document.hasFocus()
      const current = awareness.getLocalState()
      if (current?.active === visible && (visible || (current.cursor == null && current.selection == null))) return
      awareness.setLocalState({ ...(current ?? {}), active: visible,
        ...(!visible ? { cursor: null, selection: null } : {}) })
    }
    globalThis.document.addEventListener('visibilitychange', publishVisibility)
    globalThis.window.addEventListener('focus', publishVisibility)
    globalThis.window.addEventListener('blur', publishVisibility)
    publishVisibility()
    return {
      adapter: {
        key: `${input.workspaceId}:${input.resourceId}:${input.connectionId}`,
        extensions: [Collaboration.configure({ document }), collaborationCursor(provider.awareness!), collaborationBlockLocks(provider.awareness!)],
      },
      destroy() {
        if (destroyed) return
        destroyed = true
        globalThis.document.removeEventListener('visibilitychange', publishVisibility)
        globalThis.window.removeEventListener('focus', publishVisibility)
        globalThis.window.removeEventListener('blur', publishVisibility)
        document.off('update', onLocalUpdate)
        provider.destroy()
        document.destroy()
      },
    }
  },
}
