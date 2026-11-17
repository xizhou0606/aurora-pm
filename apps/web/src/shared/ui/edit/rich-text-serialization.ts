import type { JSONContent } from '@tiptap/core'

export type RichTextSerialization = 'html' | 'json'

const EMPTY_DOCUMENT: JSONContent = {
  type: 'doc',
  content: [{ type: 'paragraph' }],
}

const RICH_TEXT_BLOCK_TYPES = new Set([
  'blockquote', 'bulletList', 'callout', 'codeBlock', 'heading',
  'listItem', 'orderedList', 'paragraph', 'table', 'tableCell',
  'tableHeader', 'tableOfContents', 'tableRow', 'taskItem', 'taskList',
])

function isJSONContent(value: unknown): value is JSONContent {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value)
    && (value as { type?: unknown }).type === 'doc')
}

function isLegacyHTML(value: string): boolean {
  return /<(?:p|div|span|br|hr|h[1-6]|ul|ol|li|blockquote|aside|nav|pre|code|strong|em|b|i|s|a|img|table|thead|tbody|tr|th|td)(?:\s[^>]*)?>/i
    .test(value)
}

function plainTextDocument(value: string): JSONContent {
  const lines = value.replace(/\r\n/g, '\n').split('\n')
  return {
    type: 'doc',
    content: lines.map((line) => ({
      type: 'paragraph',
      ...(line ? { content: [{ type: 'text', text: line }] } : {}),
    })),
  }
}

export function parseRichTextValue(
  value: string,
  serialization: RichTextSerialization,
): string | JSONContent {
  if (serialization === 'html') return value
  if (!value.trim()) return EMPTY_DOCUMENT
  try {
    const parsed: unknown = JSON.parse(value)
    if (isJSONContent(parsed)) return parsed
  } catch {
    // Legacy plain text is preserved below as a Tiptap paragraph.
  }
  // Tiptap can import legacy HTML directly. It is only serialized to canonical
  // JSON after a real editor update, so opening/saving an untouched legacy
  // record does not rewrite its backend fact.
  if (isLegacyHTML(value)) return value
  return plainTextDocument(value)
}

export function serializeRichTextValue(
  editor: { getHTML: () => string; getJSON: () => JSONContent },
  serialization: RichTextSerialization,
): string {
  return serialization === 'json'
    ? JSON.stringify(editor.getJSON())
    : editor.getHTML()
}

export function canonicalRichTextValue(
  value: string,
  serialization: RichTextSerialization,
): string {
  const parsed = parseRichTextValue(value, serialization)
  return serialization === 'json' ? JSON.stringify(parsed) : String(parsed)
}

function jsonContentPlainText(node: JSONContent): string {
  if (node.type === 'hardBreak') return '\n'
  if (node.type === 'image') return '\n[图片]\n'
  if (node.type === 'tableOfContents') {
    const items = Array.isArray(node.attrs?.items) ? node.attrs.items : []
    return items.map((item) => (
      item && typeof item === 'object' ? String((item as { text?: unknown }).text || '') : ''
    )).filter(Boolean).join('\n')
  }
  if (node.type === 'unsupportedBlock') {
    try {
      const original: unknown = JSON.parse(String(node.attrs?.original || ''))
      return isJSONContent(original) || (original && typeof original === 'object')
        ? jsonContentPlainText(original as JSONContent)
        : ''
    } catch {
      return ''
    }
  }
  if (node.type === 'mention') {
    const label = String(node.attrs?.label || node.attrs?.username || node.attrs?.id || '').trim()
    return label ? `@${label}` : ''
  }
  if (node.text) return node.text
  const content = (node.content || []).map(jsonContentPlainText).join('')
  return RICH_TEXT_BLOCK_TYPES.has(node.type || '') ? `${content}\n` : content
}

function normalizedRichTextPlainText(value: string): string {
  const source = value.trim()
  if (!source) return ''
  let document: unknown
  try {
    document = JSON.parse(source)
  } catch {
    return source
  }
  if (!isJSONContent(document)) return source
  return jsonContentPlainText(document)
    .replace(/\u00a0/g, ' ')
    .split('\n')
    .map((line) => line.trim())
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/** Counts user-visible Unicode characters rather than serialized JSON bytes. */
export function richTextVisibleCharacterCount(value: string): number {
  return Array.from(normalizedRichTextPlainText(value)).length
}

function jsonContentHasVisibleContent(node: JSONContent): boolean {
  if (node.type === 'image' && String(node.attrs?.src || '').trim()) return true
  if (node.type === 'tableOfContents' && Array.isArray(node.attrs?.items)) {
    return node.attrs.items.some((item) => (
      item && typeof item === 'object' && String((item as { text?: unknown }).text || '').trim()
    ))
  }
  if (node.type === 'unsupportedBlock' && String(node.attrs?.original || '').trim()) return true
  if (node.type === 'mention'
    && String(node.attrs?.label || node.attrs?.username || node.attrs?.id || '').trim()) return true
  if (node.text?.trim()) return true
  return Boolean(node.content?.some(jsonContentHasVisibleContent))
}

function htmlHasVisibleContent(value: string): boolean {
  if (/<img\b/i.test(value)) return true
  return Boolean(
    value
      .replace(/<[^>]+>/g, ' ')
      .replace(/&nbsp;/gi, ' ')
      .replace(/\s+/g, ' ')
      .trim(),
  )
}

export function isRichTextEmpty(
  value: string,
  serialization: RichTextSerialization = 'html',
): boolean {
  const source = value.trim()
  if (!source) return true
  if (serialization === 'html') return !htmlHasVisibleContent(source)
  const parsed = parseRichTextValue(source, 'json')
  if (typeof parsed === 'string') return !htmlHasVisibleContent(parsed)
  return !jsonContentHasVisibleContent(parsed)
}
