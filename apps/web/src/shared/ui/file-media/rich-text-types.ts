import type { Editor, Extensions } from '@tiptap/core'

/** Narrow bridge from a feature-owned collaboration session to the shared editor. */
export type RichTextCollaborationAdapter = {
  key: string
  extensions: Extensions
}

export type RichTextPastedImage = {
  src: string
  alt?: string
  assetId?: number
}

export type RichTextSelectionSnapshot = {
  from: number
  to: number
  quote: string
  prefix?: string
  suffix?: string
}

export type RichTextCommentAnchorStatus = 'active' | 'resolved' | 'orphan'

export type RichTextCommentAnchorId = string | number

export type RichTextCommentAnchor = {
  id: RichTextCommentAnchorId
  revision: number
  selection: RichTextSelectionSnapshot
  status: RichTextCommentAnchorStatus
  label: string
}

export type RichTextDocumentPositionOptions = {
  focus?: boolean
  scrollBehavior?: ScrollBehavior
}

export type RichTextMention = {
  id: number
  label: string
  username?: string
  avatarUrl?: string
}

export type RichTextImageDisplay = 'document' | 'thumbnail'

export type RichTextPreset = 'field' | 'document'

export type RichTextDocumentSchemaVersion = 1 | 2 | 3

export type RichTextSlashCommandId =
  | 'paragraph'
  | 'heading-1'
  | 'heading-2'
  | 'heading-3'
  | 'bullet-list'
  | 'ordered-list'
  | 'blockquote'
  | 'code-block'
  | 'task-list'
  | 'table'
  | 'callout'
  | 'table-of-contents'
  | 'bitable'

export interface RichTextDocumentLabels {
  slashMenu: {
    label: string
    empty: string
    commands: Record<RichTextSlashCommandId, {
      label: string
      description?: string
      keywords?: readonly string[]
    }>
  }
  bubbleMenu: {
    label: string
    bold: string
    italic: string
    strike: string
    code: string
    comment: string
  }
  blockMenu: {
    label: string
    copy: string
    delete: string
    moveUp: string
    moveDown: string
  }
  tableMenu: {
    label: string
    addRowBefore: string
    addRowAfter: string
    addColumnBefore: string
    addColumnAfter: string
    deleteRow: string
    deleteColumn: string
    mergeOrSplit: string
    toggleHeaderRow: string
    deleteTable: string
  }
  tableOfContents: {
    label: string
    empty: string
  }
  unsupportedBlock: (nodeType: string) => string
}

export interface RichTextCommandContext {
  editor: Editor
  range: { from: number; to: number }
}
