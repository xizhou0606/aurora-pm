import { Extension, type Editor } from '@tiptap/core'
import type { Node as ProseMirrorNode } from '@tiptap/pm/model'
import { Plugin, PluginKey, type Transaction } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'

import type {
  RichTextCommentAnchor,
  RichTextCommentAnchorId,
  RichTextCommentAnchorStatus,
  RichTextSelectionSnapshot,
} from './rich-text-types'
import { scrollRichTextElementWithinDocument } from './rich-text-document-navigation'
import { movedRichTextRange } from './rich-text-block-move'

type CommentAnchorPluginState = {
  anchors: readonly RichTextCommentAnchor[]
  sourceAnchors: readonly RichTextCommentAnchor[]
  currentRevision?: number
  activeAnchorId?: RichTextCommentAnchorId
  decorations: DecorationSet
}

type CommentAnchorPluginUpdate = {
  anchors: readonly RichTextCommentAnchor[]
  reset?: boolean
  currentRevision?: number
  activeAnchorId?: RichTextCommentAnchorId
}

type CommentAnchorStorage = {
  onSelect?: (threadId: RichTextCommentAnchorId) => void
}

export type RichTextCommentAnchorResolution = {
  anchor: RichTextCommentAnchor
  status: RichTextCommentAnchorStatus
}

const commentAnchorPluginKey = new PluginKey<CommentAnchorPluginState>('richTextCommentAnchors')

export function richTextInlineLeafText(node: {
  type: { name: string }
  attrs: Record<string, unknown>
}): string {
  if (node.type.name === 'hardBreak') return '\n'
  if (node.type.name !== 'mention') return ''
  const label = [node.attrs.label, node.attrs.username, node.attrs.id]
    .find((value) => typeof value === 'string' && value.trim())
  return label ? `@${String(label).trim()}` : ''
}

function selectionRangeIsValid(document: ProseMirrorNode, selection: RichTextSelectionSnapshot): boolean {
  const { from, to } = selection
  if (!Number.isSafeInteger(from) || !Number.isSafeInteger(to) || from < 1 || to <= from) return false
  if (to > document.content.size) return false

  try {
    const $from = document.resolve(from)
    const $to = document.resolve(to)
    return $from.parent === $to.parent && ['paragraph', 'heading'].includes($from.parent.type.name)
  } catch {
    return false
  }
}

function mapCommentAnchors(
  transaction: Transaction,
  anchors: readonly RichTextCommentAnchor[],
): readonly RichTextCommentAnchor[] {
  return anchors.map((anchor) => {
    if (anchor.status === 'orphan') return anchor
    const moved = movedRichTextRange(transaction, anchor.selection.from, anchor.selection.to)
    const from = moved?.from ?? transaction.mapping.map(anchor.selection.from, -1)
    const to = moved?.to ?? transaction.mapping.map(anchor.selection.to, 1)
    return {
      ...anchor,
      status: to > from ? anchor.status : 'orphan',
      selection: { ...anchor.selection, from, to },
    }
  })
}

function reconcileCommentAnchors(
  previous: readonly RichTextCommentAnchor[],
  sourceAnchors: readonly RichTextCommentAnchor[],
  incoming: readonly RichTextCommentAnchor[],
  currentRevision: number | undefined,
): readonly RichTextCommentAnchor[] {
  if (!Number.isSafeInteger(currentRevision) || Number(currentRevision) <= 0) return incoming
  return incoming.map((anchor) => {
    const source = sourceAnchors.find((candidate) => String(candidate.id) === String(anchor.id))
    // React may replay fresh object instances of the same server snapshot after
    // every local edit. That is not a new authoritative position: retain the
    // transaction-mapped range until the server actually supplies a new snapshot.
    const sameSource = source?.revision === anchor.revision
      && source.selection.from === anchor.selection.from && source.selection.to === anchor.selection.to
      && source.selection.quote === anchor.selection.quote
      && source.selection.prefix === anchor.selection.prefix && source.selection.suffix === anchor.selection.suffix
    if (anchor.revision === currentRevision && !sameSource) return anchor
    const mapped = previous.find((candidate) => String(candidate.id) === String(anchor.id))
    if (!mapped) return anchor
    return {
      ...anchor,
      revision: Number(currentRevision),
      status: mapped.status === 'orphan' ? 'orphan' : anchor.status,
      selection: {
        ...anchor.selection,
        from: mapped.selection.from,
        to: mapped.selection.to,
      },
    }
  })
}

export function resolveRichTextCommentAnchor(
  document: ProseMirrorNode,
  currentRevision: number | undefined,
  anchor: RichTextCommentAnchor,
): RichTextCommentAnchorResolution {
  const currentRevisionValid = Number.isSafeInteger(currentRevision) && Number(currentRevision) > 0
  const anchorRevisionValid = Number.isSafeInteger(anchor.revision) && anchor.revision > 0
  if (
    anchor.status === 'orphan'
    || !currentRevisionValid
    || !anchorRevisionValid
    || anchor.revision !== currentRevision
    || !selectionRangeIsValid(document, anchor.selection)
  ) {
    return { anchor, status: 'orphan' }
  }
  return { anchor, status: anchor.status }
}

function decorationsFor(
  document: ProseMirrorNode,
  currentRevision: number | undefined,
  anchors: readonly RichTextCommentAnchor[],
  activeAnchorId?: RichTextCommentAnchorId,
): DecorationSet {
  const groups = new Map<string, RichTextCommentAnchorResolution[]>()
  for (const anchor of anchors) {
    const resolution = resolveRichTextCommentAnchor(document, currentRevision, anchor)
    if (resolution.status === 'orphan') continue
    const key = `${anchor.selection.from}:${anchor.selection.to}`
    groups.set(key, [...(groups.get(key) || []), resolution])
  }
  const decorations = Array.from(groups.values()).map((group) => {
    const selected = group.find(({ anchor }) => String(anchor.id) === String(activeAnchorId ?? ''))
      ?? group.find(({ status }) => status === 'active')
      ?? group[0]
    const ids = group.map(({ anchor }) => String(anchor.id))
    const focused = ids.includes(String(activeAnchorId ?? ''))
    const status = focused
      ? selected.status
      : group.some((resolution) => resolution.status === 'active') ? 'active' : 'resolved'
    return Decoration.inline(selected.anchor.selection.from, selected.anchor.selection.to, {
      'aria-label': selected.anchor.label,
      'data-rich-text-comment-anchor': '',
      'data-rich-text-comment-anchor-id': String(selected.anchor.id),
      'data-rich-text-comment-anchor-ids': JSON.stringify(ids),
      'data-comment-anchor-state': status,
      ...(focused
        ? { 'data-comment-anchor-focused': '' }
        : {}),
      role: 'button',
      tabindex: '0',
    }, {
      inclusiveStart: false,
      inclusiveEnd: false,
    })
  })
  return DecorationSet.create(document, decorations)
}

function anchorFromEvent(
  event: Event,
  anchors: readonly RichTextCommentAnchor[],
): RichTextCommentAnchor | undefined {
  const target = event.target instanceof Element
    ? event.target.closest<HTMLElement>('[data-rich-text-comment-anchor-id]')
    : null
  if (!target) return undefined
  const id = target.dataset.richTextCommentAnchorId
  return anchors.find((anchor) => String(anchor.id) === id)
}

function syncFocusedCommentAnchor(
  root: HTMLElement,
  activeAnchorId: RichTextCommentAnchorId | undefined,
): void {
  const activeId = activeAnchorId === undefined ? undefined : String(activeAnchorId)
  for (const marker of root.querySelectorAll<HTMLElement>('[data-rich-text-comment-anchor]')) {
    let ids: unknown = []
    try {
      ids = JSON.parse(marker.dataset.richTextCommentAnchorIds || '[]')
    } catch {
      ids = [marker.dataset.richTextCommentAnchorId]
    }
    marker.toggleAttribute(
      'data-comment-anchor-focused',
      activeId !== undefined && Array.isArray(ids) && ids.includes(activeId),
    )
  }
}

export const RichTextCommentAnchors = Extension.create<Record<string, never>, CommentAnchorStorage>({
  name: 'richTextCommentAnchors',
  addStorage() {
    return {}
  },
  addProseMirrorPlugins() {
    const storage = this.storage
    return [new Plugin<CommentAnchorPluginState>({
      key: commentAnchorPluginKey,
      view: () => ({
        update: (view) => {
          syncFocusedCommentAnchor(
            view.dom,
            commentAnchorPluginKey.getState(view.state)?.activeAnchorId,
          )
        },
      }),
      state: {
        init: (_, state) => ({
          anchors: [],
          sourceAnchors: [],
          currentRevision: undefined,
          decorations: DecorationSet.empty,
        }),
        apply: (transaction, previous, _oldState, newState) => {
          const update = transaction.getMeta(commentAnchorPluginKey) as CommentAnchorPluginUpdate | undefined
          if (!update && !transaction.docChanged) return previous
          const anchors = update
            ? update.reset ? update.anchors : reconcileCommentAnchors(previous.anchors, previous.sourceAnchors, update.anchors, update.currentRevision)
            : mapCommentAnchors(transaction, previous.anchors)
          const currentRevision = update ? update.currentRevision : previous.currentRevision
          const activeAnchorId = update ? update.activeAnchorId : previous.activeAnchorId
          return {
            anchors,
            sourceAnchors: update ? update.anchors : previous.sourceAnchors,
            currentRevision,
            activeAnchorId,
            decorations: decorationsFor(newState.doc, currentRevision, anchors, activeAnchorId),
          }
        },
      },
      props: {
        decorations: (state) => commentAnchorPluginKey.getState(state)?.decorations,
        handleDOMEvents: {
          click: (view, event) => {
            const state = commentAnchorPluginKey.getState(view.state)
            const anchor = state ? anchorFromEvent(event, state.anchors) : undefined
            // A short drag can finish on an annotated range and still emit a
            // click. Preserve the native text selection instead of opening a
            // thread from that trailing click.
            if (!view.state.selection.empty || !anchor || !storage.onSelect) return false
            event.preventDefault()
            storage.onSelect(anchor.id)
            return true
          },
          keydown: (view, event) => {
            if (event.key !== 'Enter' && event.key !== ' ') return false
            const state = commentAnchorPluginKey.getState(view.state)
            const anchor = state ? anchorFromEvent(event, state.anchors) : undefined
            if (!anchor || !storage.onSelect) return false
            event.preventDefault()
            storage.onSelect(anchor.id)
            return true
          },
        },
      },
    })]
  },
})

export function updateRichTextCommentAnchors(
  editor: Editor,
  input: CommentAnchorPluginUpdate & {
    onSelect?: (threadId: RichTextCommentAnchorId) => void
  },
): void {
  const storage = (editor.storage as unknown as Record<string, unknown>)
    .richTextCommentAnchors as CommentAnchorStorage | undefined
  if (!storage || editor.isDestroyed) return
  storage.onSelect = input.onSelect
  editor.view.dispatch(
    editor.state.tr
      .setMeta(commentAnchorPluginKey, {
        anchors: input.anchors,
        reset: input.reset,
        currentRevision: input.currentRevision,
        activeAnchorId: input.activeAnchorId,
      } satisfies CommentAnchorPluginUpdate)
      .setMeta('addToHistory', false),
  )
}

export function focusRichTextCommentAnchor(
  editor: Editor,
  anchorId: RichTextCommentAnchorId,
): boolean {
  if (editor.isDestroyed) return false
  const marker = Array.from(
    editor.view.dom.querySelectorAll<HTMLElement>('[data-rich-text-comment-anchor-id]'),
  ).find((candidate) => {
    try {
      const ids = JSON.parse(candidate.dataset.richTextCommentAnchorIds || '[]')
      return Array.isArray(ids) && ids.includes(String(anchorId))
    } catch {
      return candidate.dataset.richTextCommentAnchorId === String(anchorId)
    }
  })
  if (!marker) return false
  scrollRichTextElementWithinDocument(editor, marker, { behavior: 'smooth', block: 'center' })
  return true
}
