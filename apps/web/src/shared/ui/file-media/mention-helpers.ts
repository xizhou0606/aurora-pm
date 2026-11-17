import { searchFieldsMatch, USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import type { Editor } from '@tiptap/core'
import type { RichTextMention } from './lazy-rich-text-field'

export type ActiveMention = {
  from: number
  to: number
  query: string
}

export function resolveEditorMention(editor: Editor): ActiveMention | null {
  const { selection } = editor.state
  if (!selection.empty) return null
  const { $from } = selection
  const prefix = $from.parent.textBetween(0, $from.parentOffset, undefined, '\ufffc')
  const match = prefix.match(/(^|\s)@([^\s@]*)$/u)
  if (!match) return null
  const query = match[2] || ''
  return {
    from: $from.pos - query.length - 1,
    to: $from.pos,
    query,
  }
}

export function filterMentions(options: readonly RichTextMention[], query: string, search: SearchOptions = USER_SEARCH) {
  return options.filter(option => searchFieldsMatch([option.label, option.username], query, search))
}

export function safeImageSource(value: string): string {
  try {
    const url = new URL(value, window.location.origin)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.toString() : ''
  } catch {
    return ''
  }
}