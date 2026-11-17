import type { Transaction } from '@tiptap/pm/state'
import { ReplaceStep } from '@tiptap/pm/transform'
import type { Fragment } from '@tiptap/pm/model'

function withoutEmptyEdges(content: Fragment) {
  let from = content.size
  let to = 0
  content.forEach((node, position) => {
    if (node.type.name !== 'paragraph' || node.content.size) {
      from = Math.min(from, position)
      to = position + node.nodeSize
    }
  })
  return to ? { content: content.cut(from, to), offset: from } : undefined
}

/** Preserve annotations inside moved blocks, including the inverse history steps. */
export function movedRichTextRange(tr: Transaction, from: number, to: number) {
  const steps = tr.steps
  const matches: { from: number; to: number }[] = []
  // History may also contain the appended trailing-paragraph transaction.
  for (let deletionIndex = 0; deletionIndex < steps.length; deletionIndex++) {
    const deletion = steps[deletionIndex]
    if (!(deletion instanceof ReplaceStep) || deletion.to <= deletion.from || deletion.slice.size) continue
    const deleted = tr.docs[deletionIndex].slice(deletion.from, deletion.to)
    if (deleted.openStart || deleted.openEnd || !deleted.content.firstChild?.isBlock) continue
    let source = deletion.from
    for (let index = deletionIndex - 1; index >= 0; index--) source = tr.mapping.maps[index].invert().map(source, 1)
    if (from <= source || to >= source + deleted.size) continue
    for (let insertionIndex = 0; insertionIndex < steps.length; insertionIndex++) {
      const insertion = steps[insertionIndex]
      if (!(insertion instanceof ReplaceStep) || insertion.from !== insertion.to || !insertion.slice.size
        || insertion.slice.openStart || insertion.slice.openEnd) continue
      // History merges the moved block and its appended empty trailing paragraph
      // into one deletion. Only ignore empty paragraphs, never unrelated content.
      const oldCore = withoutEmptyEdges(deleted.content)
      const newCore = withoutEmptyEdges(insertion.slice.content)
      if (!oldCore || !newCore || !oldCore.content.eq(newCore.content)) continue
      if (from <= source + oldCore.offset || to >= source + oldCore.offset + oldCore.content.size) continue
      const mapped = tr.mapping.slice(insertionIndex + 1)
      matches.push({
        from: mapped.map(insertion.from + newCore.offset + from - source - oldCore.offset, -1),
        to: mapped.map(insertion.from + newCore.offset + to - source - oldCore.offset, 1),
      })
    }
  }
  return matches.length === 1 ? matches[0] : undefined
}
