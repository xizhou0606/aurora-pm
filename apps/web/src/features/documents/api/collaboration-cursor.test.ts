import { describe, expect, it, vi, type Mock, mock } from 'bun:test'
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import { yCursorPlugin, yCursorPluginKey } from '@tiptap/y-tiptap'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import { Plugin } from '@tiptap/pm/state'
import * as Y from 'yjs'
import { collaborationCursor } from './collaboration-cursor'
import { collaborationColorsFor, localCollaborationColor } from '@/shared/lib/collaboration-colors'

const realYCursorPlugin = yCursorPlugin

mock.module('@tiptap/y-tiptap', () => ({
  yCursorPlugin: vi.fn(realYCursorPlugin),
}))

describe('document collaboration cursor', () => {
  it('registers the cursor plugin without replacing the document editor', async () => {
    const document = new Y.Doc()
    const local = { id: 'local', displayName: '自己' }
    const remote = { id: 'remote', displayName: '<script>远端用户</script>' }
    const other = { id: 'other', displayName: '另一位用户' }
    const publish = vi.fn()
    const localState: Record<string, unknown> = { user: local }
    const awareness = {
      clientID: 1,
      getStates: () => new Map([[1, { user: local }], [2, { user: remote }], [3, { user: other }]]),
      getLocalState: () => localState,
      setLocalStateField: publish,
      on: () => undefined,
      off: () => undefined,
    } as unknown as Parameters<typeof collaborationCursor>[0]
    const editor = new Editor({ extensions: [StarterKit.configure({ undoRedo: false }), Collaboration.configure({ document }), collaborationCursor(awareness)] })
    expect(editor.state.plugins.some((plugin) => plugin.spec.key === yCursorPluginKey)).toBe(true)
    expect(editor.extensionManager.extensions.some((extension) => extension.name === 'documentCollaborationCursor')).toBe(true)
    const options = ((yCursorPlugin) as unknown as Mock<typeof yCursorPlugin>).mock.calls.at(-1)![1]!
    expect(options.awarenessStateFilter!(999, 1, { user: local })).toBe(false)
    expect(options.awarenessStateFilter!(999, 2, { user: remote, active: false })).toBe(false)
    expect(options.awarenessStateFilter!(999, 2, { user: remote })).toBe(true)
    const colors = collaborationColorsFor(['local', 'remote', 'other'], 'local')
    const cursor = options.cursorBuilder!(remote, 2)
    expect(cursor.className).toContain(colors.get('remote')!.caret)
    expect(cursor.firstElementChild?.className).toContain(colors.get('remote')!.label)
    expect(cursor.textContent).toBe(`\u2060${remote.displayName}\u2060`)
    expect(options.cursorBuilder!(remote, 2)).toBe(cursor)
    expect(cursor.className).toContain('-ml-px -mr-px')
    expect(cursor.querySelector('script')).toBeNull()
    expect(options.selectionBuilder!(remote, 2).class).toBe(colors.get('remote')!.selection)
    expect(options.selectionBuilder!(other, 3).class).not.toBe(colors.get('remote')!.selection)
    expect(options.selectionBuilder!(local, 1).class).toBe(localCollaborationColor.selection)
    editor.commands.setContent('<p>first</p><p>second</p>')
    const plugin = editor.state.plugins.find((value) => value.spec.key === yCursorPluginKey)!
    const widget = DecorationSet.create(editor.state.doc, [Decoration.widget(9, cursor)])
    const split = editor.state.tr.split(2)
    const mapped = plugin.spec.state!.apply.call(plugin, split, widget, editor.state, editor.state)
    expect(mapped.find()).toHaveLength(1)
    expect(mapped.find()[0].from).toBe(split.mapping.map(9, 1))
    localState.cursor = { anchor: {}, head: {} }
    publish.mockClear()
    editor.registerPlugin(new Plugin({}))
    await Promise.resolve()
    expect(publish).not.toHaveBeenCalledWith('cursor', null)
    editor.destroy()
    await Promise.resolve()
    expect(publish).toHaveBeenCalledWith('cursor', null)
    document.destroy()
  })
})
