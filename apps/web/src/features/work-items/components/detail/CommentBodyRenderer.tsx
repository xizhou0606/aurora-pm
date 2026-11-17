import { useEffect, useMemo, useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { Button, LazyRichTextField } from '@/shared/ui'

import type { UserOptionsState } from '../../model/types'

import {
  buildReplyExcerpt,
  enrichRichTextCommentMentions,
  isRichTextCommentBody,
  parseCommentBodySegments,
} from '../../model/comment-body'
import type {
  WorkItemsDownload,
} from '../../model/types'

function LegacyCommentImage({
  attachmentId,
  alt,
  onDownload,
  onPreview,
}: {
  attachmentId: number
  alt: string
  onDownload: (id: number) => Promise<WorkItemsDownload | null>
  onPreview: (url: string) => void
}) {
  const [url, setURL] = useState('')
  useEffect(() => {
    let active = true
    let objectURL = ''
    void onDownload(attachmentId).then((download) => {
      if (!active || !download) return
      objectURL = URL.createObjectURL(download.blob)
      setURL(objectURL)
    }).catch(() => undefined)
    return () => {
      active = false
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [attachmentId, onDownload])
  if (!url) return null
  return (
    <Button
      type="button"
      variant="ghost"
      className="group h-auto max-w-full overflow-hidden rounded-md border border-border bg-muted p-0 hover:bg-muted"
      onClick={() => onPreview(url)}
    >
      <img
        src={url}
        alt={alt}
        loading="lazy"
        className="max-h-64 max-w-full cursor-zoom-in object-contain transition-opacity group-hover:opacity-90"
      />
    </Button>
  )
}

function LegacyCommentBody({
  body,
  mentionLabels,
  onDownload,
  onPreview,
}: {
  body: string
  mentionLabels: Map<string, string>
  onDownload: (id: number) => Promise<WorkItemsDownload | null>
  onPreview: (url: string) => void
}) {
  const segments = useMemo(() => parseCommentBodySegments(body, mentionLabels), [body, mentionLabels])
  return (
    <div className="grid min-w-0 gap-2 text-sm">
      {segments.map((segment, index) => {
        if (segment.type === 'reply-ref') {
          return (
            <blockquote key={`reply:${index}`} className="m-0 rounded-md border-l-2 border-primary/40 bg-muted/50 px-3 py-2 text-muted-foreground">
              {WORK_ITEMS_COPY.detail.replyTo(segment.author, segment.excerpt)}
              {segment.previewAttachmentId ? (
                <LegacyCommentImage
                  attachmentId={segment.previewAttachmentId}
                  alt=""
                  onDownload={onDownload}
                  onPreview={onPreview}
                />
              ) : null}
            </blockquote>
          )
        }
        if (segment.type === 'image') {
          return (
            <LegacyCommentImage
              key={`image:${segment.attachmentId}:${index}`}
              attachmentId={segment.attachmentId}
              alt={segment.alt}
              onDownload={onDownload}
              onPreview={onPreview}
            />
          )
        }
        return (
          <span
            key={`html:${index}`}
            className="break-words whitespace-pre-wrap"
            dangerouslySetInnerHTML={{ __html: segment.html }}
          />
        )
      })}
    </div>
  )
}

export function CommentBody({
  body,
  users,
  mentionLabels,
  onDownload,
  onPreview,
}: {
  body: string
  users: UserOptionsState
  mentionLabels: Map<string, string>
  onDownload: (id: number) => Promise<WorkItemsDownload | null>
  onPreview: (url: string) => void
}) {
  if (!isRichTextCommentBody(body)) {
    return (
      <LegacyCommentBody
        body={body}
        mentionLabels={mentionLabels}
        onDownload={onDownload}
        onPreview={onPreview}
      />
    )
  }
  const directory = users.status === 'ready' ? users.data : []
  return (
    <LazyRichTextField
      label={WORK_ITEMS_COPY.detail.comments}
      value={enrichRichTextCommentMentions(body, directory)}
      serialization="json"
      visuallyHiddenLabel
      readOnly
      minHeight={0}
      imageDisplay="thumbnail"
      mentions={directory}
      onPreviewImage={(url) => onPreview(url)}
    />
  )
}

export { buildReplyExcerpt }