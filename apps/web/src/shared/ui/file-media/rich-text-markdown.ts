import type { JSONContent } from '@tiptap/core'
import { MarkdownManager } from '@tiptap/markdown'

import { createRichTextExtensions } from './rich-text-extensions'
import type { RichTextDocumentLabels } from './rich-text-types'

function markdownManager(labels: RichTextDocumentLabels): MarkdownManager {
  return new MarkdownManager({
    extensions: createRichTextExtensions({
      preset: 'document',
      schemaVersion: 2,
      placeholder: '',
      documentLabels: labels,
    }),
    markedOptions: { gfm: true, breaks: false },
  })
}

export function parseRichTextMarkdown(markdown: string, labels: RichTextDocumentLabels): JSONContent {
  return markdownManager(labels).parse(markdown.replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n'))
}

export function serializeRichTextMarkdown(content: JSONContent, labels: RichTextDocumentLabels): string {
  // TipTap 3.29's image/link serializers interpolate attributes verbatim. Escape the
  // attribute payloads, while leaving all block/mark serialization to its registry.
  const escapeAttribute = (value: unknown) => String(value ?? '')
    .replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replace(/[\\\[\]"]/g, '\\$&')
    .replace(/\r?\n/g, '&#10;')
  const escapeDestination = (value: unknown) => String(value ?? '').replace(/[()<>"`]/g, (character) => `%${character.charCodeAt(0).toString(16).toUpperCase()}`)
  const prepare = (node: JSONContent): JSONContent => ({
    ...node,
    ...(node.type === 'image' ? { attrs: { ...node.attrs,
      src: escapeDestination(node.attrs?.src), alt: escapeAttribute(node.attrs?.alt), title: escapeAttribute(node.attrs?.title) } } : {}),
    ...(node.marks ? { marks: node.marks.map((mark) => mark.type === 'link'
      ? { ...mark, attrs: { ...mark.attrs, href: escapeDestination(mark.attrs?.href), title: escapeAttribute(mark.attrs?.title) } }
      : mark) } : {}),
    ...(node.content ? { content: node.content.map(prepare) } : {}),
  })
  return markdownManager(labels).serialize(prepare(content))
}
