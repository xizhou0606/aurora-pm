import { createHash } from 'node:crypto'
import { Node, getSchema, type Extensions, type JSONContent } from '@tiptap/core'
import Image from '@tiptap/extension-image'
import { TableKit } from '@tiptap/extension-table'
import TaskItem from '@tiptap/extension-task-item'
import TaskList from '@tiptap/extension-task-list'
import StarterKit from '@tiptap/starter-kit'
import { TiptapTransformer } from '@hocuspocus/transformer'
import * as Y from 'yjs'
import type { ReferenceSet } from './contract.js'
import { rejectPayload } from './errors.js'

export const COLLABORATION_FIELD = 'default'
export const MAX_UPDATE_BYTES = 256 * 1024
export const MAX_DOCUMENT_BYTES = 4 * 1024 * 1024
export const MAX_DOCUMENT_NODES = 20_000
export const MAX_TEXT_BYTES = 2 * 1024 * 1024

const BITABLE_REFERENCE_PATTERN = /^[A-Za-z0-9_-]{1,128}$/

// These nodes deliberately mirror only the serialized attributes owned by the
// document editor. Access to either referenced resource remains authoritative
// in Go and is never inferred from this projection schema.
const DocumentImage = Image.extend({
  addAttributes() {
    return { ...this.parent?.(), assetId: { default: null } }
  },
}).configure({ inline: false, allowBase64: false })

const DocumentMention = Node.create({
  name: 'mention', group: 'inline', inline: true, atom: true, selectable: false,
  addAttributes: () => ({ id: { default: null }, label: { default: '' }, username: { default: '' } }),
})

const DocumentCallout = Node.create({
  name: 'callout', group: 'block', content: 'inline*', defining: true,
  addAttributes: () => ({ tone: { default: 'info' } }),
})

const DocumentTableOfContents = Node.create({
  name: 'tableOfContents', group: 'block', atom: true, selectable: true,
  addAttributes: () => ({ items: { default: [] } }),
})

const DocumentUnsupportedBlock = Node.create({
  name: 'unsupportedBlock', group: 'block', atom: true, selectable: true, draggable: false,
  addAttributes: () => ({ original: { default: '' }, originalType: { default: '' } }),
})

const DocumentBitable = Node.create({
  name: 'bitable',
  group: 'block',
  atom: true,
  selectable: true,
  draggable: false,
  isolating: true,
  addAttributes: () => ({
    schemaVersion: { default: 1 },
    resourceId: { default: '' },
    tableId: { default: '' },
    viewId: { default: '' },
    height: { default: 420 },
  }),
  parseHTML: () => [{ tag: 'div[data-bitable-reference]' }],
  renderHTML: ({ HTMLAttributes }) => ['div', { ...HTMLAttributes, 'data-bitable-reference': '' }],
})

export const collaborationExtensions: Extensions = [
  StarterKit.configure({ undoRedo: false }),
  DocumentImage,
  DocumentMention,
  TaskList,
  TaskItem.configure({ nested: true }),
  TableKit.configure({ table: { resizable: true, renderWrapper: true } }),
  DocumentCallout,
  DocumentTableOfContents,
  DocumentUnsupportedBlock,
  DocumentBitable,
]

const schema = getSchema(collaborationExtensions)

function assertKnownYjsNodes(document: Y.Doc): void {
  const visit = (value: unknown): void => {
    if (value instanceof Y.XmlText) return
    if (!(value instanceof Y.XmlElement) || schema.nodes[value.nodeName] === undefined) {
      throw rejectPayload('invalid-prosemirror-schema')
    }
    value.toArray().forEach(visit)
  }
  document.getXmlFragment(COLLABORATION_FIELD).toArray().forEach(visit)
}

function stable(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stable).join(',')}]`
  if (value !== null && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>).sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0)
    return `{${entries.map(([key, item]) => `${jsonString(key)}:${stable(item)}`).join(',')}}`
  }
  return jsonString(value)
}

function jsonString(value: unknown): string {
  const encoded = JSON.stringify(value)
  if (encoded === undefined) throw rejectPayload('non-json-projection')
  // Go's encoding/json always escapes these separators, even with HTML escaping
  // disabled. Keep projection attestations byte-identical across the bridge.
  return encoded.replace(/\u2028/g, '\\u2028').replace(/\u2029/g, '\\u2029')
}

export function sha256(value: Uint8Array | string): string {
  return createHash('sha256').update(value).digest('hex')
}

export function encodeSnapshot(document: Y.Doc): Uint8Array {
  const snapshot = Y.encodeStateAsUpdate(document)
  if (snapshot.byteLength > MAX_DOCUMENT_BYTES) throw rejectPayload('document-too-large')
  return snapshot
}

export function projectAndValidate(document: Y.Doc): { projection: JSONContent; references: ReferenceSet } {
  const sharedFields = [...document.share.keys()]
  if (sharedFields.some((field) => field !== COLLABORATION_FIELD)) {
    throw rejectPayload('unknown-yjs-shared-field')
  }
  assertKnownYjsNodes(document)

  let projection: JSONContent
  try {
    projection = TiptapTransformer.fromYdoc(document, COLLABORATION_FIELD) as JSONContent
    schema.nodeFromJSON(projection).check()
  } catch {
    throw rejectPayload('invalid-prosemirror-schema')
  }

  const references: ReferenceSet = { assetIds: [], bitableResourceIds: [] }
  let nodes = 0
  let textBytes = 0
  const visit = (node: JSONContent): void => {
    nodes += 1
    if (nodes > MAX_DOCUMENT_NODES) throw rejectPayload('too-many-document-nodes')
    if (typeof node.text === 'string') {
      textBytes += Buffer.byteLength(node.text)
      if (textBytes > MAX_TEXT_BYTES) throw rejectPayload('document-text-too-large')
    }
    if (node.type === 'image') {
      const assetId = node.attrs?.assetId
      if (assetId !== null && assetId !== undefined) {
        if (!Number.isSafeInteger(assetId) || (assetId as number) <= 0) throw rejectPayload('invalid-asset-reference')
        references.assetIds.push(String(assetId))
      }
    }
    if (node.type === 'bitable') {
      const { schemaVersion, resourceId, tableId, viewId, height } = node.attrs ?? {}
      if (
        schemaVersion !== 1 || typeof resourceId !== 'string' || !BITABLE_REFERENCE_PATTERN.test(resourceId) ||
        typeof tableId !== 'string' || !BITABLE_REFERENCE_PATTERN.test(tableId) ||
        typeof viewId !== 'string' || !BITABLE_REFERENCE_PATTERN.test(viewId) ||
        !Number.isSafeInteger(height) || (height as number) < 240 || (height as number) > 900
      ) throw rejectPayload('invalid-bitable-reference')
      references.bitableResourceIds.push(resourceId)
    }
    node.content?.forEach(visit)
  }
  visit(projection)
  references.assetIds = [...new Set(references.assetIds)].sort()
  references.bitableResourceIds = [...new Set(references.bitableResourceIds)].sort()
  return { projection, references }
}

export function projectionHash(projection: JSONContent): string {
  return sha256(stable(projection))
}

export function validateCandidate(document: Y.Doc, update: Uint8Array): {
  document: Y.Doc
  snapshot: Uint8Array
  snapshotHash: string
  projection: JSONContent
  references: ReferenceSet
} {
  if (update.byteLength > MAX_UPDATE_BYTES) throw rejectPayload('update-too-large')
  const candidate = new Y.Doc()
  try {
    Y.applyUpdate(candidate, Y.encodeStateAsUpdate(document))
    Y.applyUpdate(candidate, update)
  } catch {
    throw rejectPayload('malformed-yjs-update')
  }
  const snapshot = encodeSnapshot(candidate)
  const { projection, references } = projectAndValidate(candidate)
  return { document: candidate, snapshot, snapshotHash: sha256(snapshot), projection, references }
}

export function documentFromProjection(projection: JSONContent): Y.Doc {
  return TiptapTransformer.toYdoc(projection, COLLABORATION_FIELD, collaborationExtensions)
}
