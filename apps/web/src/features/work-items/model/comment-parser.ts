import { escapeHtml, commentBodyHtml } from './comment-markup'
import { WORK_TASK_COMMENT_COPY } from '@/shared/copy'

/** 评论内嵌任务附件：![alt](task-attach:123) */
export const TASK_ATTACH_IMAGE_RE = /!\[([^\]]*)\]\(task-attach:(\d+)\)/g

export type CommentBodySegment =
  | {
      type: 'reply-ref'
      author: string
      excerpt: string
      previewAttachmentId?: number
      targetCommentId?: number
    }
  | { type: 'html'; html: string }
  | { type: 'image'; attachmentId: number; alt: string }

export type ReplyRefMeta = {
  author: string
  excerpt: string
  previewAttachmentId?: number
  targetCommentId?: number
}

const REPLY_COMMENT_ID_RE = /^#cmt:(\d+)$/

export function extractCommentAttachmentIds(body: string): number[] {
  const ids: number[] = []
  const re = new RegExp(TASK_ATTACH_IMAGE_RE.source, 'g')
  let m: RegExpExecArray | null
  while ((m = re.exec(String(body || ''))) !== null) {
    const id = Number(m[2])
    if (id) ids.push(id)
  }
  return ids
}

export function stripCommentBodyText(body: string): string {
  if (isRichTextCommentBody(body)) return richTextCommentPlainText(body)
  return String(body || '')
    .replace(TASK_ATTACH_IMAGE_RE, '')
    .replace(/^> .+(\n> .+)*/gm, '')
    .replace(/\s+/g, ' ')
    .trim()
}

export function isReplyComment(body: string): boolean {
  return String(body || '').trimStart().startsWith('>')
}

export function buildReplyExcerpt(body: string): { excerpt: string; previewAttachmentId?: number } {
  const text = stripCommentBodyText(body).slice(0, 120)
  if (text) return { excerpt: text }
  const ids = extractCommentAttachmentIds(body)
  if (ids.length) return { excerpt: WORK_TASK_COMMENT_COPY.replyImageExcerpt, previewAttachmentId: ids[0] }
  return { excerpt: WORK_TASK_COMMENT_COPY.replyFallbackExcerpt }
}

export function parseReplyQuoteLines(
  lines: string[],
): ReplyRefMeta | null {
  if (!lines.length) return null
  const header = lines[0].match(/^@([^：:]+)[：:](.*)$/)
  if (!header) return null

  let previewAttachmentId: number | undefined
  let targetCommentId: number | undefined
  for (const line of lines.slice(1)) {
    const attach = line.match(/task-attach:(\d+)/)
    if (attach) previewAttachmentId = Number(attach[1])
    const cmt = line.match(REPLY_COMMENT_ID_RE)
    if (cmt) targetCommentId = Number(cmt[1])
  }

  const excerpt = header[2].trim()
  if (!previewAttachmentId && excerpt === WORK_TASK_COMMENT_COPY.replyImageExcerpt) {
    const m = lines[1]?.match(/task-attach:(\d+)/)
    if (m) previewAttachmentId = Number(m[1])
  }

  return {
    author: header[1].trim(),
    excerpt: excerpt || WORK_TASK_COMMENT_COPY.replyFallbackExcerpt,
    previewAttachmentId,
    targetCommentId,
  }
}

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

export function isRichTextCommentBody(body: string): boolean {
  return Boolean(parsedRichTextComment(body))
}

function richTextCommentNodeText(node: RichTextNode): string {
  if (node.type === 'image') return WORK_TASK_COMMENT_COPY.replyImageExcerpt
  if (node.type === 'mention') {
    const label = String(node.attrs?.label || node.attrs?.username || node.attrs?.id || '').trim()
    return label ? `@${label}` : ''
  }
  const content = (node.content || []).map(richTextCommentNodeText).join('')
  if (['paragraph', 'blockquote', 'heading', 'listItem'].includes(node.type || '')) {
    return `${content}\n`
  }
  return node.text || content
}

export function richTextCommentPlainText(body: string): string {
  const document = parsedRichTextComment(body)
  if (!document) return String(body || '')
  return richTextCommentNodeText(document)
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

function parseInlineSegments(
  text: string,
  mentionLabels?: Map<string, string>,
): Array<{ type: 'html' | 'image'; html?: string; attachmentId?: number; alt?: string }> {
  const out: Array<{ type: 'html' | 'image'; html?: string; attachmentId?: number; alt?: string }> = []
  let last = 0
  TASK_ATTACH_IMAGE_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = TASK_ATTACH_IMAGE_RE.exec(text)) !== null) {
    const before = text.slice(last, m.index)
    if (before) {
      out.push({ type: 'html', html: commentBodyHtml(before, mentionLabels) })
    }
    out.push({
      type: 'image',
      attachmentId: Number(m[2]),
      alt: m[1] || WORK_TASK_COMMENT_COPY.replyImageAlt(Number(m[2])),
    })
    last = m.index + m[0].length
  }
  const tail = text.slice(last)
  if (tail) out.push({ type: 'html', html: commentBodyHtml(tail, mentionLabels) })
  return out
}

/** 将评论正文拆为引用块、富文本段、图片段 */
export function parseCommentBodySegments(
  body: string,
  mentionLabels?: Map<string, string>,
): CommentBodySegment[] {
  const raw = String(body || '')
  if (!raw.trim()) return []

  const lines = raw.split('\n')
  const segments: CommentBodySegment[] = []
  let buffer: string[] = []
  let quoteLines: string[] | null = null

  function flushQuote() {
    if (!quoteLines?.length) return
    const parsed = parseReplyQuoteLines(quoteLines)
    if (parsed) {
      segments.push({ type: 'reply-ref', ...parsed })
    } else {
      segments.push({
        type: 'reply-ref',
        author: '',
        excerpt: quoteLines.join(' ').trim() || WORK_TASK_COMMENT_COPY.replyFallbackExcerpt,
      })
    }
    quoteLines = null
  }

  function flushBuffer() {
    if (!buffer.length) return
    const chunk = buffer.join('\n')
    buffer = []
    for (const part of parseInlineSegments(chunk, mentionLabels)) {
      if (part.type === 'image' && part.attachmentId) {
        segments.push({ type: 'image', attachmentId: part.attachmentId, alt: part.alt || '' })
      } else if (part.html) {
        segments.push({ type: 'html', html: part.html })
      }
    }
  }

  for (const line of lines) {
    if (line.startsWith('> ')) {
      flushBuffer()
      if (!quoteLines) quoteLines = []
      quoteLines.push(line.slice(2))
      continue
    }
    if (line.trim() === '' && quoteLines) continue
    flushQuote()
    buffer.push(line)
  }
  flushQuote()
  flushBuffer()
  return segments
}

export function getReplyRefFromBody(body: string): ReplyRefMeta | null {
  const segs = parseCommentBodySegments(body)
  const ref = segs.find((s) => s.type === 'reply-ref')
  if (ref?.type !== 'reply-ref' || !ref.author) return null
  return {
    author: ref.author,
    excerpt: ref.excerpt,
    previewAttachmentId: ref.previewAttachmentId,
    targetCommentId: ref.targetCommentId,
  }
}

export function resolveReplyTargetCommentId<T extends { id: number; body: string }>(
  ref: ReplyRefMeta,
  rows: T[],
  fromCommentId: number,
  authorLabel: (row: T) => string,
): number | null {
  if (ref.targetCommentId && rows.some((r) => r.id === ref.targetCommentId)) {
    return ref.targetCommentId
  }

  const candidates = rows.filter((r) => r.id !== fromCommentId && authorLabel(r) === ref.author)
  if (!candidates.length) return null

  for (let i = candidates.length - 1; i >= 0; i--) {
    const row = candidates[i]
    const excerptMeta = buildReplyExcerpt(row.body)
    if (
      ref.previewAttachmentId &&
      excerptMeta.previewAttachmentId === ref.previewAttachmentId
    ) {
      return row.id
    }
    if (
      ref.excerpt &&
      ref.excerpt !== WORK_TASK_COMMENT_COPY.replyFallbackExcerpt &&
      excerptMeta.excerpt === ref.excerpt
    ) {
      return row.id
    }
  }

  const prior = rows.filter((r) => r.id < fromCommentId && authorLabel(r) === ref.author)
  return prior.length ? prior[prior.length - 1].id : candidates[candidates.length - 1].id
}

export function quoteLinesHtml(lines: string[]): string {
  return lines.map((l) => escapeHtml(l)).join('<br>')
}