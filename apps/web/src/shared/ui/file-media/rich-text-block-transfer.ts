import { Fragment, Slice } from '@tiptap/pm/model'
import { closeHistory } from '@tiptap/pm/history'
import { NodeSelection, type EditorState } from '@tiptap/pm/state'
import { ReplaceStep } from '@tiptap/pm/transform'

// Closed slices deliberately bypass ProseMirror's contextual slice fitting.
// An incompatible destination must not turn a heading into a paragraph/list.
export function transferRichTextBlock(state: EditorState, from: number, boundary: number) {
  const node = state.doc.nodeAt(from)
  if (!node || !node.isBlock || boundary >= from && boundary <= from + node.nodeSize) return null
  const tr = closeHistory(state.tr)
  if (tr.maybeStep(new ReplaceStep(boundary, boundary, new Slice(Fragment.from(node), 0, 0))).failed) return null
  const source = tr.mapping.map(from, 1)
  const $source = tr.doc.resolve(source)
  const index = $source.index()
  if (!$source.parent.canReplace(index, index + 1)) {
    // Required containers (notably the last paragraph in a table cell) retain
    // schema-required empty content, without rewriting the moved node.
    const filler = $source.parent.contentMatchAt(index).fillBefore(Fragment.empty, true)
    if (!filler || index !== 0 || $source.parent.childCount !== 1) return null
    if (tr.maybeStep(new ReplaceStep(source + node.nodeSize, source + node.nodeSize, new Slice(filler, 0, 0))).failed) return null
  }
  if (tr.maybeStep(new ReplaceStep(source, source + node.nodeSize, Slice.empty)).failed) return null
  const destination = tr.mapping.slice(1).map(boundary, -1)
  if (!tr.doc.nodeAt(destination)?.eq(node)) return null
  return tr.setSelection(NodeSelection.create(tr.doc, destination)).setMeta('uiEvent', 'drop')
}
