import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { cn } from '@/shared/lib/utils'
import { Button, isRichTextEmpty, LazyRichTextField, UserPreview } from '@/shared/ui'

import type { DocumentCommentsComposer, DocumentCommentView } from '../../model/comment'

export type DocumentCommentComposerMode =
  | { kind: 'create' }
  | { kind: 'reply'; threadId: number; comment: DocumentCommentView }
  | { kind: 'edit'; comment: DocumentCommentView }

export function DocumentCommentComposer({
  documentId,
  composer,
  mode,
  draft,
  pending,
  onDraftChange,
  onCancel,
  onSubmit,
  embedded = false,
  compact = false,
  focusRequestId,
}: {
  documentId: number
  composer: DocumentCommentsComposer
  mode: DocumentCommentComposerMode
  draft: string
  pending: boolean
  onDraftChange: (value: string) => void
  onCancel: () => void
  onSubmit: () => void
  embedded?: boolean
  compact?: boolean
  focusRequestId?: number
}) {
  const label = mode.kind === 'reply'
    ? DOCUMENTS_COPY.comments.replyTo(mode.comment.author.name)
    : mode.kind === 'edit'
      ? DOCUMENTS_COPY.comments.editLabel
      : DOCUMENTS_COPY.comments.createLabel
  const submitLabel = mode.kind === 'edit'
    ? DOCUMENTS_COPY.comments.save
    : DOCUMENTS_COPY.comments.submit
  const showActions = !compact || mode.kind === 'create' || (mode.kind === 'edit' ? draft !== mode.comment.body : !isRichTextEmpty(draft, 'json'))

  return (
    <div className={cn(
      'grid min-w-0 items-start gap-2.5',
      !compact && 'grid-cols-[auto_minmax(0,1fr)]',
      !embedded && 'border-t border-border pt-3',
    )}>
      {!compact ? <UserPreview
        name={composer.author.name || DOCUMENTS_COPY.comments.currentUser}
        avatarUrl={composer.author.avatarUrl}
        size="sm"
        showName={false}
        aria-label={composer.author.name || DOCUMENTS_COPY.comments.currentUser}
        className="mt-2"
      /> : null}
      <div className="grid min-w-0 gap-2">
        {mode.kind === 'reply' ? <p className="m-0 truncate text-xs text-muted-foreground">{label}</p> : null}
        {mode.kind === 'create' && composer.anchor?.kind === 'selection_snapshot_v1' ? (
          <blockquote className="m-0 line-clamp-2 border-l-2 border-primary/40 pl-3 text-xs text-muted-foreground">
            {composer.anchor.quote}
          </blockquote>
        ) : null}
        {compact && mode.kind === 'create' ? <UserPreview
          name={composer.author.name || DOCUMENTS_COPY.comments.currentUser}
          avatarUrl={composer.author.avatarUrl}
          size="sm"
        /> : null}
        <LazyRichTextField
          label={label}
          value={draft}
          onChange={onDraftChange}
          onSubmit={onSubmit}
          documentId={documentId}
          scope={`document-comment-composer-${mode.kind}-${mode.kind === 'reply' ? `${mode.threadId}-${mode.comment.id}` : mode.kind === 'edit' ? mode.comment.id : 'new'}`}
          serialization="json"
          visuallyHiddenLabel
          minHeight={compact ? 32 : 80}
          compact={compact}
          placeholder={mode.kind === 'reply' ? DOCUMENTS_COPY.comments.replyPlaceholder : DOCUMENTS_COPY.comments.placeholder}
          mentions={composer.mentions}
          onMentionQueryChange={composer.onMentionQueryChange}
          mentionListLabel={DOCUMENTS_COPY.comments.mentionOptions}
          mentionEmptyLabel={DOCUMENTS_COPY.comments.noMentionMatches}
          disabled={pending}
          focusRequestId={focusRequestId}
        />
        {showActions ? <div className="flex flex-wrap items-center justify-end gap-2">
          {compact || mode.kind !== 'create' ? (
            <Button type="button" size="sm" variant="ghost" disabled={pending} onClick={onCancel}>
              {DOCUMENTS_COPY.comments.cancel}
            </Button>
          ) : null}
          <Button
            type="button"
            size="sm"
            disabled={pending || isRichTextEmpty(draft, 'json')}
            onClick={onSubmit}
          >
            {pending ? DOCUMENTS_COPY.comments.pending : submitLabel}
          </Button>
        </div> : null}
      </div>
    </div>
  )
}
