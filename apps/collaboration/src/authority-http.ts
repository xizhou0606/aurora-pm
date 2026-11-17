import type {
  AccessGrant,
  AuthorityBridge,
  CollaborationRoom,
  CommitDocumentInput,
  DurableCommit,
	LoadedDocument,
  ValidateMutationInput,
  ValidationReceipt,
} from './contract.js'
import { CollaborationError, deny, rejectPayload, unavailable } from './errors.js'

const CONTRACT_VERSION = '1'

type JsonRecord = Record<string, unknown>

function record(value: unknown): JsonRecord {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw unavailable('invalid-authority-response')
  return value as JsonRecord
}

function textField(value: JsonRecord, key: string): string {
  const field = value[key]
  if (typeof field !== 'string' || field.length === 0) throw unavailable('invalid-authority-response')
  return field
}

function integerField(value: JsonRecord, key: string): number {
  const field = value[key]
  if (!Number.isSafeInteger(field) || (field as number) < 0) throw unavailable('invalid-authority-response')
  return field as number
}

function instantField(value: JsonRecord, key: string): string {
  const field = textField(value, key)
  if (!Number.isFinite(Date.parse(field))) throw unavailable('invalid-authority-response')
  return field
}

function grantFromWire(value: unknown): AccessGrant {
  const source = record(value)
  const actor = record(source.actor)
  const level = source.level
  if (level !== 'viewer' && level !== 'editor') throw unavailable('invalid-authority-response')
		return {
    grantId: textField(source, 'grant_id'),
    actor: { id: textField(actor, 'id'), displayName: textField(actor, 'display_name') },
    level,
    policyRevision: integerField(source, 'policy_revision'),
    expiresAt: instantField(source, 'expires_at'),
    revalidateAfterMs: integerField(source, 'revalidate_after_ms'),
  }
}

function roomWire(room: CollaborationRoom) {
  return { workspace_id: room.workspaceId, resource_id: room.resourceId }
}

function grantWire(grant: AccessGrant) {
  return {
    grant_id: grant.grantId,
    actor_id: grant.actor.id,
    level: grant.level,
    policy_revision: grant.policyRevision,
  }
}

export interface HttpAuthorityBridgeOptions {
  baseUrl: string
  serviceToken: string
  timeoutMs?: number
}

export class HttpAuthorityBridge implements AuthorityBridge {
  private readonly baseUrl: URL
  private readonly serviceToken: string
  private readonly timeoutMs: number

  constructor(options: HttpAuthorityBridgeOptions) {
    this.baseUrl = new URL(options.baseUrl)
    if (this.baseUrl.protocol !== 'http:' && this.baseUrl.protocol !== 'https:') throw new Error('authority URL must use HTTP(S)')
    if (!options.serviceToken) throw new Error('authority service token is required')
    this.serviceToken = options.serviceToken
    this.timeoutMs = options.timeoutMs ?? 2_000
  }

  async health(signal?: AbortSignal): Promise<void> {
    await this.request('/internal/collaboration/v1/health', undefined, signal)
  }

  async authorize(room: CollaborationRoom, connectionTicket: string, connectionId: string): Promise<AccessGrant> {
    const value = await this.request('/internal/collaboration/v1/authorize', {
      room: roomWire(room),
      connection_ticket: connectionTicket,
      connection_id: connectionId,
    })
    return grantFromWire(record(value).grant)
  }

  async revalidate(room: CollaborationRoom, grant: AccessGrant): Promise<AccessGrant> {
    const value = await this.request('/internal/collaboration/v1/revalidate', {
      room: roomWire(room),
      grant: grantWire(grant),
    })
    return grantFromWire(record(value).grant)
  }

  async load(room: CollaborationRoom, grant: AccessGrant): Promise<LoadedDocument> {
    const source = record(await this.request('/internal/collaboration/v1/load', {
      room: roomWire(room),
      grant: grantWire(grant),
    }))
    return {
      snapshotBase64: textField(source, 'snapshot_base64'),
      durableRevision: integerField(source, 'durable_revision'),
			snapshotHash: textField(source, 'snapshot_hash'),
			stateVectorBase64: textField(source, 'state_vector_base64'),
			stateVectorHash: textField(source, 'state_vector_hash'),
			projectionHash: textField(source, 'projection_hash'),
		}
	}

  async validateMutation(input: ValidateMutationInput): Promise<ValidationReceipt> {
    const source = record(await this.request('/internal/collaboration/v1/validate-mutation', {
      room: roomWire(input.room),
      grant: grantWire(input.grant),
      candidate_snapshot_hash: input.candidateSnapshotHash,
      projection: input.projection,
      references: {
        asset_ids: input.references.assetIds,
        bitable_resource_ids: input.references.bitableResourceIds,
      },
    }))
    return {
      token: textField(source, 'validation_token'),
      candidateSnapshotHash: textField(source, 'candidate_snapshot_hash'),
      policyRevision: integerField(source, 'policy_revision'),
      expiresAt: instantField(source, 'expires_at'),
    }
  }

  async commit(input: CommitDocumentInput): Promise<DurableCommit> {
    const source = record(await this.request('/internal/collaboration/v1/commit', {
      room: roomWire(input.room),
      grant: grantWire(input.grant),
			validation_token: input.validationToken,
			operation_id: input.operationId,
			expected_durable_revision: input.expectedDurableRevision,
			base_state_vector_hash: input.baseStateVectorHash,
			update_base64: input.updateBase64,
			update_hash: input.updateHash,
      candidate_snapshot_hash: input.candidateSnapshotHash,
      snapshot_base64: input.snapshotBase64,
			state_vector_base64: input.stateVectorBase64,
			state_vector_hash: input.stateVectorHash,
      projection: input.projection,
      references: {
        asset_ids: input.references.assetIds,
        bitable_resource_ids: input.references.bitableResourceIds,
      },
    }))
    return {
      commitId: textField(source, 'commit_id'),
      durableRevision: integerField(source, 'durable_revision'),
			snapshotHash: textField(source, 'snapshot_hash'),
			stateVectorHash: textField(source, 'state_vector_hash'),
      projectionHash: textField(source, 'projection_hash'),
      storedAt: instantField(source, 'stored_at'),
    }
  }

  private async request(path: string, body?: unknown, callerSignal?: AbortSignal): Promise<unknown> {
    const timeout = AbortSignal.timeout(this.timeoutMs)
    const signal = callerSignal ? AbortSignal.any([callerSignal, timeout]) : timeout
    let response: Response
    try {
      response = await fetch(new URL(path, this.baseUrl), {
        method: body === undefined ? 'GET' : 'POST',
        headers: {
          authorization: `Bearer ${this.serviceToken}`,
          'content-type': 'application/json',
          'x-pm-collaboration-contract': CONTRACT_VERSION,
        },
        body: body === undefined ? undefined : JSON.stringify(body),
        signal,
      })
    } catch {
      throw unavailable()
    }

    if (!response.ok) {
      const reason = response.headers.get('x-pm-error-code') ?? 'authority-rejected'
      if (response.status === 401 || response.status === 403 || response.status === 404) throw deny(reason)
      if (response.status === 409) throw new CollaborationError('authority conflict', 4409, reason)
      if (response.status === 400 || response.status === 422) throw rejectPayload(reason)
      throw unavailable(reason)
    }
    if (response.status === 204) return {}
    try {
      return await response.json()
    } catch {
      throw unavailable('invalid-authority-response')
    }
  }
}
