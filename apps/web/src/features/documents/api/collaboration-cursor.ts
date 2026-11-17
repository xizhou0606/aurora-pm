import { Extension } from '@tiptap/core'
import { Plugin, type StateField } from '@tiptap/pm/state'
import type { DecorationSet, EditorView } from '@tiptap/pm/view'
import { yCursorPlugin, ySyncPluginKey } from '@tiptap/y-tiptap'
import { collaborationColorsFor, localCollaborationColor } from '@/shared/lib/collaboration-colors'

type CursorAwareness = Parameters<typeof yCursorPlugin>[0]

function projectedName(value: unknown): string | undefined {
  if (!value || typeof value !== 'object') return undefined
  const user = value as Record<string, unknown>
  return typeof user.id === 'string' && user.id.length > 0
    && typeof user.displayName === 'string' && user.displayName.length > 0
    && user.displayName.length <= 220 ? user.displayName : undefined
}

export function collaborationCursor(awareness: CursorAwareness) {
  let rebuildingView = false
  // Adapt only writes; all reads/events remain on the provider's one Awareness.
  const cursorAwareness = new Proxy(awareness, {
    get(target, property) {
      if (property === 'setLocalStateField') return (field: string, value: unknown) => {
        const current = target.getLocalState()
        if (field === 'cursor') {
          if (rebuildingView || (current?.active === false && value !== null)) return
          if (value == null && current?.cursor == null) return
        }
        target.setLocalStateField(field, value)
      }
      const value = Reflect.get(target, property, target)
      return typeof value === 'function' ? value.bind(target) : value
    },
  })
  // Reuse widgets across awareness updates without taking space from the text.
  const cursors = new Map<number, HTMLElement>()
  const colorFor = (user: { id?: string }) => {
    const ids = [...awareness.getStates().values()]
      .flatMap((state) => projectedName(state.user) ? [String(state.user.id)] : [])
    const self = awareness.getLocalState()?.user
    const selfId = projectedName(self) ? String(self.id) : undefined
    return collaborationColorsFor([...ids, ...(user.id ? [user.id] : [])], selfId).get(user.id ?? '')
      ?? localCollaborationColor
  }
  return Extension.create({
    name: 'documentCollaborationCursor',
    addProseMirrorPlugins() {
      const cursorPlugin = yCursorPlugin(cursorAwareness, {
        awarenessStateFilter: (_local, remote, state) => awareness.clientID !== remote
          && state?.active !== false && Boolean(projectedName(state?.user)),
        cursorBuilder: (user, clientId) => {
          const color = colorFor(user)
          const cursor = cursors.get(clientId) ?? document.createElement('span')
          cursor.className = `relative pointer-events-none border-l-2 -ml-px -mr-px ${color.caret}`
          cursor.setAttribute('aria-hidden', 'true')
          const label = cursor.firstElementChild ?? document.createElement('span')
          label.className = `absolute bottom-full left-0 whitespace-nowrap rounded-t px-1 text-xs font-medium ${color.label}`
          const name = projectedName(user) ?? ''
          if (label.textContent !== name) label.textContent = name
          if (!cursor.firstElementChild) cursor.append(document.createTextNode('\u2060'), label, document.createTextNode('\u2060'))
          cursors.set(clientId, cursor)
          return cursor
        },
        selectionBuilder: (user) => ({ class: colorFor(user).selection }),
      })
      const cursorState = cursorPlugin.spec.state! as StateField<DecorationSet>
      const createView = cursorPlugin.spec.view!
      cursorPlugin.spec.view = (view: EditorView) => {
        const original = createView(view)
        return {
          ...original,
          destroy() {
            // PM destroys every plugin view when any plugin is registered.
            // That is not a user blur or a provider/session disconnect.
            rebuildingView = true
            try { original.destroy?.() } finally { rebuildingView = false }
            queueMicrotask(() => {
              if (view.isDestroyed && awareness.getLocalState()?.cursor != null) {
                awareness.setLocalStateField('cursor', null)
              }
            })
          },
        }
      }
      const apply = cursorState.apply
      cursorState.apply = function (transaction, previous, oldState, state) {
        // The local PM document leads the Yjs mapping during structural edits.
        // Mapping the existing widgets preserves valid peers instead of hiding
        // every name until a later awareness packet arrives.
        if (transaction.docChanged && !ySyncPluginKey.getState(state)?.isChangeOrigin) {
          return previous.map(transaction.mapping, transaction.doc)
        }
        return apply.call(this, transaction, previous, oldState, state)
      }
      return [cursorPlugin, new Plugin({ props: { attributes: { class: 'caret-primary' } } })]
    },
  })
}
