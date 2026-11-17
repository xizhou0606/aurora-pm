import { timingSafeEqual } from 'node:crypto'
import * as Y from 'yjs'
import type { JSONContent } from '@tiptap/core'
import type { AuthorityBridge, CollaborationRoom } from './contract.js'
import { documentFromProjection, COLLABORATION_FIELD, MAX_DOCUMENT_BYTES, MAX_UPDATE_BYTES, projectAndValidate, projectionHash, sha256 } from './projection.js'
import { CollaborationError } from './errors.js'
import { encodeRoomName } from './room.js'

export class ReplacementError extends Error {
 constructor(readonly code: string, readonly status = 409) { super(code) }
}

export class ContentReplacement {
 readonly active = new Set<string>()
 constructor(private readonly authority: AuthorityBridge, private readonly busy: (room: string) => boolean, private readonly serviceToken?: string) {}

 async handle(request: Request): Promise<Response> {
  const reply = (status: number, body: unknown) =>
    Response.json(body, { status, headers: { 'cache-control': 'no-store' } })
  const provided = Buffer.from(request.headers.get('authorization') ?? '')
  const expected = Buffer.from(`Bearer ${this.serviceToken ?? ''}`)
  if (!this.serviceToken || provided.length !== expected.length || !timingSafeEqual(provided, expected)) return reply(403, { code: 'DOCUMENT_FORBIDDEN' })
  try {
   const body = Buffer.from(await request.arrayBuffer())
   if (body.length > MAX_DOCUMENT_BYTES) throw new ReplacementError('DOCUMENT_INPUT_INVALID', 422)
   const input = JSON.parse(body.toString()) as ReplacementInput
   return reply(200, await this.replace(input))
  } catch (error) {
   if (error instanceof ReplacementError) return reply(error.status, { code: error.code })
   else if (error instanceof CollaborationError && error.code === 4403) return reply(403,{code:'DOCUMENT_FORBIDDEN'})
   else if (error instanceof CollaborationError && error.code === 4400) return reply(422,{code:'DOCUMENT_INPUT_INVALID'})
   else if (error instanceof CollaborationError && error.code === 4409) return reply(409,{code:'DOCUMENT_VERSION_CONFLICT'})
   else if (error instanceof SyntaxError) return reply(422,{code:'DOCUMENT_INPUT_INVALID'})
   else return reply(503, { code: 'DOCUMENT_EDITING_STATE_UNAVAILABLE' })
  }
 }

 async replace(input: ReplacementInput) {
  if (!input || !input.room || typeof input.ticket !== 'string' || !Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 1 || !/^tool-[a-f0-9]{64}$/.test(input.operationId)) throw new ReplacementError('DOCUMENT_INPUT_INVALID', 422)
  const name = encodeRoomName(input.room)
  if (this.active.has(name)) throw new ReplacementError('DOCUMENT_REPLACE_IN_PROGRESS')
  if (this.busy(name)) throw new ReplacementError('DOCUMENT_EDITING_IN_PROGRESS')
  // No await between the room check and reservation. Authentication and loading
  // check this set too, so a connection cannot slip into a replacement.
  this.active.add(name)
  const doc = new Y.Doc()
  let replacement: Y.Doc | undefined
  try {
   const grant = await this.authority.authorize(input.room, input.ticket, input.connectionId)
   if (grant.level !== 'editor' || Date.parse(grant.expiresAt) <= Date.now()) throw new ReplacementError('DOCUMENT_FORBIDDEN', 403)
   const loaded = await this.authority.load(input.room, grant)
   if (loaded.durableRevision !== input.expectedRevision) throw new ReplacementError('DOCUMENT_VERSION_CONFLICT')
   const oldSnapshot = Buffer.from(loaded.snapshotBase64, 'base64')
   if (sha256(oldSnapshot) !== loaded.snapshotHash) throw new Error('corrupt snapshot')
   Y.applyUpdate(doc, oldSnapshot)
   if (sha256(Y.encodeStateVector(doc)) !== loaded.stateVectorHash || projectionHash(projectAndValidate(doc).projection) !== loaded.projectionHash) throw new Error('corrupt projection')
   const baseVector = Y.encodeStateVector(doc)
   try {
    replacement = documentFromProjection(input.content)
    projectAndValidate(replacement)
   } catch { throw new ReplacementError('DOCUMENT_INPUT_INVALID', 422) }
   const fragment = doc.getXmlFragment(COLLABORATION_FIELD)
   doc.transact(() => {
    fragment.delete(0, fragment.length)
    fragment.insert(0, replacement!.getXmlFragment(COLLABORATION_FIELD).toArray().map(node => { if (!(node instanceof Y.XmlElement) && !(node instanceof Y.XmlText)) throw new ReplacementError('DOCUMENT_INPUT_INVALID',422); return node.clone() }))
   })
   const { projection, references } = projectAndValidate(doc)
   const snapshot = Y.encodeStateAsUpdate(doc)
   const update = Y.encodeStateAsUpdate(doc, baseVector)
   const vector = Y.encodeStateVector(doc)
   if (update.length > MAX_UPDATE_BYTES || snapshot.length > MAX_DOCUMENT_BYTES) throw new ReplacementError('DOCUMENT_INPUT_INVALID', 422)
   const candidateSnapshotHash = sha256(snapshot)
   const receipt = await this.authority.validateMutation({ room: input.room, grant, candidateSnapshotHash, projection, references })
   if (receipt.candidateSnapshotHash !== candidateSnapshotHash || receipt.policyRevision !== grant.policyRevision || Date.parse(receipt.expiresAt) <= Date.now()) throw new Error('invalid validation receipt')
   const commit = await this.authority.commit({ room: input.room, grant, candidateSnapshotHash, projection, references, validationToken: receipt.token, operationId: input.operationId, expectedDurableRevision: loaded.durableRevision, baseStateVectorHash: loaded.stateVectorHash, updateBase64: Buffer.from(update).toString('base64'), updateHash: sha256(update), snapshotBase64: Buffer.from(snapshot).toString('base64'), stateVectorBase64: Buffer.from(vector).toString('base64'), stateVectorHash: sha256(vector) })
   if (commit.snapshotHash !== candidateSnapshotHash || commit.projectionHash !== projectionHash(projection) || commit.stateVectorHash !== sha256(vector)) throw new Error('invalid commit acknowledgement')
   return { revision: commit.durableRevision, content_hash: commit.projectionHash }
  } finally {
   doc.destroy(); replacement?.destroy(); this.active.delete(name)
  }
 }
}
export interface ReplacementInput {
 room: CollaborationRoom
 ticket: string
 connectionId: string
 operationId: string
 expectedRevision: number
 content: JSONContent
}
