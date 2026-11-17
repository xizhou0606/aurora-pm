import type { JSONContent } from '@tiptap/core'

export const QUALITY_REPRODUCTION_TEMPLATE_KEY = 'standard-reproduction'

function heading(text: string): JSONContent {
  return {
    type: 'heading',
    attrs: { level: 3 },
    content: [{ type: 'text', text }],
  }
}

function paragraph(text = ''): JSONContent {
  return {
    type: 'paragraph',
    ...(text ? { content: [{ type: 'text', text }] } : {}),
  }
}

export function qualityReproductionTemplatePayload(
  key = QUALITY_REPRODUCTION_TEMPLATE_KEY,
): string {
  if (key !== QUALITY_REPRODUCTION_TEMPLATE_KEY) return ''
  return JSON.stringify({
    type: 'doc',
    content: [
      heading('[步骤]'),
      paragraph('1. '),
      paragraph('2. '),
      heading('[实际结果]'),
      paragraph(),
      heading('[期望结果]'),
      paragraph(),
    ],
  } satisfies JSONContent)
}

function hasNodeContent(node: JSONContent): boolean {
  return Boolean(node.text?.trim()) || Boolean(node.content?.some(hasNodeContent))
}

export function isQualityRichTextEmpty(value: string): boolean {
  const source = value.trim()
  if (!source) return true
  try {
    const parsed = JSON.parse(source) as JSONContent
    return parsed.type === 'doc' ? !hasNodeContent(parsed) : false
  } catch {
    return false
  }
}

const BLOCK_NODE_TYPES = new Set([
  'blockquote',
  'bulletList',
  'codeBlock',
  'heading',
  'listItem',
  'orderedList',
  'paragraph',
])

type QualityRichTextPlainTextOptions = {
  imageLabel?: string
  imageText?: (url: string) => string
}

function richTextNodeText(
  node: JSONContent,
  options: QualityRichTextPlainTextOptions,
): string {
  if (node.type === 'hardBreak') return '\n'
  if (node.type === 'image') {
    const source = typeof node.attrs?.src === 'string' ? node.attrs.src.trim() : ''
    const imageText = options.imageText?.(source) || options.imageLabel || ''
    return imageText ? `\n${imageText}\n` : ''
  }
  if (node.text) return node.text
  const content = node.content?.map((child) => richTextNodeText(child, options)).join('') ?? ''
  return BLOCK_NODE_TYPES.has(node.type ?? '') ? `${content}\n` : content
}

export function qualityRichTextPlainText(
  value: string | undefined,
  options: QualityRichTextPlainTextOptions = {},
): string {
  const source = value?.trim() ?? ''
  if (!source) return ''
  try {
    const parsed = JSON.parse(source) as JSONContent
    if (parsed.type !== 'doc') return source
    return richTextNodeText(parsed, options)
      .replaceAll('\u00a0', ' ')
      .split('\n')
      .map((line) => line.trim())
      .join('\n')
      .replace(/\n{3,}/g, '\n\n')
      .trim()
  } catch {
    return source
  }
}
