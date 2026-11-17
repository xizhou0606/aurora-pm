import { describe, expect, it } from 'bun:test'
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import Collaboration from '@tiptap/extension-collaboration'
import * as Y from 'yjs'
import { collaborationBlockLocks } from './collaboration-block-locks'

describe('document block lock editing guard', () => {
  it('blocks typing, deletion and format changes in the remote block only', () => {
    const document = new Y.Doc()
    const states = new Map<number, Record<string, unknown>>()
    const awareness = { clientID: 1, getStates: () => states, on() {}, off() {} } as unknown as Parameters<typeof collaborationBlockLocks>[0]
    const editor = new Editor({ extensions: [StarterKit.configure({ undoRedo: false }),
      Collaboration.configure({ document }), collaborationBlockLocks(awareness)] })
    editor.commands.setContent('<p>one</p><p>two</p>')
    const block = document.getXmlFragment('default').get(0) as Y.XmlElement
    states.set(2, { user: { id: 'other', displayName: 'Other' }, editingBlock: Y.createRelativePositionFromTypeIndex(block, 0) })
    const original = editor.getJSON()
    editor.commands.insertContentAt(2, '!')
    expect(editor.getJSON()).toEqual(original)
    editor.commands.deleteRange({ from: 0, to: 5 })
    expect(editor.getJSON()).toEqual(original)
    editor.commands.setTextSelection(2)
    editor.commands.toggleHeading({ level: 2 })
    expect(editor.getJSON()).toEqual(original)
    editor.commands.insertContentAt(7, '!')
    expect(editor.getText()).toContain('t!wo')
    states.set(2, { ...states.get(2), active: false })
    editor.commands.insertContentAt(2, '!')
    expect(editor.getText()).toContain('o!ne')
    editor.destroy()
    document.destroy()
  })
})
