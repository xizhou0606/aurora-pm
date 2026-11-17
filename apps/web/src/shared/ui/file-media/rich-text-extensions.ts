import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import { TableKit } from '@tiptap/extension-table'
import TaskItem from '@tiptap/extension-task-item'
import TaskList from '@tiptap/extension-task-list'
import {
  Extension,
  Node,
  mergeAttributes,
  type Extensions,
  type JSONContent,
} from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import StarterKit from '@tiptap/starter-kit'

import type {
  RichTextDocumentLabels,
  RichTextDocumentSchemaVersion,
  RichTextPreset,
} from './rich-text-types'
import { RichTextCommentAnchors } from './rich-text-comment-anchors'
import { RichTextPhysicalControlHistoryShortcuts } from './rich-text-history-shortcuts'
import { RichTextBitableNode } from './rich-text-bitable-node'

const RichTextImageNode = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      assetId: {
        default: null,
        parseHTML: (element) => {
          const value = Number(element.getAttribute('data-asset-id') || 0)
          return Number.isSafeInteger(value) && value > 0 ? value : null
        },
        renderHTML: (attributes) => attributes.assetId
          ? { 'data-asset-id': String(attributes.assetId) }
          : {},
      },
    }
  },
})

export const RICH_TEXT_DOCUMENT_SCHEMA_VERSION = 3 as const

const FIELD_NODE_TYPES = new Set([
  'doc',
  'paragraph',
  'text',
  'heading',
  'bulletList',
  'orderedList',
  'listItem',
  'codeBlock',
  'blockquote',
  'hardBreak',
  'horizontalRule',
  'image',
  'mention',
])

const DOCUMENT_V2_NODE_TYPES = new Set([
  ...FIELD_NODE_TYPES,
  'taskList',
  'taskItem',
  'table',
  'tableRow',
  'tableCell',
  'tableHeader',
  'callout',
  'tableOfContents',
  'unsupportedBlock',
])

export const RichTextMentionNode = Node.create({
  name: 'mention',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: false,
  addAttributes() {
    return {
      id: { default: null },
      label: { default: '' },
      username: { default: '' },
    }
  },
  parseHTML() {
    return [{ tag: 'span[data-comment-mention]' }]
  },
  renderHTML({ node, HTMLAttributes }) {
    const label = String(node.attrs.label || node.attrs.username || node.attrs.id || '')
    return [
      'span',
      mergeAttributes(HTMLAttributes, {
        'data-comment-mention': '',
        'data-user-id': node.attrs.id,
        'data-user-username': node.attrs.username,
        tabindex: 0,
        class: 'rounded bg-primary/10 px-0.5 font-medium text-primary',
      }),
      `@${label}`,
    ]
  },
  renderText({ node }) {
    return `@${String(node.attrs.label || node.attrs.username || node.attrs.id || '')}`
  },
})

const RichTextCalloutNode = Node.create({
  name: 'callout',
  group: 'block',
  content: 'inline*',
  defining: true,
  addAttributes() {
    return {
      tone: {
        default: 'info',
        parseHTML: (element) => element.getAttribute('data-callout-tone') || 'info',
      },
    }
  },
  parseHTML() {
    return [{ tag: 'aside[data-rich-text-callout]' }]
  },
  renderHTML({ HTMLAttributes }) {
    return [
      'aside',
      mergeAttributes(HTMLAttributes, {
        'data-rich-text-callout': '',
        'data-callout-tone': HTMLAttributes.tone || 'info',
      }),
      0,
    ]
  },
})

type TableOfContentsItem = {
  anchor: string
  level: number
  text: string
}

function tocItems(document: JSONContent): TableOfContentsItem[] {
  const items: TableOfContentsItem[] = []
  let headingIndex = 0
  const visit = (node: JSONContent) => {
    if (node.type === 'heading') {
      headingIndex += 1
      const text = (node.content || []).map((child) => child.text || '').join('').trim()
      items.push({
        anchor: String(node.attrs?.id || `section-${headingIndex}`),
        level: Number(node.attrs?.level || 1),
        text,
      })
    }
    node.content?.forEach(visit)
  }
  visit(document)
  return items
}

function sameTOCItems(left: unknown, right: TableOfContentsItem[]): boolean {
  return JSON.stringify(left || []) === JSON.stringify(right)
}

function headingDecorations(document: Parameters<typeof DecorationSet.create>[0]) {
  const decorations: Decoration[] = []
  let headingIndex = 0
  document.descendants((node, position) => {
    if (node.type.name !== 'heading') return
    headingIndex += 1
    decorations.push(Decoration.node(
      position,
      position + node.nodeSize,
      { id: `section-${headingIndex}` },
    ))
  })
  return DecorationSet.create(document, decorations)
}

function renderTOCNode(
  node: { attrs: Record<string, unknown> },
  labels: RichTextDocumentLabels['tableOfContents'],
) {
  const dom = document.createElement('nav')
  dom.dataset.richTextTableOfContents = ''
  dom.setAttribute('aria-label', labels.label)

  const render = (items: unknown) => {
    dom.replaceChildren()
    const resolved = Array.isArray(items) ? items as TableOfContentsItem[] : []
    if (!resolved.length) {
      const empty = document.createElement('span')
      empty.textContent = labels.empty
      dom.append(empty)
      return
    }
    const list = document.createElement('ol')
    for (const item of resolved) {
      const entry = document.createElement('li')
      entry.dataset.level = String(item.level)
      const link = document.createElement('a')
      link.href = `#${encodeURIComponent(item.anchor)}`
      link.textContent = item.text
      entry.append(link)
      list.append(entry)
    }
    dom.append(list)
  }

  render(node.attrs.items)
  return {
    dom,
    update: (nextNode: typeof node) => {
      render(nextNode.attrs.items)
      return true
    },
    stopEvent: (event: Event) => event.type === 'click',
  }
}

function createTableOfContentsNode(labels: RichTextDocumentLabels['tableOfContents']) {
  return Node.create({
    name: 'tableOfContents',
    group: 'block',
    atom: true,
    selectable: true,
    addAttributes() {
      return { items: { default: [] } }
    },
    parseHTML() {
      return [{ tag: 'nav[data-rich-text-table-of-contents]' }]
    },
    renderHTML({ HTMLAttributes }) {
      const { items: _items, ...attributes } = HTMLAttributes
      return [
        'nav',
        mergeAttributes(attributes, {
          'aria-label': labels.label,
          'data-rich-text-table-of-contents': '',
        }),
        labels.empty,
      ]
    },
    addNodeView() {
      return ({ node }) => renderTOCNode(node, labels)
    },
  })
}

function createUnsupportedBlockNode(label: RichTextDocumentLabels['unsupportedBlock']) {
  return Node.create({
    name: 'unsupportedBlock',
    group: 'block',
    atom: true,
    selectable: true,
    draggable: false,
    addAttributes() {
      return {
        original: { default: '' },
        originalType: { default: '' },
      }
    },
    parseHTML() {
      return [{ tag: 'div[data-rich-text-unsupported-block]' }]
    },
    renderHTML({ node, HTMLAttributes }) {
      const resolvedLabel = label(String(node.attrs.originalType || ''))
      return [
        'div',
        mergeAttributes(HTMLAttributes, {
          'aria-label': resolvedLabel,
          'data-rich-text-unsupported-block': '',
          role: 'note',
        }),
        resolvedLabel,
      ]
    },
  })
}

const tableOfContentsPluginKey = new PluginKey('richTextTableOfContents')

const RichTextTableOfContentsSync = Extension.create({
  name: 'richTextTableOfContentsSync',
  addProseMirrorPlugins() {
    return [new Plugin({
      key: tableOfContentsPluginKey,
      props: {
        decorations: (state) => headingDecorations(state.doc),
      },
      appendTransaction: (transactions, _oldState, newState) => {
        if (!transactions.some((transaction) => transaction.docChanged)) return null
        const items = tocItems(newState.doc.toJSON())
        let transaction = newState.tr
        let changed = false
        newState.doc.descendants((node, position) => {
          if (node.type.name !== 'tableOfContents' || sameTOCItems(node.attrs.items, items)) return
          transaction = transaction.setNodeMarkup(position, undefined, { ...node.attrs, items })
          changed = true
        })
        return changed ? transaction : null
      },
    })]
  },
})

export function richTextNodeTypes(
  preset: RichTextPreset,
  schemaVersion: RichTextDocumentSchemaVersion,
): ReadonlySet<string> {
  return preset === 'document' && schemaVersion >= 2
    ? schemaVersion >= 3 ? new Set([...DOCUMENT_V2_NODE_TYPES, 'bitable']) : DOCUMENT_V2_NODE_TYPES
    : FIELD_NODE_TYPES
}

export function createRichTextExtensions(input: {
  preset: RichTextPreset
  schemaVersion: RichTextDocumentSchemaVersion
  placeholder: string
  documentLabels?: RichTextDocumentLabels
  collaboration?: boolean
}): Extensions {
  const extensions: Extensions = [
    input.collaboration ? StarterKit.configure({ undoRedo: false }) : StarterKit,
    ...(input.collaboration ? [] : [RichTextPhysicalControlHistoryShortcuts]),
    RichTextImageNode.configure({ inline: false, allowBase64: false }),
    RichTextMentionNode,
    Placeholder.configure({ placeholder: input.placeholder }),
  ]

  if (input.preset !== 'document') {
    return extensions
  }

  extensions.push(RichTextCommentAnchors)

  if (input.schemaVersion < 2 || !input.documentLabels) return extensions

  return [
    ...extensions,
    TaskList,
    TaskItem.configure({ nested: true }),
    TableKit.configure({ table: { resizable: true, renderWrapper: true } }),
    RichTextCalloutNode,
    createTableOfContentsNode(input.documentLabels.tableOfContents),
    createUnsupportedBlockNode(input.documentLabels.unsupportedBlock),
    RichTextTableOfContentsSync,
    ...(input.schemaVersion >= 3 ? [RichTextBitableNode] : []),
  ]
}
