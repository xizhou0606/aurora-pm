import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { scrollCommentIntoView } from './scroll-comment-into-view'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { AlertBanner, Button, CopyLinkButton, LazyRichTextField, Popover, PopoverContent, PopoverTitle, PopoverTrigger, UserPreview } from '@/shared/ui'
import {
  PhArrowBendUpLeft,
  PhCaretDown,
  PhCaretRight,
  PhCheckCircle,
  PhPencil,
  PhTrash,
} from '@/shared/ui/icons'

import {
  documentCommentPath,
  type DocumentCommentCapabilities,
  type DocumentCommentsComposer,
  type DocumentCommentsControl,
  type DocumentCommentThreadView,
  type DocumentCommentView,
} from '../../model'

function SelectionSnapshot({
  thread,
  onFocusAnchor,
}: {
  thread: DocumentCommentThreadView
  onFocusAnchor?: (threadId: number) => void
}) {
  if (thread.anchor.kind === 'document') return null

  const orphaned = thread.anchor.state === 'orphaned'
  return (
    <div className="grid min-w-0 gap-2" data-anchor-state={thread.anchor.state}>
      <div className="flex min-w-0 items-center justify-between gap-2">
        <span className="text-xs font-medium text-muted-foreground">
          {orphaned ? DOCUMENTS_COPY.comments.orphaned : DOCUMENTS_COPY.comments.selectionSnapshot}
        </span>
        {!orphaned && onFocusAnchor ? (
          <Button type="button" size="xs" variant="ghost" onClick={() => onFocusAnchor(thread.id)}>
            {DOCUMENTS_COPY.comments.focusSelection}
          </Button>
        ) : null}
      </div>
      <blockquote className="m-0 line-clamp-3 border-l-2 border-primary/40 pl-3 text-sm text-muted-foreground">
        {thread.anchor.quote}
      </blockquote>
      {orphaned ? (
        <AlertBanner tone="warning">{DOCUMENTS_COPY.comments.orphanedDescription}</AlertBanner>
      ) : null}
    </div>
  )
}

function CommentRow({
  comment,
  documentId,
  mentions,
  pending,
  canReply,
  onReply,
  onEdit,
  onDelete,
  showActions = true,
  replyToName,
}: {
  comment: DocumentCommentView
  documentId: number
  mentions: DocumentCommentsComposer['mentions']
  pending: boolean
  canReply: boolean
  onReply: (comment: DocumentCommentView) => void
  onEdit: (comment: DocumentCommentView) => void
  onDelete: (comment: DocumentCommentView) => void
  showActions?: boolean
  replyToName?: string
}) {
  return (
    <li className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-2.5 border-b border-border py-3 last:border-b-0">
      <UserPreview
        name={comment.author.name}
        avatarUrl={comment.author.avatarUrl}
        size="sm"
        showName={false}
        aria-label={comment.author.name}
        className="self-start"
      />
      <div className="grid min-w-0 gap-1.5">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs">
          <strong className="truncate text-foreground">{comment.author.name}</strong>
          {replyToName ? <span className="min-w-0 truncate text-muted-foreground">{DOCUMENTS_COPY.comments.replyTo(replyToName)}</span> : null}
          {comment.updatedAtLabel || comment.createdAtLabel ? (
            <span className="text-muted-foreground">
              {comment.updatedAtLabel || comment.createdAtLabel}
            </span>
          ) : null}
        </div>
        {comment.deleted ? (
          <p className="m-0 text-sm italic text-muted-foreground">{DOCUMENTS_COPY.comments.deleted}</p>
        ) : (
          <LazyRichTextField
            label={DOCUMENTS_COPY.comments.title}
            value={comment.body}
            documentId={documentId}
            scope={`document-comment-${comment.id}`}
            serialization="json"
            visuallyHiddenLabel
            readOnly
            minHeight={0}
            imageDisplay="thumbnail"
            mentions={mentions}
          />
        )}
        {showActions && (canReply || comment.canEdit || comment.canDelete) ? (
          <div className="flex flex-wrap items-center gap-1 opacity-0 group-hover/comment:opacity-100 group-focus-within/comment:opacity-100">
            {canReply ? (
              <Button type="button" size="xs" variant="ghost" disabled={pending} onClick={() => onReply(comment)}>
                <PhArrowBendUpLeft aria-hidden="true" />
                {DOCUMENTS_COPY.comments.reply}
              </Button>
            ) : null}
            {comment.canEdit ? (
              <Button type="button" size="xs" variant="ghost" disabled={pending} onClick={() => onEdit(comment)}>
                <PhPencil aria-hidden="true" />
                {DOCUMENTS_COPY.comments.edit}
              </Button>
            ) : null}
            {comment.canDelete ? (
              <Button
                type="button"
                size="xs"
                variant="destructive"
                disabled={pending}
                onClick={() => onDelete(comment)}
              >
                <PhTrash aria-hidden="true" />
                {DOCUMENTS_COPY.comments.remove}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
    </li>
  )
}

export function DocumentCommentThreadCard({
  thread,
  documentId,
  mentions,
  pending,
  capabilities,
  control,
  onReply,
  onEdit,
  onDelete,
  active = false,
  compact = false,
  composerNode,
  onSelect,
}: {
  thread: DocumentCommentThreadView
  documentId: number
  mentions: DocumentCommentsComposer['mentions']
  pending: boolean
  capabilities: DocumentCommentCapabilities
  control: DocumentCommentsControl
  onReply: (threadId: number, comment: DocumentCommentView) => void
  onEdit: (comment: DocumentCommentView) => void
  onDelete: (comment: DocumentCommentView) => void
  active?: boolean
  compact?: boolean
  composerNode?: ReactNode
  onSelect?: () => void
}) {
  const [expanded, setExpanded] = useState(thread.status === 'open')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const contentId = useId()
  const threadRef = useRef<HTMLElement>(null)

  useEffect(() => {
    setExpanded(thread.status === 'open')
  }, [thread.id, thread.status])

  useEffect(() => {
    if (!active) return
    setExpanded(true)
    const alignThread = () => scrollCommentIntoView(threadRef.current)
    alignThread()
    const layoutTimers = [
      window.setTimeout(alignThread, 300),
      window.setTimeout(alignThread, 1_000),
    ]
    return () => layoutTimers.forEach((timer) => window.clearTimeout(timer))
  }, [active])

  return (
    <article
      id={`comment-${thread.id}`}
      ref={threadRef}
      className={`group/comment relative min-w-0 w-full max-w-full break-words rounded-lg border border-border bg-card ${active ? 'shadow-md before:absolute before:inset-x-0 before:top-0 before:h-1 before:rounded-t-lg before:bg-amber-400' : ''}`}
      tabIndex={compact ? 0 : undefined}
      onClick={() => { if (compact && !active) onSelect?.() }}
      onKeyDown={(event) => {
        if (compact && event.target === event.currentTarget && (event.key === 'Enter' || event.key === ' ')) {
          event.preventDefault()
          onSelect?.()
        }
      }}
      data-thread-status={thread.status}
      data-thread-id={thread.id}
      data-active={active || undefined}
    >
      <div className="flex min-w-0 items-center gap-1 px-3 pt-3">
        {!compact ? <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          aria-expanded={expanded}
          aria-controls={contentId}
          aria-label={expanded ? DOCUMENTS_COPY.comments.collapseThread : DOCUMENTS_COPY.comments.expandThread}
          title={expanded ? DOCUMENTS_COPY.comments.collapseThread : DOCUMENTS_COPY.comments.expandThread}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? <PhCaretDown aria-hidden="true" /> : <PhCaretRight aria-hidden="true" />}
        </Button> : null}
        <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
          {compact && thread.anchor.kind !== 'document' ? <span className="border-l-2 border-border pl-2">{thread.anchor.quote}</span> : DOCUMENTS_COPY.comments.replies(thread.comments.length)}
        </span>
        <div className={`flex items-center gap-1 rounded-md border border-border bg-card ${compact && !active ? 'opacity-0 group-hover/comment:opacity-100 group-focus-within/comment:opacity-100' : ''}`} onClick={event => event.stopPropagation()}>
        <CopyLinkButton path={documentCommentPath(documentId, thread.id)} iconOnly />
        {thread.canResolve ? (
          <Popover open={confirmOpen} onOpenChange={setConfirmOpen}>
            <PopoverTrigger render={<Button type="button" size="icon-xs" variant="ghost" disabled={pending} />} aria-label={DOCUMENTS_COPY.comments.resolve}>
              <PhCheckCircle aria-hidden="true" />
            </PopoverTrigger>
            <PopoverContent className="w-56 p-3" onClick={(event) => event.stopPropagation()}>
              <PopoverTitle className="text-xs font-medium">{DOCUMENTS_COPY.comments.resolveDescription}</PopoverTitle>
              <div className="mt-3 flex justify-end gap-2">
                <Button size="xs" variant="ghost" disabled={pending} onClick={() => setConfirmOpen(false)}>{DOCUMENTS_COPY.comments.cancel}</Button>
                <Button size="xs" disabled={pending} onClick={async () => {
                  if (await control.resolve(thread.id).catch(() => false)) setConfirmOpen(false)
                }}>{DOCUMENTS_COPY.comments.confirmResolve}</Button>
              </div>
            </PopoverContent>
          </Popover>
        ) : null}
        {thread.canReopen ? (
          <Button type="button" size="xs" variant="ghost" disabled={pending} onClick={() => { void control.reopen(thread.id) }}>
            {DOCUMENTS_COPY.comments.reopen}
          </Button>
        ) : null}
        </div>
      </div>
      {compact || expanded ? (
        <div id={contentId} className="grid min-w-0 gap-2 px-3 pb-1 pt-3">
          {!compact ? <SelectionSnapshot thread={thread} onFocusAnchor={control.focusAnchor} /> : null}
          <ul className="m-0 grid min-w-0 list-none p-0">
            {thread.comments.map((comment) => (
              <CommentRow
                key={comment.id}
                comment={comment}
                replyToName={thread.comments.find(parent => parent.id === comment.parentCommentId)?.author.name}
                documentId={documentId}
                mentions={mentions}
                pending={pending}
                showActions
                canReply={capabilities.canReply && thread.status === 'open'}
                onReply={(value) => onReply(thread.id, value)}
                onEdit={onEdit}
                onDelete={onDelete}
              />
            ))}
          </ul>
          {compact && active && composerNode ? <div className="pb-3">{composerNode}</div> : null}
        </div>
      ) : null}
    </article>
  )
}
