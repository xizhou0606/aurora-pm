import { useCommentAnchorLayout } from '../hooks/useCommentAnchorLayout'
import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import {
  AlertBanner,
  Button,
  ConfirmDialog,
  EmptyState,
  isRichTextEmpty,
  LoadingSkeleton,
} from '@/shared/ui'
import { PhCaretDown, PhCaretRight, PhX } from '@/shared/ui/icons'

import type {
  DocumentCommentCapabilities,
  DocumentCommentMutationState,
  DocumentCommentsComposer,
  DocumentCommentsControl,
  DocumentCommentsDisclosure,
  DocumentCommentsViewState,
  DocumentCommentTimelineItem,
  DocumentCommentView,
} from '../model/comment'
import { deriveDocumentCommentTimelineGroups, replyMentionDraft } from '../model/comment'
import {
  DocumentCommentComposer,
  type DocumentCommentComposerMode,
} from './document-comments/CommentComposer'
import { DocumentCommentThreadCard } from './document-comments/DocumentCommentThreadCard'
import { scrollCommentIntoView } from './document-comments/scroll-comment-into-view'

type DeleteTarget = Pick<DocumentCommentView, 'id' | 'author'>

export function DocumentCommentsPanel({
  documentId,
  state,
  mutation,
  capabilities,
  composer,
  disclosure,
  control,
  title = DOCUMENTS_COPY.comments.title,
  emptyLabel = DOCUMENTS_COPY.comments.empty,
  activeThreadId,
  onClose,
  collapsible = true,
  layout = 'section',
  focusComposerRequestId,
  headerAction,
  anchored = false,
}: {
  documentId: number
  state: DocumentCommentsViewState
  mutation: DocumentCommentMutationState
  capabilities: DocumentCommentCapabilities
  composer: DocumentCommentsComposer
  disclosure: DocumentCommentsDisclosure
  control: DocumentCommentsControl
  title?: ReactNode
  emptyLabel?: ReactNode
  activeThreadId?: number
  onClose?: () => void
  collapsible?: boolean
  layout?: 'section' | 'timeline'
  focusComposerRequestId?: number
  anchored?: boolean
  headerAction?: ReactNode
}) {
  const anchorLayoutRef = useCommentAnchorLayout(anchored, documentId)
  const contentId = useId()
  const [mode, setMode] = useState<DocumentCommentComposerMode>({ kind: 'create' })
  const [draft, setDraft] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget>()
  const [selectedThreadId, setSelectedThreadId] = useState<number>()
  const [createDismissed, setCreateDismissed] = useState(false)
  const [replyFocusRequestId, setReplyFocusRequestId] = useState(0)
  const composerContainerRef = useRef<HTMLDivElement>(null)
  const pending = mutation.status === 'pending'
  const threads = useMemo(() => state.status === 'ready'
    ? state.threads.filter(thread => thread.status === 'open')
    : [], [state])
  const showComposer = (capabilities.canCreate && (layout !== 'timeline' || !createDismissed && composer.anchor?.kind === 'selection_snapshot_v1')) || mode.kind !== 'create'
  const timelineGroups = useMemo(() => deriveDocumentCommentTimelineGroups(
    threads,
    showComposer
      ? mode.kind === 'create'
        ? { kind: 'create', anchor: composer.anchor }
        : mode.kind === 'reply'
          ? { kind: 'reply', threadId: mode.threadId }
          : { kind: 'edit', commentId: mode.comment.id }
      : undefined,
  ), [composer.anchor, mode, showComposer, threads])

  useEffect(() => {
    setMode({ kind: 'create' })
    setDraft('')
    setDeleteTarget(undefined)
    setSelectedThreadId(undefined)
    setCreateDismissed(false)
  }, [documentId])

  useEffect(() => {
    if (composer.anchor?.kind !== 'selection_snapshot_v1') return
    setMode({ kind: 'create' })
    setDraft('')
    setSelectedThreadId(undefined)
    setCreateDismissed(false)
  }, [
    composer.anchor?.kind,
    composer.anchor?.kind === 'selection_snapshot_v1' ? composer.anchor.documentRevision : undefined,
    composer.anchor?.kind === 'selection_snapshot_v1' ? composer.anchor.from : undefined,
    composer.anchor?.kind === 'selection_snapshot_v1' ? composer.anchor.to : undefined,
    focusComposerRequestId,
  ])

  useEffect(() => {
    if (
      layout !== 'timeline'
      || focusComposerRequestId === undefined
      || composer.anchor?.kind !== 'selection_snapshot_v1'
    ) return
    scrollCommentIntoView(composerContainerRef.current, 'nearest')
  }, [composer.anchor?.kind, focusComposerRequestId, layout, state.status])

  useEffect(() => {
    setSelectedThreadId(activeThreadId)
  }, [activeThreadId])

  useEffect(() => {
    if (layout !== 'timeline' || selectedThreadId === undefined) return
    const thread = threads.find(thread => thread.id === selectedThreadId)
    if (!thread && state.status === 'ready') {
      setSelectedThreadId(undefined)
      setMode({ kind: 'create' })
      setDraft('')
      setCreateDismissed(true)
      return
    }
    if (!capabilities.canReply || mode.kind === 'edit') return
    if (mode.kind === 'reply' && mode.threadId === selectedThreadId) return
    if (thread?.comments[0]) setMode({ kind: 'reply', threadId: thread.id, comment: thread.comments[0] })
  }, [capabilities.canReply, layout, mode, selectedThreadId, state.status, threads])

  const resetComposer = () => {
    setMode({ kind: 'create' })
    setDraft('')
    setCreateDismissed(true)
  }

  const submit = async () => {
    if (pending || isRichTextEmpty(draft, 'json')) return
    const succeeded = await (mode.kind === 'create'
      ? control.createThread({
          anchor: composer.anchor ?? { kind: 'document', state: 'document' },
          body: draft,
        })
      : mode.kind === 'reply'
        ? control.reply({
            threadId: mode.threadId,
            parentCommentId: mode.comment.id,
            body: draft,
          })
        : control.edit({ commentId: mode.comment.id, body: draft }))
      .catch(() => false)
    if (succeeded) resetComposer()
  }

  const renderContent = () => {
    if (state.status === 'loading') {
      return <LoadingSkeleton rows={4} label={DOCUMENTS_COPY.comments.loading} />
    }
    if (state.status === 'error') {
      return (
        <div className="grid gap-3">
          <EmptyState
            title={DOCUMENTS_COPY.comments.loadFailedTitle}
            action={(
              <Button type="button" variant="outline" size="sm" onClick={control.retry}>
                {DOCUMENTS_COPY.comments.retry}
              </Button>
            )}
          />
          <AlertBanner tone="error">
            {state.message || DOCUMENTS_COPY.comments.loadFailedDescription}
          </AlertBanner>
        </div>
      )
    }

    const visibleThreads = threads
    const composerNode = (
      <DocumentCommentComposer
        documentId={documentId}
        composer={composer}
        mode={mode}
        draft={draft}
        pending={pending}
        embedded={layout === 'timeline'}
        compact={layout === 'timeline'}
        onDraftChange={setDraft}
        onCancel={resetComposer}
        onSubmit={() => { void submit() }}
        focusRequestId={replyFocusRequestId ? replyFocusRequestId + (focusComposerRequestId ?? 0) : focusComposerRequestId}
      />
    )
    const renderTimelineItem = (item: DocumentCommentTimelineItem) => item.kind === 'composer' ? (mode.kind !== 'create' ? null : (
      <div
        key="comment-composer"
        ref={composerContainerRef}
        data-slot="comment-timeline-composer"
        className="rounded-lg border border-border border-t-[3px] border-t-amber-400 bg-card p-3 shadow-sm"
      >
        {composerNode}
      </div>
    )) : (
      <DocumentCommentThreadCard
        key={item.thread.id}
        thread={item.thread}
        documentId={documentId}
        mentions={composer.mentions}
        pending={pending}
        capabilities={capabilities}
        control={control}
        active={item.thread.id === selectedThreadId}
        compact={layout === 'timeline'}
        composerNode={layout === 'timeline' && (mode.kind === 'reply' && mode.threadId === item.thread.id || mode.kind === 'edit' && item.thread.comments.some(comment => comment.id === mode.comment.id)) ? composerNode : undefined}
        onSelect={() => {
          if (pending || !isRichTextEmpty(draft, 'json')) return
          setSelectedThreadId(item.thread.id)
          if (capabilities.canReply && item.thread.comments[0]) setMode({ kind: 'reply', threadId: item.thread.id, comment: item.thread.comments[0] })
        }}
        onReply={(threadId, comment) => {
          setReplyFocusRequestId(value => value + 1)
          setMode({ kind: 'reply', threadId, comment })
          setSelectedThreadId(threadId)
          setDraft(replyMentionDraft(comment.author))
        }}
        onEdit={(comment) => {
          setReplyFocusRequestId(value => value + 1)
          setMode({ kind: 'edit', comment })
          setSelectedThreadId(item.thread.id)
          setDraft(comment.body)
        }}
        onDelete={(comment) => setDeleteTarget({ id: comment.id, author: comment.author })}
      />
    )
    return (
      <div className="grid min-w-0 gap-3">
        {visibleThreads.length || (layout === 'timeline' && showComposer) ? (
          <div className="grid min-w-0 gap-3">
            {layout === 'timeline' ? (
              timelineGroups.unresolved.map(renderTimelineItem)
            ) : visibleThreads.map((thread) => renderTimelineItem({ kind: 'thread', thread }))}
          </div>
        ) : (
          <EmptyState title={emptyLabel} />
        )}
        {layout !== 'timeline' && showComposer ? composerNode : null}
        {mutation.status === 'error' ? (
          <AlertBanner tone="error">
            {mutation.message || DOCUMENTS_COPY.comments.operationFailed}
          </AlertBanner>
        ) : null}
      </div>
    )
  }

  return (
    <section
      ref={anchorLayoutRef}
      data-anchored-comments={anchored || undefined}
      className={layout === 'timeline'
        ? 'flex h-full w-full min-h-0 min-w-0 max-w-full flex-col bg-card'
        : 'min-w-0 overflow-hidden rounded-lg border border-border bg-card'}
      aria-busy={pending || undefined}
    >
      <div className={layout === 'timeline'
        ? 'flex h-10 min-w-0 shrink-0 items-center gap-2 border-b border-border px-2'
        : 'flex min-w-0 items-center gap-2 px-3 py-2.5'}>
        {collapsible ? (
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-expanded={disclosure.expanded}
            aria-controls={contentId}
            aria-label={disclosure.expanded ? DOCUMENTS_COPY.comments.collapsePanel : DOCUMENTS_COPY.comments.expandPanel}
            title={disclosure.expanded ? DOCUMENTS_COPY.comments.collapsePanel : DOCUMENTS_COPY.comments.expandPanel}
            onClick={() => disclosure.onExpandedChange(!disclosure.expanded)}
          >
            {disclosure.expanded ? <PhCaretDown aria-hidden="true" /> : <PhCaretRight aria-hidden="true" />}
          </Button>
        ) : null}
        <h3 className="m-0 min-w-0 flex-1 truncate text-sm font-semibold">
          {title}
        </h3>
        <span className="text-xs text-muted-foreground">
          {layout === 'timeline'
            ? DOCUMENTS_COPY.comments.compactCount(threads.length)
            : DOCUMENTS_COPY.comments.count(threads.length)}
        </span>
        {headerAction}
        {onClose ? (
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            aria-label={DOCUMENTS_COPY.comments.closePanel}
            title={DOCUMENTS_COPY.comments.closePanel}
            onClick={onClose}
          >
            <PhX aria-hidden="true" />
          </Button>
        ) : null}
      </div>
      {!collapsible || disclosure.expanded ? (
        <div id={contentId} data-slot={layout === 'timeline' ? 'document-comments-scroll' : undefined} className={anchored ? 'relative min-h-0 min-w-0 w-full flex-1 overflow-hidden px-3 py-4' : layout === 'timeline' ? 'min-h-0 min-w-0 w-full flex-1 overflow-x-hidden overflow-y-auto overscroll-contain px-3 py-4' : 'border-t border-border px-3 py-3'}>
          {renderContent()}
        </div>
      ) : null}
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => { if (!open) setDeleteTarget(undefined) }}
        onConfirm={() => {
          if (!deleteTarget) return
          void control.remove(deleteTarget.id).then((removed) => {
            if (removed) setDeleteTarget(undefined)
          }).catch(() => undefined)
        }}
        title={DOCUMENTS_COPY.comments.removeTitle}
        description={DOCUMENTS_COPY.comments.removeDescription(deleteTarget?.author.name || '')}
        cancelLabel={DOCUMENTS_COPY.comments.cancel}
        confirmLabel={DOCUMENTS_COPY.comments.confirmRemove}
        pending={pending}
        error={mutation.status === 'error' && mutation.intent === 'delete'
          ? mutation.message || DOCUMENTS_COPY.comments.operationFailed
          : undefined}
      />
    </section>
  )
}
