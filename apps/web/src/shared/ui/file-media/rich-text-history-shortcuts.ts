import { Extension, type Editor } from '@tiptap/core'
import { Plugin } from '@tiptap/pm/state'

function handlePhysicalControlHistoryShortcut(editor: Editor, event: KeyboardEvent): boolean {
  if (!event.ctrlKey || event.metaKey || event.altKey || event.isComposing) return false
  const key = event.key.toLocaleLowerCase()
  const code = event.code
  const isUndoKey = code === 'KeyZ' || key === 'z'
  const isRedoKey = code === 'KeyY' || key === 'y'
  let handled = false
  if (isUndoKey) {
    handled = event.shiftKey ? editor.commands.redo() : editor.commands.undo()
  } else if (isRedoKey && !event.shiftKey) {
    handled = editor.commands.redo()
  }
  if (handled) event.preventDefault()
  return handled
}

export const RichTextPhysicalControlHistoryShortcuts = Extension.create({
  name: 'richTextPhysicalControlHistoryShortcuts',
  addProseMirrorPlugins() {
    return [new Plugin({
      props: {
        handleKeyDown: (_view, event) => handlePhysicalControlHistoryShortcut(this.editor, event),
      },
    })]
  },
})
