import { PhArrowBendUpLeft, PhPaperPlaneTilt, PhTrash } from '@/shared/ui/icons'
import { useEffect, useMemo, useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  EmptyState,
  isRichTextEmpty,
  LazyMediaPreview,
  LazyRichTextField,
  LoadingSkeleton,
  richTextVisibleCharacterCount,
  UserPreview,
} from '@/shared/ui'

import {
  buildReplyExcerpt,
  prependRichTextReply,
} from '../../model/comment-body'
import type {
  DetailResourceEntry,
  MutationState,
  UserOptionsState,
  WorkItemsDownload,
} from '../../model/types'
import { workItemCollaborationUser } from '../../model/detail-presenter'
import { CommentBody } from './CommentBodyRenderer'

const MAX_VISIBLE_CHARACTERS = 5_000

export function resolveActiveMention(value: string, cursor: number) {
  const prefix = value.slice(0, Math.max(0, cursor))
  const match = prefix.match(/(^|\s)@([^\s@]*)$/u)
  if (!match) return null
  const query = match[2] || ''
  return { start: prefix.length - query.length - 1, end: prefix.length, query }
}

export function CommentSection({
  taskId,
  entry,
  canComment,
  canAttach,
  canUploadBinary,
  users,
  currentUserId,
  mutation,
  onComment,
  onUploadImage,
  onDownload,
}: {
  taskId: number
  entry: DetailResourceEntry
  canComment: boolean
  canAttach: boolean
  canUploadBinary: boolean
  users: UserOptionsState
  currentUserId: number
  mutation: MutationState
  onComment: (body: string) => Promise<boolean>
  onUploadImage: (file: File) => Promise<string | null>
  onDownload: (id: number) => Promise<WorkItemsDownload | null>
}) {
  const [draft, setDraft] = useState('')
  const [reply, setReply] = useState<{ author: string; excerpt: string }>()
  const [message, setMessage] = useState('')
  const [deleteRow, setDeleteRow] = useState<{ id: string; label: string }>()
  const [previewURL, setPreviewURL] = useState<string | null>(null)
  const directory = users.status === 'ready' ? users.data : []
  const mentions = useMemo(() => directory.map((user) => ({
    id: user.id,
    label: user.label,
    username: user.username,
    avatarUrl: user.avatarUrl,
  })), [directory])
  const mentionLabels = useMemo(() => new Map(
    directory.flatMap((user) => user.username
      ? [[user.username.toLowerCase(), user.label] as const]
      : []),
  ), [directory])
  const currentUser = directory.find((user) => user.id === currentUserId)
  const pending = mutation.status === 'pending'
  const tooLong = richTextVisibleCharacterCount(draft) > MAX_VISIBLE_CHARACTERS
  const empty = isRichTextEmpty(draft, 'json')

  useEffect(() => {
    setDraft('')
    setReply(undefined)
    setMessage('')
    setPreviewURL(null)
  }, [taskId])

  const submit = async () => {
    if (pending || empty || tooLong) return
    const body = reply ? prependRichTextReply(draft, reply) : draft
    setMessage('')
    if (await onComment(body).catch(() => false)) {
      setDraft('')
      setReply(undefined)
      return
    }
    setMessage(WORK_ITEMS_COPY.detail.commentSubmitFailed)
  }

  return (
    <section className="grid min-w-0 content-start gap-3" aria-labelledby="work-items-detail-comments">
      <h3 id="work-items-detail-comments" className="sr-only">{entry.title}</h3>
      {entry.state.status === 'idle' || entry.state.status === 'loading' ? (
        <LoadingSkeleton rows={3} label={entry.title} />
      ) : entry.state.status === 'error' ? (
        <Button type="button" variant="outline" size="sm" onClick={entry.state.retry}>{WORK_ITEMS_COPY.actions.retry}</Button>
      ) : entry.state.data.length ? (
        <ul className="m-0 grid list-none gap-0 p-0">{entry.state.data.map((row) => {
          const author = row.secondary || WORK_ITEMS_COPY.enums.fallback
          const authorUser = users.status === 'ready'
            ? workItemCollaborationUser({ userId: row.userId, userName: author }, users.data)
            : undefined
          return (
            <li key={row.id} id={`comment-${row.id}`} className="grid grid-cols-[auto_minmax(0,1fr)] gap-3 border-b border-border/60 py-4 first:pt-1 last:border-b-0">
              <UserPreview
                name={author}
                avatarUrl={authorUser?.avatarUrl}
                size="sm"
                showName={false}
                aria-label={author}
                className="self-start"
              />
              <div className="grid min-w-0 gap-1.5">
                <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs">
                  <span className="truncate text-sm font-medium text-foreground">{author}</span>
                  {row.meta ? <span className="text-muted-foreground">{row.meta}</span> : null}
                {canComment || (row.canDelete && entry.onDeleteRow) ? (
                  <div className="ml-auto flex shrink-0 items-center gap-0.5">
                    {canComment ? (
                      <Button type="button" size="xs" variant="ghost" onClick={() => {
                        const excerpt = buildReplyExcerpt(row.primary)
                        setReply({ author, excerpt: excerpt.excerpt })
                      }}>
                        <PhArrowBendUpLeft aria-hidden="true" />
                        {WORK_ITEMS_COPY.detail.reply}
                      </Button>
                    ) : null}
                    {row.canDelete && entry.onDeleteRow ? (
                      <Button type="button" size="icon-xs" variant="danger-ghost" aria-label={WORK_ITEMS_COPY.detail.deleteComment} title={WORK_ITEMS_COPY.detail.deleteComment} onClick={() => setDeleteRow({ id: row.id, label: buildReplyExcerpt(row.primary).excerpt })}>
                        <PhTrash aria-hidden="true" />
                      </Button>
                    ) : null}
                  </div>
                ) : null}
                </div>
                <CommentBody
                  body={row.primary}
                  users={users}
                  mentionLabels={mentionLabels}
                  onDownload={onDownload}
                  onPreview={setPreviewURL}
                />

              </div>
            </li>
          )
        })}</ul>
      ) : <EmptyState title={entry.emptyLabel} />}
      {canComment ? (
        <div className="grid grid-cols-[auto_minmax(0,1fr)] items-start gap-2.5">
          <UserPreview
            name={currentUser?.label || WORK_ITEMS_COPY.detail.currentUser}
            avatarUrl={currentUser?.avatarUrl}
            size="sm"
            showName={false}
            aria-label={currentUser?.label || WORK_ITEMS_COPY.detail.currentUser}
            className="mt-2"
          />
          <div className="grid min-w-0 gap-2">
            {reply ? (
              <div className="flex items-start justify-between gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs">
                <span className="min-w-0 truncate">{WORK_ITEMS_COPY.detail.replyingTo(reply.author)}</span>
                <Button type="button" size="xs" variant="ghost" onClick={() => setReply(undefined)}>{WORK_ITEMS_COPY.mutation.cancel}</Button>
              </div>
            ) : null}
            <LazyRichTextField
              label={WORK_ITEMS_COPY.detail.addComment}
              value={draft}
              onChange={(value) => {
                setDraft(value)
                setMessage('')
              }}
              documentId={taskId}
              scope="comment-composer"
              serialization="json"
              visuallyHiddenLabel
              minHeight={96}
              placeholder={WORK_ITEMS_COPY.detail.commentPlaceholder}
              mentions={mentions}
              mentionListLabel={WORK_ITEMS_COPY.detail.mentionOptions}
              mentionEmptyLabel={WORK_ITEMS_COPY.detail.noMentionMatches}
              onUploadImage={canAttach && canUploadBinary ? async (file) => {
                const url = await onUploadImage(file)
                if (!url) throw new Error('comment_image_upload_failed')
                return url
              } : undefined}
            />
            <div className="flex items-center justify-end">
              <Button type="button" size="sm" disabled={pending || empty || tooLong} onClick={() => void submit()}>
                <PhPaperPlaneTilt aria-hidden="true" />
                {pending ? WORK_ITEMS_COPY.detail.sendingComment : WORK_ITEMS_COPY.detail.sendComment}
              </Button>
            </div>
          </div>
          {tooLong || message || mutation.status === 'error' ? (
            <p role="alert" className="col-start-2 m-0 text-xs text-destructive">
              {tooLong ? WORK_ITEMS_COPY.detail.commentTooLong : message || WORK_ITEMS_COPY.detail.commentSubmitFailed}
            </p>
          ) : null}
        </div>
      ) : null}
      <ConfirmDialog
        open={Boolean(deleteRow)}
        onOpenChange={(open) => { if (!open) setDeleteRow(undefined) }}
        onConfirm={() => {
          if (!deleteRow || !entry.onDeleteRow) return
          void entry.onDeleteRow(deleteRow.id).then((removed) => { if (removed) setDeleteRow(undefined) })
        }}
        title={WORK_ITEMS_COPY.detail.removeResourceTitle}
        description={WORK_ITEMS_COPY.detail.removeResourceDescription(deleteRow?.label || '')}
        cancelLabel={WORK_ITEMS_COPY.mutation.cancel}
        confirmLabel={WORK_ITEMS_COPY.detail.deleteComment}
        pending={pending}
      />
      <LazyMediaPreview
        open={Boolean(previewURL)}
        url={previewURL || ''}
        title={WORK_ITEMS_COPY.detail.comments}
        onOpenChange={(open) => { if (!open) setPreviewURL(null) }}
      />
    </section>
  )
}