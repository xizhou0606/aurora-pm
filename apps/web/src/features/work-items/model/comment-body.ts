import type { WorkItemUserOption } from './types'
import { WORK_TASK_COMMENT_COPY } from '@/shared/copy'
import type { ReplyRefMeta } from './comment-parser'

export type { CommentBodySegment, ReplyRefMeta } from './comment-parser'
export {
  TASK_ATTACH_IMAGE_RE,
  buildReplyExcerpt,
  extractCommentAttachmentIds,
  getReplyRefFromBody,
  isReplyComment,
  isRichTextCommentBody,
  parseCommentBodySegments,
  parseReplyQuoteLines,
  quoteLinesHtml,
  resolveReplyTargetCommentId,
  richTextCommentPlainText,
  stripCommentBodyText,
} from './comment-parser'

type RichTextNode = {
  type?: string
  text?: string
  attrs?: Record<string, unknown>
  content?: RichTextNode[]
}

function parsedRichTextComment(body: string): RichTextNode | null {
  const source = String(body || '').trim()
  if (!source.startsWith('{')) return null
  try {
    const parsed = JSON.parse(source) as RichTextNode
    return parsed?.type === 'doc' ? parsed : null
  } catch {
    return null
  }
}

export function enrichRichTextCommentMentions(
  body: string,
  users: readonly WorkItemUserOption[],
): string {
  const document = parsedRichTextComment(body)
  if (!document) return body
  const byID = new Map(users.map((user) => [String(user.id), user]))
  const byUsername = new Map(users.flatMap((user) => user.username
    ? [[user.username.toLocaleLowerCase(), user] as const]
    : []))
  const visit = (node: RichTextNode) => {
    if (node.type === 'mention') {
      const id = String(node.attrs?.id || '')
      const username = String(node.attrs?.username || '').toLocaleLowerCase()
      const user = byID.get(id) || byUsername.get(username)
      if (user) {
        node.attrs = {
          ...node.attrs,
          id: String(user.id),
          label: user.label,
          username: user.username || '',
        }
      }
    }
    node.content?.forEach(visit)
  }
  visit(document)
  return JSON.stringify(document)
}

export function prependRichTextReply(
  body: string,
  reply: Pick<ReplyRefMeta, 'author' | 'excerpt'>,
): string {
  const document = parsedRichTextComment(body) || { type: 'doc', content: [] }
  document.content = [
    {
      type: 'blockquote',
      content: [{
        type: 'paragraph',
        content: [{ type: 'text', text: `回复 @${reply.author}：${reply.excerpt}` }],
      }],
    },
    ...(document.content || []),
  ]
  return JSON.stringify(document)
}

export function buildReplyQuote(
  author: string,
  excerpt: string,
  previewAttachmentId?: number,
  targetCommentId?: number,
): string {
  const who = author.trim() || WORK_TASK_COMMENT_COPY.replyFallbackAuthor
  const text = excerpt.trim() || WORK_TASK_COMMENT_COPY.replyFallbackExcerpt
  let out = `> @${who}：${text.replace(/\n/g, ' ')}\n`
  if (previewAttachmentId) {
    out += `> ![preview](task-attach:${previewAttachmentId})\n`
  }
  if (targetCommentId) {
    out += `> #cmt:${targetCommentId}\n`
  }
  return `${out}\n`
}

export function appendTaskAttachMarkdown(filename: string, attachmentId: number): string {
  const alt = filename.trim() || WORK_TASK_COMMENT_COPY.replyImageAlt(attachmentId)
  return `\n![${alt}](task-attach:${attachmentId})`
}