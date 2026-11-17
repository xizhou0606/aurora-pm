import { Hocuspocus, type Connection, type Document } from '@hocuspocus/server'
import * as Y from 'yjs'
import type {
  AccessGrant,
  AuthorityBridge,
  ConnectionContext,
  DurableAcknowledgement,
  ReferenceSet,
  ValidationReceipt,
} from './contract.js'
import type { JSONContent } from '@tiptap/core'
import { deny, rejectPayload, unavailable } from './errors.js'
import {
  projectAndValidate,
  projectionHash,
  sha256,
  validateCandidate,
} from './projection.js'
import { decodeRoomName } from './room.js'
import { BlockLocks } from './block-locks.js'
import { ContentReplacement } from './content-replacement.js'
import { BunTransport } from './bun-transport.js'

const SYNC_STEP_TWO = 1
const SYNC_UPDATE = 2

export interface CollaborationRuntimeOptions {
  authority: AuthorityBridge
  serviceToken?: string
  address?: string
  port?: number
  maxConnections?: number
  maxDocuments?: number
  maxPayloadBytes?: number
  revalidateCeilingMs?: number
}

interface DurableRoomState {
	revision: number
	stateVector: Uint8Array
	stateVectorHash: string
}

function isExpired(grant: AccessGrant): boolean {
  return Date.parse(grant.expiresAt) <= Date.now()
}

function assertSamePrincipal(previous: AccessGrant, next: AccessGrant): void {
  // Go intentionally rotates the opaque grant on every revalidation. The
  // canonical room is supplied separately and the actor identity must remain
  // stable; requiring the bearer value itself to stay stable rejects every
  // healthy production refresh.
  if (previous.actor.id !== next.actor.id) throw deny('grant-principal-changed')
  if (isExpired(next)) throw deny('grant-expired')
}

interface PendingMutation {
  release: () => void
  receipt: ValidationReceipt
  grant: AccessGrant
  update: Uint8Array
  snapshot: Uint8Array
  snapshotHash: string
  stateVector: Uint8Array
  projection: JSONContent
  references: ReferenceSet
}

function responseJson(status: number, value: unknown): Response {
  return Response.json(value, {
    status,
    headers: {
      'cache-control': 'no-store',
    },
  })
}

export class CollaborationRuntime {
  readonly server: { hocuspocus: Hocuspocus<ConnectionContext> }
  private readonly hocuspocus: Hocuspocus<ConnectionContext>
  private readonly transport: BunTransport
  private readonly authority: AuthorityBridge
  private readonly maxConnections: number
  private readonly maxDocuments: number
  private readonly revalidateCeilingMs: number
  private readonly timers = new Map<Connection<ConnectionContext>, NodeJS.Timeout>()
  private readonly durableRooms = new Map<string, DurableRoomState>()
  private readonly pendingMutations = new Map<string, PendingMutation[]>()
  private readonly mutationTails = new Map<string, Promise<void>>()
  private readonly mutationReleases = new Map<Connection<ConnectionContext>, () => void>()
  private readonly quarantinedRooms = new Set<string>()
  private readonly quarantineTimers = new Map<string, NodeJS.Timeout>()
  private readonly blockLocks = new Map<string, BlockLocks>()

  private readonly joiningRooms = new Map<string, number>()
  private readonly replacement: ContentReplacement

  constructor(options: CollaborationRuntimeOptions) {
    this.replacement = new ContentReplacement(options.authority, name => this.server.hocuspocus.documents.has(name) || (this.joiningRooms.get(name) ?? 0) > 0, options.serviceToken)
    this.authority = options.authority
    this.maxConnections = options.maxConnections ?? 200
    this.maxDocuments = options.maxDocuments ?? 100
    this.revalidateCeilingMs = options.revalidateCeilingMs ?? 5_000

    this.hocuspocus = new Hocuspocus<ConnectionContext>({
      name: 'pm-collaboration-p15-spike',
      quiet: true,
      timeout: 30_000,
      // Persistence is intentionally one callback per accepted inbound Yjs
      // update. Enabling debounce would collapse actor/operation attribution.
      debounce: 0,
      maxDebounce: 0,
      unloadImmediately: true,
      flushDelay: 0,
      flushMaxBytes: options.maxPayloadBytes ?? 256 * 1024,
      maxUnauthenticatedQueueSize: 64 * 1024,
      maxUnauthenticatedQueueMessages: 16,
      maxPendingDocuments: 2,

      onConnect: async ({ instance }) => {
        if (instance.getConnectionsCount() >= this.maxConnections) throw unavailable('connection-capacity-exceeded')
        if (instance.getDocumentsCount() >= this.maxDocuments) throw unavailable('document-capacity-exceeded')
      },

      onAuthenticate: async ({ documentName, token, socketId, connectionConfig }) => {
        if (this.quarantinedRooms.has(documentName)) throw unavailable('room-reloading-durable-state')
        if (!token) throw deny('missing-connection-ticket')
        const room = decodeRoomName(documentName)
        if (this.replacement.active.has(documentName)) throw unavailable('document-replacement-in-progress')
        this.joiningRooms.set(documentName, (this.joiningRooms.get(documentName) ?? 0) + 1)
        let grant: AccessGrant
        try { grant = await this.authority.authorize(room, token, socketId) }
        finally { this.joiningRooms.set(documentName, (this.joiningRooms.get(documentName) ?? 1) - 1) }
        if (this.replacement.active.has(documentName)) throw unavailable('document-replacement-in-progress')
        if (isExpired(grant)) throw deny('grant-expired')
        connectionConfig.readOnly = grant.level === 'viewer'
        return { room, grant }
      },

      connected: async ({ connection }) => {
        this.scheduleRevalidation(connection)
      },

      onLoadDocument: async ({ documentName, context }) => {
        if (this.quarantinedRooms.has(documentName)) throw unavailable('room-reloading-durable-state')
        if (this.replacement.active.has(documentName)) throw unavailable('document-replacement-in-progress')
        this.joiningRooms.set(documentName, (this.joiningRooms.get(documentName) ?? 0) + 1)
        let loaded
        try { loaded = await this.authority.load(context.room, context.grant) }
        finally { this.joiningRooms.set(documentName, (this.joiningRooms.get(documentName) ?? 1) - 1) }
        if (this.replacement.active.has(documentName)) throw unavailable('document-replacement-in-progress')
        const snapshot = Buffer.from(loaded.snapshotBase64, 'base64')
        if (sha256(snapshot) !== loaded.snapshotHash) throw rejectPayload('snapshot-hash-mismatch')
		const document = new Y.Doc()
        try {
          Y.applyUpdate(document, snapshot)
        } catch {
          throw rejectPayload('invalid-authority-snapshot')
        }
		const { projection } = projectAndValidate(document)
		if (projectionHash(projection) !== loaded.projectionHash) throw rejectPayload('projection-hash-mismatch')
		const stateVector = Buffer.from(loaded.stateVectorBase64, 'base64')
		if (sha256(stateVector) !== loaded.stateVectorHash || sha256(Y.encodeStateVector(document)) !== loaded.stateVectorHash) {
			throw rejectPayload('state-vector-hash-mismatch')
		}
		this.durableRooms.set(documentName, {
			revision: loaded.durableRevision,
			stateVector,
			stateVectorHash: loaded.stateVectorHash,
		})
        return document
      },

      beforeSync: async ({ connection, document, documentName, type, payload }) => {
        if (type !== SYNC_STEP_TWO && type !== SYNC_UPDATE) {
          await this.refreshGrant(connection)
          return
        }

        const release = await this.acquireMutation(documentName)
        this.mutationReleases.set(connection, release)
        try {
          if (this.quarantinedRooms.has(documentName)) throw unavailable('room-reloading-durable-state')
          // Refresh after waiting: grants and receipts have short authority TTLs.
          const grant = await this.refreshGrant(connection)
          if (grant.level !== 'editor') return
          const candidate = validateCandidate(document, payload)
          try {
            this.blockLocks.get(documentName)?.assertMutation(connection.context, document, candidate.document)
          } catch (error) {
            connection.sendStateless(JSON.stringify({ type: 'edit_conflict' }))
            throw error
          }
          const candidateStateVector = Y.encodeStateVector(candidate.document)
          // Deletions change the delete set, not the state vector. Treating
          // equal vectors as a no-op silently skips their durable receipt.
          if (candidate.snapshotHash === sha256(Y.encodeStateAsUpdate(document))) {
            this.mutationReleases.delete(connection)
            release()
            return
          }
          const receipt = await this.authority.validateMutation({
            room: connection.context.room,
            grant,
            candidateSnapshotHash: candidate.snapshotHash,
            projection: candidate.projection,
            references: candidate.references,
          })
          if (
            receipt.candidateSnapshotHash !== candidate.snapshotHash ||
            receipt.policyRevision !== grant.policyRevision ||
            Date.parse(receipt.expiresAt) <= Date.now()
          ) {
            throw deny('invalid-validation-receipt')
          }
          if (this.quarantinedRooms.has(documentName)) throw unavailable('room-reloading-durable-state')
          const queue = this.pendingMutations.get(documentName) ?? []
          queue.push({
            release,
            receipt,
            grant,
            update: Uint8Array.from(payload),
            snapshot: candidate.snapshot,
            snapshotHash: candidate.snapshotHash,
            stateVector: candidateStateVector,
            projection: candidate.projection,
            references: candidate.references,
          })
          this.pendingMutations.set(documentName, queue)
          // Persistence owns the lock now. Applying/broadcasting an update is
          // not a durable commit; do not prevalidate a backlog of later edits.
          this.mutationReleases.delete(connection)
        } catch (error) {
          this.mutationReleases.delete(connection)
          release()
          throw error
        }
      },

      afterHandleMessage: async ({ connection }) => {
        const release = this.mutationReleases.get(connection)
        if (!release) return
        this.mutationReleases.delete(connection)
        release()
      },

      beforeHandleAwareness: async ({ connection, states, document, documentName }) => {
        if (!connection) throw deny()
        const grant = await this.refreshGrant(connection)
        const locks = this.blockLocks.get(documentName) ?? new BlockLocks()
        this.blockLocks.set(documentName, locks)
        for (const [clientId, state] of states) {
          // Hocuspocus decodes into a scratch Awareness with an empty local
          // state. It is not a connected participant and must not be projected.
          if (!state || Object.keys(state).length === 0) { states.delete(clientId); continue }
          const selection = state.selection
          const cursor = state.cursor
          const editingBlock = locks.claim(connection.context, document,
            state.active !== false && grant.level === 'editor' ? cursor : null)
          states.set(clientId, {
            user: { id: grant.actor.id, displayName: grant.actor.displayName },
            active: state.active !== false,
            ...(selection !== undefined ? { selection } : {}),
            ...(cursor !== undefined ? { cursor } : {}),
            ...(editingBlock ? { editingBlock } : {}),
          })
        }
      },

      onStoreDocument: async ({ document, documentName, instance }) => {
        try {
          await this.store(document, documentName)
        } catch (error) {
          this.quarantineRoom(instance, document, documentName)
          throw error
        }
      },

      onDisconnect: async ({ documentName, context }) => {
        this.blockLocks.get(documentName)?.release(context)
        this.clearDetachedTimers(documentName)
      },

      afterUnloadDocument: async ({ documentName }) => {
        this.blockLocks.delete(documentName)
        this.durableRooms.delete(documentName)
        this.pendingMutations.delete(documentName)
        this.mutationTails.delete(documentName)
        this.releaseQuarantine(documentName)
      },
    })

    // The hocuspocus `Server` wrapper is Node-only (crossws node adapter);
    // under Bun the official integration wires the `Hocuspocus` class into
    // `Bun.serve` via crossws' Bun adapter (see bun-transport.ts).
    this.server = { hocuspocus: this.hocuspocus }
    this.transport = new BunTransport({
      hocuspocus: this.hocuspocus,
      hostname: options.address ?? '127.0.0.1',
      port: options.port ?? 0,
      maxPayloadSize: options.maxPayloadBytes ?? 256 * 1024,
      handleHttpRequest: (request) => this.handleHttpRequest(request),
    })
    this.transport.serve()
  }

  async listen(): Promise<void> {
    await this.hocuspocus.hooks('onListen', {
      instance: this.hocuspocus,
      configuration: this.hocuspocus.configuration,
      port: this.transport.port,
    })
  }

  get webSocketUrl(): string {
    return this.transport.webSocketUrl
  }

  get httpUrl(): string {
    return this.transport.httpUrl
  }

  async destroy(): Promise<void> {
    for (const timer of this.timers.values()) clearTimeout(timer)
    this.timers.clear()
    for (const release of this.mutationReleases.values()) release()
    this.mutationReleases.clear()
    this.mutationTails.clear()
    const sockets = new Set<Connection<ConnectionContext>['webSocket']>()
    for (const document of this.server.hocuspocus.documents.values()) {
      for (const connection of document.getConnections()) sockets.add(connection.webSocket)
    }
    for (const socket of sockets) socket.close(1001, 'server-shutdown')
    await new Promise<void>((resolve) => setImmediate(resolve))
    this.destroyPromise ??= this.runDestroy()
    await this.destroyPromise
    for (const timer of this.quarantineTimers.values()) clearTimeout(timer)
    this.quarantineTimers.clear()
    this.quarantinedRooms.clear()
  }

  private destroyPromise?: Promise<void>

  // Mirrors @hocuspocus/server's own `Server.runDestroy` shutdown contract:
  // stop accepting connections, close the remaining ones, and resolve once
  // every document has been stored and unloaded.
  private runDestroy(): Promise<void> {
    return new Promise<void>((resolve) => {
      this.transport.stop()
      try {
        this.hocuspocus.configuration.extensions.push({
          async afterUnloadDocument({ instance }) {
            if (instance.getDocumentsCount() === 0) resolve()
          },
        })
        if (this.hocuspocus.getDocumentsCount() === 0) resolve()
        this.hocuspocus.closeConnections()
        this.hocuspocus.flushPendingStores()
      } catch (error) {
        console.error(error)
      }
    }).then(async () => {
      await this.hocuspocus.hooks('onDestroy', { instance: this.hocuspocus })
    })
  }

  private async handleHttpRequest(request: Request): Promise<Response> {
    const path = new URL(request.url).pathname
    if (path === '/internal/content-replacement' && request.method === 'POST') {
      return this.replacement.handle(request)
    }
    if (path === '/healthz') {
      return responseJson(200, { status: 'ok' })
    }
    if (path === '/readyz') {
      try {
        await this.authority.health(AbortSignal.timeout(1_000))
        return responseJson(200, { status: 'ready' })
      } catch {
        return responseJson(503, { status: 'not_ready' })
      }
    }
    return responseJson(404, { error: 'not_found' })
  }

  private async refreshGrant(connection: Connection<ConnectionContext>): Promise<AccessGrant> {
    let refreshed: AccessGrant
    try {
      refreshed = await this.authority.revalidate(connection.context.room, connection.context.grant)
      assertSamePrincipal(connection.context.grant, refreshed)
    } catch (error) {
      connection.sendStateless(JSON.stringify({ type: 'access_revoked' }))
      connection.close({ code: 4403, reason: 'access-revoked' })
      throw error
    }
    connection.context.grant = refreshed
    if (refreshed.level === 'viewer' && !connection.readOnly) {
      connection.readOnly = true
      connection.sendStateless(JSON.stringify({ type: 'access_changed', level: 'viewer' }))
    }
    return refreshed
  }

  private scheduleRevalidation(connection: Connection<ConnectionContext>): void {
    const previous = this.timers.get(connection)
    if (previous) clearTimeout(previous)
    const untilExpiry = Math.max(1, Date.parse(connection.context.grant.expiresAt) - Date.now())
    const delay = Math.max(1, Math.min(connection.context.grant.revalidateAfterMs, this.revalidateCeilingMs, untilExpiry))
    const timer = setTimeout(async () => {
      try {
        await this.refreshGrant(connection)
        if (connection.document.hasConnection(connection)) this.scheduleRevalidation(connection)
      } catch {
        this.timers.delete(connection)
      }
    }, delay)
    this.timers.set(connection, timer)
  }

  private clearDetachedTimers(documentName: string): void {
    for (const [connection, timer] of this.timers) {
      if (connection.document.name === documentName && !connection.document.hasConnection(connection)) {
        clearTimeout(timer)
        this.timers.delete(connection)
      }
    }
  }

  private async store(document: Document, documentName: string): Promise<void> {
    if (this.quarantinedRooms.has(documentName)) throw unavailable('room-reloading-durable-state')
    const queue = this.pendingMutations.get(documentName)
    const pending = queue?.shift()
    if (!pending) throw deny('missing-validation-receipt')
    if (queue?.length === 0) this.pendingMutations.delete(documentName)
    try {
    if (pending.grant.level !== 'editor') throw deny('missing-editor-context')
    const durable = this.durableRooms.get(documentName)
    if (!durable) throw unavailable('missing-durable-room-state')
		const { snapshot, snapshotHash: candidateSnapshotHash, stateVector, update, projection, references } = pending
		const updateHash = sha256(update)
		const operationId = sha256(Buffer.concat([Buffer.from(documentName), Buffer.from(update)]))
		const commit = await this.authority.commit({
      room: decodeRoomName(documentName),
      grant: pending.grant,
			validationToken: pending.receipt.token,
			operationId,
			expectedDurableRevision: durable.revision,
			baseStateVectorHash: durable.stateVectorHash,
			candidateSnapshotHash,
			updateBase64: Buffer.from(update).toString('base64'),
			updateHash,
			snapshotBase64: Buffer.from(snapshot).toString('base64'),
			stateVectorBase64: Buffer.from(stateVector).toString('base64'),
			stateVectorHash: sha256(stateVector),
      projection,
      references,
    })
    const expectedProjectionHash = projectionHash(projection)
		if (
			commit.snapshotHash !== candidateSnapshotHash ||
			commit.stateVectorHash !== sha256(stateVector) ||
			commit.projectionHash !== expectedProjectionHash
		) {
      throw unavailable('commit-attestation-mismatch')
    }
		durable.revision = commit.durableRevision
		durable.stateVector = stateVector
		durable.stateVectorHash = commit.stateVectorHash
    const acknowledgement: DurableAcknowledgement = {
      type: 'durable_commit',
      roomName: documentName,
      stateVectorBase64: Buffer.from(stateVector).toString('base64'),
      ...commit,
    }
    document.broadcastStateless(JSON.stringify(acknowledgement))
    } catch (error) {
      // Close the room before waking another mutation waiting on its lock.
      this.quarantineRoom(this.server.hocuspocus, document, documentName)
      throw error
    } finally {
      pending.release()
    }
  }

  private quarantineRoom(
    instance: Hocuspocus<ConnectionContext>,
    document: Document,
    documentName: string,
  ): void {
    if (this.quarantinedRooms.has(documentName)) return
    this.quarantinedRooms.add(documentName)
    for (const pending of this.pendingMutations.get(documentName) ?? []) pending.release()
    this.pendingMutations.delete(documentName)
    this.durableRooms.delete(documentName)

    for (const connection of document.getConnections()) {
      try {
        connection.sendStateless(JSON.stringify({ type: 'durable_commit_failed' }))
      } finally {
        connection.close({ code: 4410, reason: 'durable-commit-failed' })
      }
    }

    let attempt = 0
    const unload = async (): Promise<void> => {
      this.quarantineTimers.delete(documentName)
      try {
        await instance.unloadDocument(document)
      } catch {
        // Keep the room closed and retry. Reopening stale memory is less safe
        // than temporary unavailability after an authority failure.
      }
      if (instance.documents.get(documentName) !== document) {
        this.releaseQuarantine(documentName)
        return
      }
      attempt += 1
      const timer = setTimeout(() => void unload(), Math.min(1_000, 5 * 2 ** Math.min(attempt, 8)))
      this.quarantineTimers.set(documentName, timer)
    }
    const timer = setTimeout(() => void unload(), 0)
    this.quarantineTimers.set(documentName, timer)
  }

  private releaseQuarantine(documentName: string): void {
    const timer = this.quarantineTimers.get(documentName)
    if (timer) clearTimeout(timer)
    this.quarantineTimers.delete(documentName)
    this.quarantinedRooms.delete(documentName)
  }

  private async acquireMutation(documentName: string): Promise<() => void> {
    const previous = this.mutationTails.get(documentName) ?? Promise.resolve()
    let resolve!: () => void
    const gate = new Promise<void>((done) => { resolve = done })
    const tail = previous.then(() => gate)
    this.mutationTails.set(documentName, tail)
    await previous
    let released = false
    return () => {
      if (released) return
      released = true
      resolve()
      void tail.then(() => {
        if (this.mutationTails.get(documentName) === tail) this.mutationTails.delete(documentName)
      })
    }
  }
}
