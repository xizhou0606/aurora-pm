import type { JSONContent } from '@tiptap/core'

export type AccessLevel = 'viewer' | 'editor'

export interface CollaborationRoom {
  workspaceId: string
  resourceId: string
}

export interface ActorProjection {
  id: string
  displayName: string
}

export interface AccessGrant {
  grantId: string
  actor: ActorProjection
  level: AccessLevel
  policyRevision: number
  expiresAt: string
  revalidateAfterMs: number
}

export interface LoadedDocument {
	snapshotBase64: string
	durableRevision: number
	snapshotHash: string
	stateVectorBase64: string
	stateVectorHash: string
	projectionHash: string
}

export interface ReferenceSet {
  assetIds: string[]
  bitableResourceIds: string[]
}

export interface ValidationReceipt {
  token: string
  candidateSnapshotHash: string
  policyRevision: number
  expiresAt: string
}

export interface DurableCommit {
  commitId: string
  durableRevision: number
	snapshotHash: string
	stateVectorHash: string
	projectionHash: string
  storedAt: string
}

export interface ValidateMutationInput {
  room: CollaborationRoom
  grant: AccessGrant
  candidateSnapshotHash: string
  projection: JSONContent
  references: ReferenceSet
}

export interface CommitDocumentInput extends ValidateMutationInput {
	validationToken: string
	operationId: string
	expectedDurableRevision: number
	baseStateVectorHash: string
	updateBase64: string
	updateHash: string
	snapshotBase64: string
	stateVectorBase64: string
	stateVectorHash: string
}

export interface AuthorityBridge {
  health(signal?: AbortSignal): Promise<void>
  authorize(room: CollaborationRoom, connectionTicket: string, connectionId: string): Promise<AccessGrant>
  revalidate(room: CollaborationRoom, grant: AccessGrant): Promise<AccessGrant>
	load(room: CollaborationRoom, grant: AccessGrant): Promise<LoadedDocument>
  validateMutation(input: ValidateMutationInput): Promise<ValidationReceipt>
  commit(input: CommitDocumentInput): Promise<DurableCommit>
}

export interface ConnectionContext {
  room: CollaborationRoom
  grant: AccessGrant
}

export interface DurableAcknowledgement extends DurableCommit {
  type: 'durable_commit'
  roomName: string
  stateVectorBase64: string
}
