import type { RichTextMention } from '@/shared/ui'

export type DocumentCommentAuthor = {
  id: number
  name: string
  username?: string
  avatarUrl?: string
}

export type DocumentCommentAnchorView =
  | {
      kind: 'document'
      state: 'document'
    }
  | {
      kind: 'selection_snapshot_v1'
      state: 'anchored' | 'orphaned'
      quote: string
      documentRevision: number
      from: number
      to: number
      prefix?: string
      suffix?: string
    }

export type DocumentCommentAnchor = DocumentCommentAnchorView

export type DocumentCommentRecord = {
  id: number
  workspaceId: number
  documentId: number
  threadId: number
  parentCommentId?: number
  author?: DocumentCommentAuthor
  authorUserId: number
  body: string
  mentionedUserIds: readonly number[]
  canEdit: boolean
  canDelete: boolean
  deletedAt?: string
  createdAt: string
  updatedAt: string
}

export type DocumentCommentThreadRecord = {
  id: number
  workspaceId: number
  documentId: number
  anchor: DocumentCommentAnchor
  status: 'open' | 'resolved'
  createdBy: number
  resolvedBy?: number
  resolvedAt?: string
  createdAt: string
  updatedAt: string
  comments: readonly DocumentCommentRecord[]
  canResolve: boolean
  canReopen: boolean
}

export type DocumentCommentThreadPage = {
  items: readonly DocumentCommentThreadRecord[]
  total: number
}

export type DocumentCommentView = {
  id: number
  parentCommentId?: number
  author: DocumentCommentAuthor
  body: string
  deleted?: boolean
  createdAtLabel?: string
  updatedAtLabel?: string
  canEdit: boolean
  canDelete: boolean
}

export type DocumentCommentThreadView = {
  id: number
  status: 'open' | 'resolved'
  anchor: DocumentCommentAnchorView
  comments: readonly DocumentCommentView[]
  canResolve: boolean
  canReopen: boolean
}

export type DocumentCommentThreadGroups = {
  inlineThreads: readonly DocumentCommentThreadView[]
  documentThreads: readonly DocumentCommentThreadView[]
}

export function partitionDocumentCommentThreads(
  threads: readonly DocumentCommentThreadView[],
): DocumentCommentThreadGroups {
  const inlineThreads: DocumentCommentThreadView[] = []
  const documentThreads: DocumentCommentThreadView[] = []
  for (const thread of threads) {
    if (thread.anchor.kind === 'selection_snapshot_v1') {
      inlineThreads.push(thread)
    } else {
      documentThreads.push(thread)
    }
  }
  return { inlineThreads, documentThreads }
}

export type DocumentCommentsViewState =
  | { status: 'hidden' }
  | { status: 'loading' }
  | { status: 'error'; message?: string }
  | { status: 'empty' }
  | { status: 'ready'; threads: readonly DocumentCommentThreadView[] }

export type DocumentCommentThreadKind = DocumentCommentAnchorView['kind']

export function filterDocumentCommentsState(
  state: DocumentCommentsViewState,
  kind: DocumentCommentThreadKind,
): DocumentCommentsViewState {
  if (state.status !== 'ready') return state
  const threads = state.threads.filter((thread) => thread.anchor.kind === kind)
  return threads.length ? { status: 'ready', threads } : { status: 'empty' }
}

export type DocumentCommentMutationIntent =
  | 'create'
  | 'reply'
  | 'edit'
  | 'delete'
  | 'resolve'
  | 'reopen'

export type DocumentCommentMutationState =
  | { status: 'idle' }
  | {
      status: 'pending'
      intent: DocumentCommentMutationIntent
      targetId?: number
    }
  | {
      status: 'error'
      intent: DocumentCommentMutationIntent
      targetId?: number
      message?: string
    }

export type DocumentCommentCapabilities = {
  canCreate: boolean
  canReply: boolean
}

export type DocumentCommentsDisclosure = {
  expanded: boolean
  onExpandedChange: (expanded: boolean) => void
}

export type DocumentCommentsControl = {
  retry: () => void
  selectAnchor: (anchor?: DocumentCommentAnchorView) => void
  createThread: (input: {
    anchor: DocumentCommentAnchorView
    body: string
  }) => Promise<boolean>
  reply: (input: {
    threadId: number
    parentCommentId: number
    body: string
  }) => Promise<boolean>
  edit: (input: { commentId: number; body: string }) => Promise<boolean>
  remove: (commentId: number) => Promise<boolean>
  resolve: (threadId: number) => Promise<boolean>
  reopen: (threadId: number) => Promise<boolean>
  focusAnchor?: (threadId: number) => void
}

export type DocumentCommentsComposer = {
  author: DocumentCommentAuthor
  mentions?: readonly RichTextMention[]
  onMentionQueryChange?: (query: string) => void
  anchor?: DocumentCommentAnchorView
}

export function replyMentionDraft(author: DocumentCommentAuthor): string {
  return JSON.stringify({ type: 'doc', content: [{ type: 'paragraph', content: [
    { type: 'mention', attrs: { id: String(author.id), label: author.name, username: author.username ?? '' } },
    { type: 'text', text: ' ' },
  ] }] })
}

export type DocumentCommentTimelineComposerTarget =
  | { kind: 'create'; anchor?: DocumentCommentAnchorView }
  | { kind: 'reply'; threadId: number }
  | { kind: 'edit'; commentId: number }

export type DocumentCommentTimelineItem =
  | { kind: 'thread'; thread: DocumentCommentThreadView }
  | { kind: 'composer' }

export type DocumentCommentTimelineGroups = {
  unresolved: readonly DocumentCommentTimelineItem[]
  resolved: readonly DocumentCommentTimelineItem[]
}

function inlineAnchorPosition(thread: DocumentCommentThreadView): number {
  return thread.anchor.kind === 'selection_snapshot_v1'
    ? thread.anchor.from
    : Number.MAX_SAFE_INTEGER
}

/**
 * Owns the view-only order for the inline comments rail. A draft remains local
 * UI state and is inserted beside its anchor without becoming a server thread.
 */
export function deriveDocumentCommentTimeline(
  threads: readonly DocumentCommentThreadView[],
  composer?: DocumentCommentTimelineComposerTarget,
): readonly DocumentCommentTimelineItem[] {
  const ordered = [...threads].sort((left, right) => {
    const byPosition = inlineAnchorPosition(left) - inlineAnchorPosition(right)
    return byPosition || left.id - right.id
  })
  const items: DocumentCommentTimelineItem[] = ordered.map((thread) => ({ kind: 'thread', thread }))
  if (!composer) return items

  if (composer.kind === 'create') {
    const position = composer.anchor?.kind === 'selection_snapshot_v1'
      ? composer.anchor.from
      : Number.MAX_SAFE_INTEGER
    const index = ordered.findIndex((thread) => inlineAnchorPosition(thread) >= position)
    items.splice(index < 0 ? items.length : index, 0, { kind: 'composer' })
    return items
  }

  const threadIndex = composer.kind === 'reply'
    ? ordered.findIndex((thread) => thread.id === composer.threadId)
    : ordered.findIndex((thread) => thread.comments.some((comment) => comment.id === composer.commentId))
  items.splice(threadIndex < 0 ? items.length : threadIndex + 1, 0, { kind: 'composer' })
  return items
}

/**
 * Owns status grouping without creating a second comment data source. New
 * drafts belong to the unresolved group; reply/edit drafts stay with their
 * owning thread while each group retains document order.
 */
export function deriveDocumentCommentTimelineGroups(
  threads: readonly DocumentCommentThreadView[],
  composer?: DocumentCommentTimelineComposerTarget,
): DocumentCommentTimelineGroups {
  const unresolvedThreads = threads.filter((thread) => thread.status === 'open')
  const resolvedThreads = threads.filter((thread) => thread.status === 'resolved')
  const owner = composer?.kind === 'reply'
    ? threads.find((thread) => thread.id === composer.threadId)
    : composer?.kind === 'edit'
      ? threads.find((thread) => thread.comments.some((comment) => comment.id === composer.commentId))
      : undefined
  const composerStatus = composer?.kind === 'create' ? 'open' : owner?.status

  return {
    unresolved: deriveDocumentCommentTimeline(
      unresolvedThreads,
      composerStatus === 'open' ? composer : undefined,
    ),
    resolved: deriveDocumentCommentTimeline(
      resolvedThreads,
      composerStatus === 'resolved' ? composer : undefined,
    ),
  }
}
