import { Node, mergeAttributes } from '@tiptap/core'

export const CommentMention = Node.create({
  name: 'mention',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: false,
  addAttributes() {
    return {
      id: { default: null },
      label: { default: '' },
      username: { default: '' },
    }
  },
  parseHTML() {
    return [{ tag: 'span[data-comment-mention]' }]
  },
  renderHTML({ node, HTMLAttributes }) {
    const label = String(node.attrs.label || node.attrs.username || node.attrs.id || '')
    return [
      'span',
      mergeAttributes(HTMLAttributes, {
        'data-comment-mention': '',
        'data-user-id': node.attrs.id,
        'data-user-username': node.attrs.username,
        tabindex: 0,
        class: 'rounded bg-primary/10 px-0.5 font-medium text-primary',
      }),
      `@${label}`,
    ]
  },
  renderText({ node }) {
    return `@${String(node.attrs.label || node.attrs.username || node.attrs.id || '')}`
  },
})