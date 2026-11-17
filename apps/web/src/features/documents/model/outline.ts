import type { JSONContent } from '@tiptap/core'

export type DocumentOutlineLevel = 1 | 2 | 3 | 4 | 5 | 6

export type DocumentOutlineTarget = {
  id: string
  anchor: string
  position: number
  level: DocumentOutlineLevel
  text: string
}

export type DocumentOutlineItem = {
  target: DocumentOutlineTarget
  children: readonly DocumentOutlineItem[]
}

type MutableOutlineItem = {
  target: DocumentOutlineTarget
  children: MutableOutlineItem[]
}

function record(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : undefined
}

function parseDocument(source: unknown): JSONContent | undefined {
  let value = source
  if (typeof source === 'string') {
    try {
      value = JSON.parse(source)
    } catch {
      return undefined
    }
  }
  const document = record(value)
  if (document?.type !== 'doc' || !Array.isArray(document.content)) return undefined
  return document as JSONContent
}

function outlineLevel(value: unknown): DocumentOutlineLevel {
  const parsed = Number(value)
  if (Number.isInteger(parsed) && parsed >= 1 && parsed <= 6) {
    return parsed as DocumentOutlineLevel
  }
  return 1
}

function inlineText(node: JSONContent): string {
  if (node.type === 'text') return node.text || ''
  if (node.type === 'mention') {
    return String(node.attrs?.label || node.attrs?.username || node.attrs?.id || '')
  }
  if (node.type === 'hardBreak') return ' '
  return (node.content || []).map(inlineText).join('')
}

const CONTAINER_NODE_TYPES = new Set([
  'blockquote',
  'bulletList',
  'callout',
  'codeBlock',
  'heading',
  'listItem',
  'orderedList',
  'paragraph',
  'table',
  'tableCell',
  'tableHeader',
  'tableRow',
  'taskItem',
  'taskList',
])

function nodeSize(node: JSONContent): number {
  if (node.type === 'text') return node.text?.length || 0
  if (Array.isArray(node.content) || CONTAINER_NODE_TYPES.has(node.type || '')) {
    // Empty ProseMirror containers commonly omit `content` in JSON, but still
    // consume their opening and closing positions.
    return 2 + (node.content || []).reduce((size, child) => size + nodeSize(child), 0)
  }
  return 1
}

function headingTargets(document: JSONContent): DocumentOutlineTarget[] {
  const targets: DocumentOutlineTarget[] = []
  let headingIndex = 0

  const visit = (node: JSONContent, position: number, isDocument = false) => {
    if (node.type === 'heading') {
      const text = inlineText(node).trim()
      if (text) {
        headingIndex += 1
        const anchor = String(node.attrs?.id || `section-${headingIndex}`)
        targets.push({
          id: `${anchor}:${position}`,
          anchor,
          position,
          level: outlineLevel(node.attrs?.level),
          text,
        })
      }
    }

    let childPosition = isDocument ? position : position + 1
    for (const child of node.content || []) {
      visit(child, childPosition)
      childPosition += nodeSize(child)
    }
  }

  visit(document, 0, true)
  return targets
}

export function deriveDocumentOutline(source: unknown): readonly DocumentOutlineItem[] {
  const document = parseDocument(source)
  if (!document) return []

  const roots: MutableOutlineItem[] = []
  const ancestors: MutableOutlineItem[] = []
  for (const target of headingTargets(document)) {
    const item: MutableOutlineItem = { target, children: [] }
    while (
      ancestors.length > 0
      && ancestors[ancestors.length - 1].target.level >= target.level
    ) {
      ancestors.pop()
    }
    const parent = ancestors[ancestors.length - 1]
    if (parent) parent.children.push(item)
    else roots.push(item)
    ancestors.push(item)
  }
  return roots
}
