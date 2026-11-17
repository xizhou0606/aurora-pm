import { randomUUID } from 'node:crypto'
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import type { JSONContent } from '@tiptap/core'
import * as Y from 'yjs'
import type { AccessLevel, CollaborationRoom } from '../src/contract.js'
import { documentFromProjection, encodeSnapshot, projectAndValidate, projectionHash, sha256 } from '../src/projection.js'

interface GrantRow {
  grantId: string
  ticket: string
  room: CollaborationRoom
  actorId: string
  displayName: string
  level: AccessLevel
  policyRevision: number
  expiresAt: string
  active: boolean
}

interface StoredDocument {
  snapshotBase64: string
  durableRevision: number
  snapshotHash: string
  stateVectorBase64: string
  stateVectorHash: string
  projectionHash: string
  projection: JSONContent
}

interface CommitRow {
  requestFingerprint: string
  response: Record<string, unknown>
}

interface ValidationRow {
  token: string
  grantId: string
  roomKey: string
  snapshotHash: string
  policyRevision: number
  expiresAt: string
}

function key(room: CollaborationRoom): string {
  return `${room.workspaceId}/${room.resourceId}`
}

function wireGrant(grant: GrantRow) {
  return {
    grant_id: grant.grantId,
    actor: { id: grant.actorId, display_name: grant.displayName },
    level: grant.level,
    policy_revision: grant.policyRevision,
    expires_at: grant.expiresAt,
    revalidate_after_ms: 10,
  }
}

function emptyStoredDocument(): StoredDocument {
  const document = documentFromProjection({ type: 'doc', content: [{ type: 'paragraph' }] })
  const snapshot = encodeSnapshot(document)
  const { projection } = projectAndValidate(document)
  const stateVector = Y.encodeStateVector(document)
  return {
    snapshotBase64: Buffer.from(snapshot).toString('base64'),
    durableRevision: 0,
    snapshotHash: sha256(snapshot),
    stateVectorBase64: Buffer.from(stateVector).toString('base64'),
    stateVectorHash: sha256(stateVector),
    projectionHash: projectionHash(projection),
    projection,
  }
}

async function body(request: IncomingMessage): Promise<Record<string, any>> {
  const chunks: Buffer[] = []
  for await (const chunk of request) chunks.push(Buffer.from(chunk))
  return JSON.parse(Buffer.concat(chunks).toString('utf8')) as Record<string, any>
}

function roomFrom(value: Record<string, any>): CollaborationRoom {
  return { workspaceId: value.workspace_id, resourceId: value.resource_id }
}

function send(response: ServerResponse, status: number, value: unknown, code?: string): void {
  response.writeHead(status, {
    'content-type': 'application/json',
    ...(code ? { 'x-pm-error-code': code } : {}),
  })
  response.end(JSON.stringify(value))
}

export class AuthorityDouble {
  readonly serviceToken = 'spike-service-token'
  readonly allowedAssets = new Set<string>()
  readonly allowedBitables = new Set<string>()
  authorizeCalls = 0
  loadCalls = 0
  validateCalls = 0
  commitCalls = 0
  commitDelayMs = 0
  receiptTtlMs = 2_000
  failCommits = false
  readonly committedActors: string[] = []
  readonly committedOperationIds: string[] = []
  readonly committedProjections: JSONContent[] = []
  private server?: Server
  private grants = new Map<string, GrantRow>()
  private tickets = new Map<string, GrantRow>()
  private documents = new Map<string, StoredDocument>()
  private validations = new Map<string, ValidationRow>()
  private commits = new Map<string, CommitRow>()

  get url(): string {
    const address = this.server?.address() as AddressInfo | undefined
    if (!address) throw new Error('authority is not listening')
    return `http://127.0.0.1:${address.port}`
  }

  async listen(): Promise<void> {
    this.server = createServer(this.handle)
    await new Promise<void>((resolve) => this.server!.listen(0, '127.0.0.1', resolve))
  }

  async close(): Promise<void> {
    if (!this.server) return
    await new Promise<void>((resolve, reject) => this.server!.close((error) => error ? reject(error) : resolve()))
  }

  addGrant(room: CollaborationRoom, level: AccessLevel, options: { ticket?: string; expiresInMs?: number } = {}): GrantRow {
    const grant: GrantRow = {
      grantId: randomUUID(),
      ticket: options.ticket ?? randomUUID(),
      room,
      actorId: randomUUID(),
      displayName: level === 'editor' ? 'Spike Editor' : 'Spike Viewer',
      level,
      policyRevision: 1,
      expiresAt: new Date(Date.now() + (options.expiresInMs ?? 60_000)).toISOString(),
      active: true,
    }
    this.grants.set(grant.grantId, grant)
    this.tickets.set(grant.ticket, grant)
    if (!this.documents.has(key(room))) this.documents.set(key(room), emptyStoredDocument())
    return grant
  }

  revoke(grantId: string): void {
    const grant = this.grants.get(grantId)
    if (!grant) return
    for (const candidate of this.grants.values()) {
      if (candidate.actorId === grant.actorId && key(candidate.room) === key(grant.room)) candidate.active = false
    }
  }

  seed(room: CollaborationRoom, document: Y.Doc): void {
    const snapshot = encodeSnapshot(document)
    const { projection } = projectAndValidate(document)
    const stateVector = Y.encodeStateVector(document)
    this.documents.set(key(room), {
      snapshotBase64: Buffer.from(snapshot).toString('base64'),
      durableRevision: 1,
      snapshotHash: sha256(snapshot),
      stateVectorBase64: Buffer.from(stateVector).toString('base64'),
      stateVectorHash: sha256(stateVector),
      projectionHash: projectionHash(projection),
      projection,
    })
  }

  stored(room: CollaborationRoom): StoredDocument {
    const value = this.documents.get(key(room))
    if (!value) throw new Error('missing stored document')
    return value
  }

  private handle = async (request: IncomingMessage, response: ServerResponse) => {
    if (
      request.headers.authorization !== `Bearer ${this.serviceToken}` ||
      request.headers['x-pm-collaboration-contract'] !== '1'
    ) {
      send(response, 401, {}, 'invalid-service-identity')
      return
    }
    const path = new URL(request.url ?? '/', 'http://authority.invalid').pathname
    if (path === '/internal/collaboration/v1/health') {
      send(response, 204, {})
      return
    }
    try {
      const input = await body(request)
      if (path === '/internal/collaboration/v1/authorize') return this.authorize(input, response)
      if (path === '/internal/collaboration/v1/revalidate') return this.revalidate(input, response)
      if (path === '/internal/collaboration/v1/load') return this.load(input, response)
      if (path === '/internal/collaboration/v1/validate-mutation') return this.validate(input, response)
      if (path === '/internal/collaboration/v1/commit') return await this.commit(input, response)
      send(response, 404, {}, 'not-found')
    } catch {
      send(response, 400, {}, 'invalid-json')
    }
  }

  private currentGrant(input: Record<string, any>, response: ServerResponse): GrantRow | undefined {
    const room = roomFrom(input.room)
    const supplied = input.grant
    const grant = this.grants.get(supplied.grant_id)
    if (
      !grant || !grant.active || key(grant.room) !== key(room) ||
      grant.actorId !== supplied.actor_id || grant.policyRevision !== supplied.policy_revision ||
      Date.parse(grant.expiresAt) <= Date.now()
    ) {
      send(response, 403, {}, 'access-revoked')
      return undefined
    }
    return grant
  }

  private authorize(input: Record<string, any>, response: ServerResponse): void {
    this.authorizeCalls += 1
    const room = roomFrom(input.room)
    const grant = this.tickets.get(input.connection_ticket)
    if (!grant || !grant.active || key(grant.room) !== key(room) || Date.parse(grant.expiresAt) <= Date.now()) {
      send(response, 404, {}, 'resource-unavailable')
      return
    }
    send(response, 200, { grant: wireGrant(grant) })
  }

  private revalidate(input: Record<string, any>, response: ServerResponse): void {
    const grant = this.currentGrant(input, response)
    if (!grant) return
    const rotated: GrantRow = { ...grant, grantId: randomUUID() }
    this.grants.set(rotated.grantId, rotated)
    send(response, 200, { grant: wireGrant(rotated) })
  }

  private load(input: Record<string, any>, response: ServerResponse): void {
    this.loadCalls += 1
    const grant = this.currentGrant(input, response)
    if (!grant) return
    const stored = this.stored(roomFrom(input.room))
    send(response, 200, {
      snapshot_base64: stored.snapshotBase64,
      durable_revision: stored.durableRevision,
      snapshot_hash: stored.snapshotHash,
      state_vector_base64: stored.stateVectorBase64,
      state_vector_hash: stored.stateVectorHash,
      projection_hash: stored.projectionHash,
    })
  }

  private validate(input: Record<string, any>, response: ServerResponse): void {
    this.validateCalls += 1
    const grant = this.currentGrant(input, response)
    if (!grant) return
    if (grant.level !== 'editor') {
      send(response, 403, {}, 'viewer-cannot-edit')
      return
    }
    const assetIds = input.references?.asset_ids
    const bitableIds = input.references?.bitable_resource_ids
    if (
      !Array.isArray(assetIds) || assetIds.some((id) => !this.allowedAssets.has(id)) ||
      !Array.isArray(bitableIds) || bitableIds.some((id) => !this.allowedBitables.has(id))
    ) {
      send(response, 422, {}, 'reference-not-allowed')
      return
    }
    const validation: ValidationRow = {
      token: randomUUID(),
      grantId: grant.grantId,
      roomKey: key(grant.room),
      snapshotHash: input.candidate_snapshot_hash,
      policyRevision: grant.policyRevision,
      expiresAt: new Date(Date.now() + this.receiptTtlMs).toISOString(),
    }
    this.validations.set(validation.token, validation)
    send(response, 200, {
      validation_token: validation.token,
      candidate_snapshot_hash: validation.snapshotHash,
      policy_revision: validation.policyRevision,
      expires_at: validation.expiresAt,
    })
  }

  private async commit(input: Record<string, any>, response: ServerResponse): Promise<void> {
    this.commitCalls += 1
    if (this.commitDelayMs) await new Promise((resolve) => setTimeout(resolve, this.commitDelayMs))
    if (this.failCommits) {
      send(response, 503, {}, 'synthetic-storage-failure')
      return
    }
    const grant = this.currentGrant(input, response)
    if (!grant || grant.level !== 'editor') return
    const requestFingerprint = sha256(JSON.stringify({
      expected: input.expected_durable_revision,
      base: input.base_state_vector_hash,
      update: input.update_hash,
      snapshot: input.candidate_snapshot_hash,
      stateVector: input.state_vector_hash,
      projection: projectionHash(input.projection),
    }))
    const operationKey = `${key(grant.room)}/${input.operation_id}`
    const previousCommit = this.commits.get(operationKey)
    if (previousCommit) {
      if (previousCommit.requestFingerprint !== requestFingerprint) {
        send(response, 409, {}, 'operation-conflict')
      } else {
        send(response, 200, previousCommit.response)
      }
      return
    }
    const validation = this.validations.get(input.validation_token)
    if (
      !validation || validation.grantId !== grant.grantId || validation.roomKey !== key(grant.room) ||
      validation.snapshotHash !== input.candidate_snapshot_hash || validation.policyRevision !== grant.policyRevision ||
      Date.parse(validation.expiresAt) <= Date.now()
    ) {
      send(response, 409, {}, 'validation-receipt-invalid')
      return
    }
    const current = this.stored(grant.room)
    if (
      current.durableRevision !== input.expected_durable_revision ||
      current.stateVectorHash !== input.base_state_vector_hash
    ) {
      send(response, 409, {}, 'durable-revision-conflict')
      return
    }
    const snapshot = Buffer.from(input.snapshot_base64, 'base64')
    const update = Buffer.from(input.update_base64, 'base64')
    const stateVector = Buffer.from(input.state_vector_base64, 'base64')
    const document = new Y.Doc()
    Y.applyUpdate(document, snapshot)
    const { projection, references } = projectAndValidate(document)
    const reconstructed = new Y.Doc()
    Y.applyUpdate(reconstructed, Buffer.from(current.snapshotBase64, 'base64'))
    Y.applyUpdate(reconstructed, update)
    const reconstructedSnapshot = Y.encodeStateAsUpdate(reconstructed)
    if (
      sha256(snapshot) !== input.candidate_snapshot_hash ||
      sha256(update) !== input.update_hash || sha256(stateVector) !== input.state_vector_hash ||
      sha256(Y.encodeStateVector(document)) !== input.state_vector_hash ||
      sha256(reconstructedSnapshot) !== input.candidate_snapshot_hash ||
      projectionHash(projection) !== projectionHash(input.projection) ||
      references.assetIds.some((id) => !this.allowedAssets.has(id)) ||
      references.bitableResourceIds.some((id) => !this.allowedBitables.has(id))
    ) {
      send(response, 422, {}, 'commit-attestation-invalid')
      return
    }
    const stored = {
      snapshotBase64: input.snapshot_base64,
      durableRevision: current.durableRevision + 1,
      snapshotHash: sha256(snapshot),
      stateVectorBase64: input.state_vector_base64,
      stateVectorHash: input.state_vector_hash,
      projectionHash: projectionHash(projection),
      projection,
    }
    this.documents.set(key(grant.room), stored)
    this.validations.delete(validation.token)
    this.committedActors.push(grant.actorId)
    this.committedOperationIds.push(input.operation_id)
    this.committedProjections.push(projection)
    const commitResponse = {
      commit_id: randomUUID(),
      durable_revision: stored.durableRevision,
      snapshot_hash: stored.snapshotHash,
      state_vector_hash: stored.stateVectorHash,
      projection_hash: stored.projectionHash,
      stored_at: new Date().toISOString(),
    }
    this.commits.set(operationKey, { requestFingerprint, response: commitResponse })
    send(response, 200, commitResponse)
  }
}
