import { PM_OPERATION_ACTIONS_BY_DOMAIN } from '@/generated/access-catalog'
import {
  platformApi,
  PMSafeApiError,
  toSafeApiError,
  unwrapApiData,
} from '@/shared/api'
import type { RichTextDocumentSchemaVersion } from '@/shared/ui'
import {
  type DocumentCommentAnchor,
  type DocumentCommentRecord,
  type DocumentCommentThreadPage,
  type DocumentCommentThreadRecord,
} from '../model/comment'
import {
  type CreatedDocumentNode,
  type CreatableDocumentNodeType,
  type DocumentAction,
  type DocumentAsset,
  type DocumentDeletionImpact,
  type DocumentNode,
  type DocumentNodePreference,
  type DocumentNodeSystemKind,
  type DocumentNodeType,
  type DocumentRecord,
  type DocumentSpace,
  type DocumentTree,
} from '../model/types'
import type { DocumentCollaborationTicket } from './collaboration-port'

const DOCUMENT_CONTRACT_ERROR = 'DOCUMENT_CONTRACT_INVALID'
const DOCUMENT_REQUEST_ERROR = 'DOCUMENT_REQUEST_FAILED'
const COLLABORATION_CONNECTION_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

type RawRecord = Record<string, unknown>

const knownDocumentActions = new Set<string>(PM_OPERATION_ACTIONS_BY_DOMAIN.documents)

function record(value: unknown): RawRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return value as RawRecord
}

function positiveInteger(value: unknown): number {
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed <= 0) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return parsed
}

function nonNegativeInteger(value: unknown): number {
  const parsed = Number(value)
  if (!Number.isSafeInteger(parsed) || parsed < 0) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return parsed
}

function text(value: unknown, allowEmpty = false): string {
  if (typeof value !== 'string' || (!allowEmpty && !value.trim())) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return value
}

function optionalPositiveInteger(value: unknown): number | undefined {
  if (value === null || value === undefined) return undefined
  return positiveInteger(value)
}

function optionalText(value: unknown): string | undefined {
  if (value === null || value === undefined || value === '') return undefined
  return text(value)
}

function optionalResourceId(value: unknown): string | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const id = text(value)
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$/.test(id)) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return id
}

function bool(value: unknown): boolean {
  if (typeof value !== 'boolean') throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return value
}

function positiveIntegerList(value: unknown): readonly number[] {
  if (!Array.isArray(value)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return value.map(positiveInteger)
}

function documentNodeType(value: unknown): DocumentNodeType {
  if (value === 'folder' || value === 'document' || value === 'shortcut') return value
  throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
}

function documentNodeSystemKind(value: unknown): DocumentNodeSystemKind | undefined {
  if (value === null || value === undefined || value === '') return undefined
  if (value === 'favorites_folder' || value === 'favorite_reference') return value
  throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
}

function documentSchemaVersion(value: unknown): RichTextDocumentSchemaVersion {
  if (value === 1 || value === 2 || value === 3) return value
  throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
}

function contentString(value: unknown): string {
  const source = record(value)
  if (source.type !== 'doc') throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return JSON.stringify(source)
}

export function normalizeDocumentCollaborationTicket(value: unknown): DocumentCollaborationTicket {
  const source = record(value)
  const ticket = text(source.ticket)
  if (ticket.length > 4096 || /[\r\n]/.test(ticket)) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  const websocketUrl = text(source.websocket_url)
  let parsed: URL
  try {
    parsed = new URL(websocketUrl)
  } catch {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  if (
    (parsed.protocol !== 'ws:' && parsed.protocol !== 'wss:')
    || parsed.username
    || parsed.password
    || parsed.search
    || parsed.hash
  ) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  const expiresAt = text(source.expires_at)
  if (!Number.isFinite(Date.parse(expiresAt))) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return { ticket, websocketUrl: parsed.toString(), expiresAt }
}

function allowedActions(value: unknown): readonly DocumentAction[] {
  if (!Array.isArray(value)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return [...new Set(value.flatMap((candidate) => (
    typeof candidate === 'string' && knownDocumentActions.has(candidate)
      ? [candidate as DocumentAction]
      : []
  )))]
}

export function normalizeDocumentSpace(value: unknown): DocumentSpace {
  const source = record(value)
  if (typeof source.is_default !== 'boolean') {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return {
    id: positiveInteger(source.id),
    workspaceId: positiveInteger(source.workspace_id),
    name: text(source.name),
    isDefault: source.is_default,
    collaborationEnabled: source.collaboration_enabled === true,
    createdAt: text(source.created_at),
    updatedAt: text(source.updated_at),
  }
}

function normalizeUserDisplay(value: unknown, userId: unknown) {
  if (value == null) return undefined
  const source = record(value)
  const id = positiveInteger(source.id)
  if (id !== positiveInteger(userId)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return { id, name: text(source.name), ...(optionalText(source.avatar_url) ? { avatarUrl: optionalText(source.avatar_url) } : {}) }
}

export function normalizeDocumentNode(value: unknown): DocumentNode {
  const source = record(value)
  const type = documentNodeType(source.type)
  const documentId = optionalPositiveInteger(source.document_id)
  const shortcutTargetNodeId = optionalPositiveInteger(source.shortcut_target_node_id)
  if (type === 'document' && !documentId) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  if (type === 'shortcut' && (!documentId || !shortcutTargetNodeId)) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return {
    id: positiveInteger(source.id),
    workspaceId: positiveInteger(source.workspace_id),
    spaceId: positiveInteger(source.space_id),
    parentId: optionalPositiveInteger(source.parent_id),
    type,
    title: text(source.title),
    sortOrder: nonNegativeInteger(source.sort_order),
    ...(documentId ? { documentId } : {}),
    ...(optionalResourceId(source.resource_id) ? { resourceId: optionalResourceId(source.resource_id) } : {}),
    ...(shortcutTargetNodeId ? { shortcutTargetNodeId } : {}),
    systemManaged: bool(source.system_managed),
    canMove: source.can_move === true,
    directoryRevision: optionalPositiveInteger(source.directory_revision),
    ...(documentNodeSystemKind(source.system_kind)
      ? { systemKind: documentNodeSystemKind(source.system_kind) }
      : {}),
    ...(optionalText(source.pinned_at) ? { pinnedAt: optionalText(source.pinned_at) } : {}),
    favorited: bool(source.favorited),
    ...(source.creator ? { creator: normalizeUserDisplay(source.creator, source.created_by_user_id) } : {}),
    createdBy: positiveInteger(source.created_by_user_id),
    updatedBy: positiveInteger(source.updated_by_user_id),
    createdAt: text(source.created_at),
    updatedAt: text(source.updated_at),
  }
}

export function normalizeDocumentRecord(value: unknown): DocumentRecord {
  const source = record(value)
  return {
    id: positiveInteger(source.id),
    workspaceId: positiveInteger(source.workspace_id),
    nodeId: positiveInteger(source.node_id),
    ...(optionalResourceId(source.resource_id) ? { resourceId: optionalResourceId(source.resource_id) } : {}),
    content: contentString(source.content),
    schemaVersion: documentSchemaVersion(source.schema_version),
    revision: positiveInteger(source.revision),
    contentHash: text(source.content_hash, true),
    plainText: text(source.plain_text, true),
    ownerUserId: positiveInteger(source.owner_user_id),
    createdBy: positiveInteger(source.created_by_user_id),
    updatedBy: positiveInteger(source.updated_by_user_id),
    collaborationCutoverAt: optionalText(source.collaboration_cutover_at),
    createdAt: text(source.created_at),
    updatedAt: text(source.updated_at),
    allowedActions: allowedActions(source.allowed_actions),
  }
}

export function normalizeDocumentNodePreference(value: unknown): DocumentNodePreference {
  const source = record(value)
  return {
    nodeId: positiveInteger(source.node_id),
    ...(optionalText(source.pinned_at) ? { pinnedAt: optionalText(source.pinned_at) } : {}),
    favorited: bool(source.favorited),
    updatedAt: text(source.updated_at),
  }
}

export function normalizeDocumentAsset(value: unknown): DocumentAsset {
  const source = record(value)
  return {
    id: positiveInteger(source.id),
    workspaceId: positiveInteger(source.workspace_id),
    documentId: positiveInteger(source.document_id),
    fileName: text(source.file_name),
    contentType: text(source.content_type),
    sizeBytes: positiveInteger(source.size_bytes),
    contentHash: text(source.content_hash),
    createdBy: positiveInteger(source.created_by_user_id),
    createdAt: text(source.created_at),
    updatedAt: text(source.updated_at),
  }
}

function normalizeDocumentAssets(value: unknown): readonly DocumentAsset[] {
  const source = record(value)
  if (!Array.isArray(source.items)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return source.items.map(normalizeDocumentAsset)
}

export function normalizeDocumentCommentAnchor(value: unknown): DocumentCommentAnchor {
  const source = record(value)
  const kind = text(source.kind)
  const state = text(source.state)
  if (kind === 'document' && state === 'document') return { kind, state }
  if (kind !== 'selection_snapshot_v1' || (state !== 'anchored' && state !== 'orphaned')) {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  const selection = record(source.selection)
  return {
    kind,
    state,
    documentRevision: positiveInteger(selection.document_revision),
    from: nonNegativeInteger(selection.from),
    to: positiveInteger(selection.to),
    quote: text(selection.quote),
    ...(optionalText(selection.prefix) ? { prefix: optionalText(selection.prefix) } : {}),
    ...(optionalText(selection.suffix) ? { suffix: optionalText(selection.suffix) } : {}),
  }
}

export function normalizeDocumentComment(value: unknown): DocumentCommentRecord {
  const source = record(value)
  return {
    id: positiveInteger(source.id),
    workspaceId: positiveInteger(source.workspace_id),
    documentId: positiveInteger(source.document_id),
    threadId: positiveInteger(source.thread_id),
    ...(optionalPositiveInteger(source.parent_comment_id)
      ? { parentCommentId: optionalPositiveInteger(source.parent_comment_id) }
      : {}),
    ...(source.author ? { author: normalizeUserDisplay(source.author, source.author_user_id) } : {}),
    authorUserId: positiveInteger(source.author_user_id),
    body: text(source.body, true),
    mentionedUserIds: positiveIntegerList(source.mentioned_user_ids),
    canEdit: bool(source.can_edit),
    canDelete: bool(source.can_delete),
    ...(optionalText(source.deleted_at) ? { deletedAt: optionalText(source.deleted_at) } : {}),
    createdAt: text(source.created_at),
    updatedAt: text(source.updated_at),
  }
}

export function normalizeDocumentCommentThread(value: unknown): DocumentCommentThreadRecord {
  const source = record(value)
  if (!Array.isArray(source.comments)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  const status = text(source.status)
  if (status !== 'open' && status !== 'resolved') {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return {
    id: positiveInteger(source.id),
    workspaceId: positiveInteger(source.workspace_id),
    documentId: positiveInteger(source.document_id),
    anchor: normalizeDocumentCommentAnchor(source.anchor),
    status,
    createdBy: positiveInteger(source.created_by_user_id),
    ...(optionalPositiveInteger(source.resolved_by_user_id)
      ? { resolvedBy: optionalPositiveInteger(source.resolved_by_user_id) }
      : {}),
    ...(optionalText(source.resolved_at) ? { resolvedAt: optionalText(source.resolved_at) } : {}),
    createdAt: text(source.created_at),
    updatedAt: text(source.updated_at),
    comments: source.comments.map(normalizeDocumentComment),
    canResolve: bool(source.can_resolve),
    canReopen: bool(source.can_reopen),
  }
}

function normalizeDocumentCommentThreadPage(value: unknown): DocumentCommentThreadPage {
  const source = record(value)
  if (!Array.isArray(source.items)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return {
    items: source.items.map(normalizeDocumentCommentThread),
    total: nonNegativeInteger(source.total),
  }
}

function commentAnchorPayload(anchor: DocumentCommentAnchor): RawRecord {
  if (anchor.kind === 'document') return { kind: 'document', state: 'document' }
  return {
    kind: anchor.kind,
    state: anchor.state,
    selection: {
      document_revision: anchor.documentRevision,
      from: anchor.from,
      to: anchor.to,
      quote: anchor.quote,
      prefix: anchor.prefix,
      suffix: anchor.suffix,
    },
  }
}

export function documentAssetContentPath(documentId: number, assetId: number): string {
  return `/api/v1/platform/documents/${documentId}/assets/${assetId}/content`
}

function normalizeDocumentTree(value: unknown): DocumentTree {
  const source = record(value)
  if (!Array.isArray(source.nodes)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  return {
    space: normalizeDocumentSpace(source.space),
    nodes: source.nodes.map(normalizeDocumentNode),
  }
}

function normalizeCreatedNode(value: unknown): CreatedDocumentNode {
  const source = record(value)
  return {
    node: normalizeDocumentNode(source.node),
    ...(source.document ? { document: normalizeDocumentRecord(source.document) } : {}),
  }
}

function normalizeDeletionImpact(value: unknown): DocumentDeletionImpact {
  const source = record(value)
  if (typeof source.requires_confirmation !== 'boolean') {
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
  return {
    nodeId: positiveInteger(source.node_id),
    descendantCount: nonNegativeInteger(source.descendant_count),
    documentCount: nonNegativeInteger(source.document_count),
    requiresConfirmation: source.requires_confirmation,
  }
}

function parseContentPayload(content: string): RawRecord {
  try {
    const parsed: unknown = JSON.parse(content)
    const source = record(parsed)
    if (source.type !== 'doc') throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
    return source
  } catch (error) {
    if (error instanceof PMSafeApiError) throw error
    throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
  }
}

function latestRevision(error: unknown): number | undefined {
  const raw = (error as {
    response?: { data?: { details?: { latest_revision?: unknown } } }
  })?.response?.data?.details?.latest_revision
  const revision = Number(raw)
  return Number.isSafeInteger(revision) && revision > 0 ? revision : undefined
}

function latestArchiveImpact(error: unknown): DocumentDeletionImpact | undefined {
  const value = (error as {
    response?: { data?: { details?: { latest_impact?: unknown } } }
  })?.response?.data?.details?.latest_impact
  if (value === undefined) return undefined
  try {
    return normalizeDeletionImpact(value)
  } catch {
    return undefined
  }
}

export class DocumentRevisionConflictError extends PMSafeApiError {
  readonly latestRevision: number | undefined

  constructor(latest: number | undefined) {
    super('DOCUMENT_REVISION_CONFLICT', 409)
    this.name = 'DocumentRevisionConflictError'
    this.latestRevision = latest
  }
}

export class DocumentArchiveImpactChangedError extends PMSafeApiError {
  readonly latestImpact: DocumentDeletionImpact | undefined

  constructor(latestImpact: DocumentDeletionImpact | undefined) {
    super('DOCUMENT_ARCHIVE_IMPACT_CHANGED', 409)
    this.name = 'DocumentArchiveImpactChangedError'
    this.latestImpact = latestImpact
  }
}

async function safeRequest<T>(request: () => Promise<T>): Promise<T> {
  try {
    return await request()
  } catch (error) {
    const safe = toSafeApiError(error, DOCUMENT_REQUEST_ERROR)
    if (safe.status === 409 && safe.code === 'DOCUMENT_REVISION_CONFLICT') {
      throw new DocumentRevisionConflictError(latestRevision(error))
    }
    if (safe.status === 409 && safe.code === 'DOCUMENT_ARCHIVE_IMPACT_CHANGED') {
      throw new DocumentArchiveImpactChangedError(latestArchiveImpact(error))
    }
    throw safe
  }
}

export type CreateDocumentNodeInput = {
  collaborative?: boolean
  parentId?: number
  type: CreatableDocumentNodeType
  title: string
}

export type MoveDocumentNodeInput = {
  beforeNodeId?: number
  afterNodeId?: number
  expectedDirectoryRevision?: number
  parentId?: number
  sortOrder?: number
}

export type CopyDocumentNodeInput = {
  parentId?: number
}

export type CreateDocumentShortcutInput = {
  parentId?: number
}

export type UpdateDocumentNodePreferenceInput =
  | { pinned: boolean; favorited?: boolean }
  | { pinned?: boolean; favorited: boolean }

export type UpdateDocumentInput = {
  content: string
  expectedRevision: number
  schemaVersion: RichTextDocumentSchemaVersion
}

export type ArchiveDocumentNodeInput = {
  confirmNonEmpty: boolean
  expectedDescendantCount?: number
  expectedDocumentCount?: number
}

export type CreateDocumentCommentThreadInput = {
  anchor: DocumentCommentAnchor
  body: string
}

export type ReplyDocumentCommentInput = {
  parentCommentId?: number
  body: string
}

export interface DocumentsClient {
  defaultSpace(signal?: AbortSignal): Promise<DocumentSpace>
  tree(signal?: AbortSignal): Promise<DocumentTree>
  createNode(input: CreateDocumentNodeInput): Promise<CreatedDocumentNode>
  renameNode(nodeId: number, title: string): Promise<DocumentNode>
  moveNode(nodeId: number, input: MoveDocumentNodeInput): Promise<DocumentNode>
  copyNode(nodeId: number, input: CopyDocumentNodeInput): Promise<CreatedDocumentNode>
  createShortcut(nodeId: number, input: CreateDocumentShortcutInput): Promise<DocumentNode>
  updateNodePreference(
    nodeId: number,
    input: UpdateDocumentNodePreferenceInput,
  ): Promise<DocumentNodePreference>
  deletionImpact(nodeId: number, signal?: AbortSignal): Promise<DocumentDeletionImpact>
  archiveNode(nodeId: number, input: ArchiveDocumentNodeInput): Promise<void>
  document(documentId: number, signal?: AbortSignal): Promise<DocumentRecord>
  exportDocument(documentId: number, signal?: AbortSignal): Promise<DocumentRecord>
  updateDocument(documentId: number, input: UpdateDocumentInput): Promise<DocumentRecord>
  collaborationTicket(
    resourceId: string,
    connectionId: string,
    signal?: AbortSignal,
  ): Promise<DocumentCollaborationTicket>
  transferDocumentOwner(documentId: number, ownerUserId: number): Promise<DocumentRecord>
  listAssets(documentId: number, signal?: AbortSignal): Promise<readonly DocumentAsset[]>
  uploadAsset(documentId: number, file: File): Promise<DocumentAsset>
  downloadAsset(documentId: number, assetId: number, signal?: AbortSignal): Promise<Blob>
  deleteAsset(documentId: number, assetId: number): Promise<void>
  listCommentThreads(documentId: number, signal?: AbortSignal): Promise<DocumentCommentThreadPage>
  createCommentThread(documentId: number, input: CreateDocumentCommentThreadInput): Promise<DocumentCommentThreadRecord>
  replyComment(documentId: number, threadId: number, input: ReplyDocumentCommentInput): Promise<DocumentCommentRecord>
  editComment(documentId: number, commentId: number, body: string): Promise<DocumentCommentRecord>
  deleteComment(documentId: number, commentId: number): Promise<void>
  resolveCommentThread(documentId: number, threadId: number): Promise<DocumentCommentThreadRecord>
  reopenCommentThread(documentId: number, threadId: number): Promise<DocumentCommentThreadRecord>
}

export const pmDocumentsClient: DocumentsClient = {
  defaultSpace: (signal) => safeRequest(async () => normalizeDocumentSpace(
    unwrapApiData((await platformApi.get('/personal-document-spaces/default', { signal })).data),
  )),
  tree: (signal) => safeRequest(async () => normalizeDocumentTree(
    unwrapApiData((await platformApi.get('/personal-document-nodes/tree', { signal })).data),
  )),
  createNode: (input) => safeRequest(async () => normalizeCreatedNode(
    unwrapApiData((await platformApi.post('/personal-document-nodes', {
      parent_id: input.parentId,
      type: input.type,
      title: input.title,
      ...(input.collaborative ? { collaborative: true } : {}),
    })).data),
  )),
  renameNode: (nodeId, title) => safeRequest(async () => normalizeDocumentNode(
    unwrapApiData((await platformApi.patch(`/personal-document-nodes/${nodeId}`, { title })).data),
  )),
  moveNode: (nodeId, input) => safeRequest(async () => normalizeDocumentNode(
    unwrapApiData((await platformApi.post(`/personal-document-nodes/${nodeId}/move`, {
      parent_id: input.parentId,
      before_node_id: input.beforeNodeId,
      after_node_id: input.afterNodeId,
      expected_directory_revision: input.expectedDirectoryRevision,
    })).data),
  )),
  copyNode: (nodeId, input) => safeRequest(async () => normalizeCreatedNode(
    unwrapApiData((await platformApi.post(`/personal-document-nodes/${nodeId}/copies`, {
      parent_id: input.parentId,
    })).data),
  )),
  createShortcut: (nodeId, input) => safeRequest(async () => normalizeDocumentNode(
    unwrapApiData((await platformApi.post(`/personal-document-nodes/${nodeId}/shortcuts`, {
      parent_id: input.parentId,
    })).data),
  )),
  updateNodePreference: (nodeId, input) => safeRequest(async () => normalizeDocumentNodePreference(
    unwrapApiData((await platformApi.put(`/personal-document-nodes/${nodeId}/preferences`, input)).data),
  )),
  deletionImpact: (nodeId, signal) => safeRequest(async () => normalizeDeletionImpact(
    unwrapApiData((await platformApi.get(`/personal-document-nodes/${nodeId}/deletion-impact`, { signal })).data),
  )),
  archiveNode: (nodeId, input) => safeRequest(async () => {
    const source = record(unwrapApiData((await platformApi.delete(`/personal-document-nodes/${nodeId}`, {
      params: {
        confirm_non_empty: input.confirmNonEmpty,
        ...(input.confirmNonEmpty ? {
          expected_descendant_count: input.expectedDescendantCount,
          expected_document_count: input.expectedDocumentCount,
        } : {}),
      },
    })).data))
    if (positiveInteger(source.archived_id) !== nodeId) {
      throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
    }
  }),
  document: (documentId, signal) => safeRequest(async () => normalizeDocumentRecord(
    unwrapApiData((await platformApi.get(`/documents/${documentId}`, { signal })).data),
  )),
  exportDocument: (documentId, signal) => safeRequest(async () => normalizeDocumentRecord(
    unwrapApiData((await platformApi.get(`/documents/${documentId}/export`, { signal })).data),
  )),
  updateDocument: (documentId, input) => safeRequest(async () => normalizeDocumentRecord(
    unwrapApiData((await platformApi.patch(`/documents/${documentId}`, {
      content: parseContentPayload(input.content),
      expected_revision: input.expectedRevision,
      schema_version: input.schemaVersion,
    })).data),
  )),
  collaborationTicket: (resourceId, connectionId, signal) => safeRequest(async () => {
    if (!COLLABORATION_CONNECTION_ID.test(connectionId) || !optionalResourceId(resourceId)) {
      throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
    }
    return normalizeDocumentCollaborationTicket(unwrapApiData((await platformApi.post(
      `/document-resources/${encodeURIComponent(resourceId)}/collaboration/ticket`,
      undefined,
      {
        signal,
        headers: { 'X-PM-Collaboration-Connection': connectionId },
      },
    )).data))
  }),
  transferDocumentOwner: (documentId, ownerUserId) => safeRequest(async () => normalizeDocumentRecord(
    unwrapApiData((await platformApi.put(`/documents/${documentId}/owner`, {
      owner_user_id: ownerUserId,
    })).data),
  )),
  listAssets: (documentId, signal) => safeRequest(async () => normalizeDocumentAssets(
    unwrapApiData((await platformApi.get(`/documents/${documentId}/assets`, { signal })).data),
  )),
  uploadAsset: (documentId, file) => safeRequest(async () => {
    const payload = new FormData()
    payload.append('file', file)
    return normalizeDocumentAsset(unwrapApiData((await platformApi.post(
      `/documents/${documentId}/assets`,
      payload,
    )).data))
  }),
  downloadAsset: (documentId, assetId, signal) => safeRequest(async () => {
    const response = await platformApi.get(
      `/documents/${documentId}/assets/${assetId}/content`,
      { responseType: 'blob', signal },
    )
    if (!(response.data instanceof Blob)) throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
    return response.data
  }),
  deleteAsset: (documentId, assetId) => safeRequest(async () => {
    const source = record(unwrapApiData((await platformApi.delete(
      `/documents/${documentId}/assets/${assetId}`,
    )).data))
    if (positiveInteger(source.deleted_id) !== assetId) {
      throw new PMSafeApiError(DOCUMENT_CONTRACT_ERROR)
    }
  }),
  listCommentThreads: (documentId, signal) => safeRequest(async () => normalizeDocumentCommentThreadPage(
    unwrapApiData((await platformApi.get(`/documents/${documentId}/comment-threads`, {
      params: { status: 'all', limit: 200, offset: 0 },
      signal,
    })).data),
  )),
  createCommentThread: (documentId, input) => safeRequest(async () => normalizeDocumentCommentThread(
    unwrapApiData((await platformApi.post(`/documents/${documentId}/comment-threads`, {
      anchor: commentAnchorPayload(input.anchor),
      body: JSON.stringify(parseContentPayload(input.body)),
    })).data),
  )),
  replyComment: (documentId, threadId, input) => safeRequest(async () => normalizeDocumentComment(
    unwrapApiData((await platformApi.post(`/documents/${documentId}/comment-threads/${threadId}/comments`, {
      parent_comment_id: input.parentCommentId,
      body: JSON.stringify(parseContentPayload(input.body)),
    })).data),
  )),
  editComment: (documentId, commentId, body) => safeRequest(async () => normalizeDocumentComment(
    unwrapApiData((await platformApi.patch(`/documents/${documentId}/comments/${commentId}`, {
      body: JSON.stringify(parseContentPayload(body)),
    })).data),
  )),
  deleteComment: (documentId, commentId) => safeRequest(async () => {
    await platformApi.delete(`/documents/${documentId}/comments/${commentId}`)
  }),
  resolveCommentThread: (documentId, threadId) => safeRequest(async () => normalizeDocumentCommentThread(
    unwrapApiData((await platformApi.post(`/documents/${documentId}/comment-threads/${threadId}/resolve`)).data),
  )),
  reopenCommentThread: (documentId, threadId) => safeRequest(async () => normalizeDocumentCommentThread(
    unwrapApiData((await platformApi.post(`/documents/${documentId}/comment-threads/${threadId}/reopen`)).data),
  )),
}
