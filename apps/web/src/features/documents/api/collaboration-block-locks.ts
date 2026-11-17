import { Extension } from '@tiptap/core'
import { Plugin, PluginKey, type EditorState } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import { relativePositionToAbsolutePosition, ySyncPluginKey } from '@tiptap/y-tiptap'
import * as Y from 'yjs'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { collaborationCursor } from './collaboration-cursor'

type Awareness = Parameters<typeof collaborationCursor>[0]
const key = new PluginKey('documentBlockLocks')

function lockedBlocks(state: EditorState, awareness: Awareness) {
  const sync = ySyncPluginKey.getState(state)
  const blocks: { from: number; to: number; name: string }[] = []
  if (!sync?.binding) return blocks
  for (const [clientId, remote] of awareness.getStates()) {
    if (clientId === awareness.clientID || remote.active === false || !remote.editingBlock) continue
    if (typeof remote.user?.displayName !== 'string') continue
    try {
      const position = relativePositionToAbsolutePosition(sync.doc, sync.type,
        Y.createRelativePositionFromJSON(remote.editingBlock), sync.binding.mapping)
      if (position === null || position < 0 || position > state.doc.content.size) continue
      const resolved = state.doc.resolve(position)
      for (let depth = resolved.depth; depth > 0; depth -= 1) {
        const node = resolved.node(depth)
        if (!node.isTextblock) continue
        const from = resolved.before(depth)
        blocks.push({ from, to: from + node.nodeSize, name: remote.user.displayName })
        break
      }
    } catch { /* Malformed or removed awareness anchors never affect content. */ }
  }
  return blocks
}

export function collaborationBlockLocks(awareness: Awareness) {
  return Extension.create({
    name: 'documentBlockLocks',
    addProseMirrorPlugins() {
      return [new Plugin({
        key,
        filterTransaction(transaction, state) {
          // Remote CRDT updates have already passed the room's authoritative guard.
          if (!transaction.docChanged || transaction.getMeta(ySyncPluginKey)) return true
          return lockedBlocks(state, awareness).every(({ from }) => {
            const mapped = transaction.mapping.mapResult(from, 1)
            const before = state.doc.nodeAt(from)
            const after = transaction.doc.nodeAt(mapped.pos)
            return !mapped.deleted && Boolean(before && after && before.eq(after))
          })
        },
        props: {
          decorations(state) {
            return DecorationSet.create(state.doc, lockedBlocks(state, awareness).map(({ from, to, name }) =>
              Decoration.node(from, to, {
                class: 'bg-muted/40 outline outline-1 outline-border cursor-not-allowed',
                title: DOCUMENTS_COPY.editor.blockLockedBy(name),
                'data-collaboration-locked': 'true',
              })))
          },
        },
        view(view) {
          const update = () => {
            queueMicrotask(() => { if (!view.isDestroyed) view.dispatch(view.state.tr.setMeta(key, true)) })
          }
          awareness.on('change', update)
          return { destroy: () => awareness.off('change', update) }
        },
      })]
    },
  })
}
