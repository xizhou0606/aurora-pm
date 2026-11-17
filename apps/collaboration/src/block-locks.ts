import * as Y from 'yjs'
import { rejectPayload } from './errors.js'

// Locks belong to the live room, not the durable document. Relative positions
// keep their identity when preceding paragraphs are inserted or moved.
export class BlockLocks {
  private readonly locks = new Map<object, { position: Y.RelativePosition; expires: number }>()

  release(owner: object): void { this.locks.delete(owner) }

  claim(owner: object, document: Y.Doc, cursor: unknown): Y.RelativePosition | undefined {
    this.prune()
    const head = cursor && typeof cursor === 'object' ? (cursor as { head?: unknown }).head : undefined
    const block = resolveBlock(document, head)
    if (!block) { this.release(owner); return undefined }
    for (const [other, lock] of this.locks) {
      if (other !== owner && resolveBlock(document, lock.position) === block) {
        this.release(owner)
        return undefined
      }
    }
    const position = Y.createRelativePositionFromTypeIndex(block, 0)
    this.locks.set(owner, { position, expires: Date.now() + 30_000 })
    return position
  }

  assertMutation(owner: object, document: Y.Doc, candidate: Y.Doc): void {
    this.prune()
    for (const [other, lock] of this.locks) {
      if (other === owner) continue
      const before = resolveBlock(document, lock.position)
      const after = resolveBlock(candidate, lock.position)
      if (before && (!after || before.toString() !== after.toString())) {
        throw rejectPayload('document-block-locked')
      }
    }
  }

  private prune(): void {
    for (const [owner, lock] of this.locks) if (lock.expires <= Date.now()) this.locks.delete(owner)
  }
}

function resolveBlock(document: Y.Doc, value: unknown): Y.XmlElement | undefined {
  if (!value || typeof value !== 'object') return undefined
  try {
    const position = Y.createAbsolutePositionFromRelativePosition(Y.createRelativePositionFromJSON(value), document)
    const type = position?.type
    const block = type instanceof Y.XmlText ? type.parent : type
    return block instanceof Y.XmlElement ? block : undefined
  } catch { return undefined }
}
