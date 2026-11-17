import type { JSONContent } from '@tiptap/core'

import {
  parseRichTextValue,
  type RichTextSerialization,
} from '../edit/rich-text-serialization'
import { richTextNodeTypes } from './rich-text-extensions'
import type {
  RichTextDocumentSchemaVersion,
  RichTextPreset,
} from './rich-text-types'

function isDocument(value: unknown): value is JSONContent {
  return Boolean(
    value
    && typeof value === 'object'
    && !Array.isArray(value)
    && (value as { type?: unknown }).type === 'doc',
  )
}

function preserveUnknownTopLevelNodes(
  document: JSONContent,
  knownNodeTypes: ReadonlySet<string>,
): JSONContent {
  return {
    ...document,
    content: document.content?.map((node) => {
      if (!node.type || knownNodeTypes.has(node.type)) return node
      return {
        type: 'unsupportedBlock',
        attrs: {
          original: JSON.stringify(node),
          originalType: node.type,
        },
      }
    }),
  }
}

export function restoreUnsupportedRichTextNodes(node: JSONContent): JSONContent {
  if (node.type === 'unsupportedBlock') {
    try {
      const original: unknown = JSON.parse(String(node.attrs?.original || ''))
      if (original && typeof original === 'object' && !Array.isArray(original)) {
        return original as JSONContent
      }
    } catch {
      // Preserve the visible fallback node when the source payload is corrupt.
    }
  }
  return {
    ...node,
    ...(node.content
      ? { content: node.content.map(restoreUnsupportedRichTextNodes) }
      : {}),
  }
}

export function parseRichTextPresetValue(input: {
  value: string
  serialization: RichTextSerialization
  preset: RichTextPreset
  schemaVersion: RichTextDocumentSchemaVersion
  preserveUnknown?: boolean
}): string | JSONContent {
  const parsed = parseRichTextValue(input.value, input.serialization)
  if (
    input.serialization !== 'json'
    || input.preset !== 'document'
    || input.schemaVersion < 2
    || input.preserveUnknown === false
    || !isDocument(parsed)
  ) {
    return parsed
  }
  return preserveUnknownTopLevelNodes(
    parsed,
    richTextNodeTypes(input.preset, input.schemaVersion),
  )
}

export function serializeRichTextPresetValue(
  editor: { getHTML: () => string; getJSON: () => JSONContent },
  serialization: RichTextSerialization,
  preset: RichTextPreset,
): string {
  if (serialization === 'html') return editor.getHTML()
  const document = editor.getJSON()
  return JSON.stringify(
    preset === 'document' ? restoreUnsupportedRichTextNodes(document) : document,
  )
}

export function migrateRichTextDocument(
  value: string,
  fromVersion: RichTextDocumentSchemaVersion,
  toVersion: RichTextDocumentSchemaVersion,
): string {
  if (toVersion < fromVersion) {
    throw new Error('Rich-text schema downgrade is not supported')
  }
  const parsed: unknown = JSON.parse(value)
  if (!isDocument(parsed)) {
    throw new Error('Rich-text document must be a Tiptap JSON document')
  }
  // Schema v2 is additive. Canonicalization is intentionally explicit so merely
  // opening a v1 document never causes a write.
  return JSON.stringify(parsed)
}
