import { PhFileText, PhDownloadSimple, PhEye, PhTrash } from '@/shared/ui/icons'
import {
  Button,
  DetailSection,
  EmptyState,
  ConfirmDialog,
  LazyFileUploader,
  LazyMediaPreview,
  LoadingSkeleton,
} from '@/shared/ui'
import { useEffect, useRef, useState } from 'react'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { MutationState, ResourceState, WorkItemAttachment, WorkItemsDownload } from '../../model/types'
import { WORK_ITEM_ATTACHMENT_MAX_BYTES } from '../../model/attachments'

function sizeLabel(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function previewable(row: WorkItemAttachment): boolean {
  return String(row.contentType || '').toLowerCase().startsWith('image/')
    || /\.(jpe?g|png|gif|webp|bmp|svg)$/i.test(row.filename)
}

function saveDownload(download: WorkItemsDownload, fallbackName: string) {
  const url = URL.createObjectURL(download.blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = download.filename || fallbackName
  anchor.click()
  URL.revokeObjectURL(url)
}

export function AttachmentSection({
  state,
  canUpload,
  canDelete,
  canUploadBinary,
  canDownloadBinary,
  mutation,
  onUpload,
  onDelete,
  onDownload,
}: {
  state: ResourceState<WorkItemAttachment[]>
  canUpload: boolean
  canDelete: boolean
  canUploadBinary: boolean
  canDownloadBinary: boolean
  mutation: MutationState
  onUpload: (file: File) => Promise<boolean>
  onDelete: (id: number) => Promise<boolean>
  onDownload: (id: number) => Promise<WorkItemsDownload | null>
}) {
  const previewTriggerRef = useRef<HTMLButtonElement>(null)
  const [message, setMessage] = useState('')
  const [deleteRow, setDeleteRow] = useState<WorkItemAttachment | null>(null)
  const [preview, setPreview] = useState<{ url: string; title: string } | null>(null)
  const pending = mutation.status === 'pending'
  useEffect(() => () => {
    if (preview) URL.revokeObjectURL(preview.url)
  }, [preview])

  const closePreview = () => setPreview(null)

  const transfer = async (row: WorkItemAttachment, showPreview: boolean) => {
    try {
      const download = await onDownload(row.id)
      if (!download) return
      if (showPreview && previewable(row)) {
        setPreview({ url: URL.createObjectURL(download.blob), title: row.filename })
      } else {
        saveDownload(download, row.filename)
      }
    } catch {
      setMessage(WORK_ITEMS_COPY.detail.attachmentOperationFailed)
    }
  }
  const upload = async (file: File | undefined) => {
    if (!file) return
    if (file.size <= 0 || file.size > WORK_ITEM_ATTACHMENT_MAX_BYTES) {
      setMessage(WORK_ITEMS_COPY.detail.attachmentTooLarge)
      return
    }
    setMessage('')
    try {
      if (!await onUpload(file)) setMessage(WORK_ITEMS_COPY.detail.attachmentOperationFailed)
    } catch {
      setMessage(WORK_ITEMS_COPY.detail.attachmentOperationFailed)
    }
  }
  const remove = async () => {
    if (!deleteRow) return
    try {
      if (await onDelete(deleteRow.id)) setDeleteRow(null)
    } catch {
      setMessage(WORK_ITEMS_COPY.detail.attachmentOperationFailed)
    }
  }

  return (
    <DetailSection id="work-items-detail-attachments" title={WORK_ITEMS_COPY.detail.attachments} contentClassName="grid min-w-0 gap-3">
      {canUpload && canUploadBinary ? (
        <LazyFileUploader
          compact={{ dropLabel: WORK_ITEMS_COPY.detail.uploadAttachment }}
          label={WORK_ITEMS_COPY.detail.uploadAttachment}
          inputLabel={WORK_ITEMS_COPY.detail.uploadAttachment}
          hint={WORK_ITEMS_COPY.detail.attachmentLimit}
          maxBytes={WORK_ITEM_ATTACHMENT_MAX_BYTES}
          disabled={pending}
          pending={pending}
          error={message || (mutation.status === 'error' ? WORK_ITEMS_COPY.detail.attachmentOperationFailed : undefined)}
          onUpload={(file) => upload(file)}
        />
      ) : null}
      {!canUploadBinary && !canDownloadBinary ? <p className="text-xs text-muted-foreground">{WORK_ITEMS_COPY.detail.attachmentBinaryUnavailable}</p> : null}
      {!(canUpload && canUploadBinary) && (message || mutation.status === 'error') ? <p role="alert">{message || WORK_ITEMS_COPY.detail.attachmentOperationFailed}</p> : null}
      {state.status === 'idle' || state.status === 'loading' ? (
        <LoadingSkeleton rows={3} label={WORK_ITEMS_COPY.detail.attachments} />
      ) : state.status === 'error' ? (
        <div role="alert"><span>{WORK_ITEMS_COPY.detail.resourceError}</span><Button type="button" onClick={state.retry}>{WORK_ITEMS_COPY.actions.retry}</Button></div>
      ) : state.data.length ? (
        <ul className="m-0 grid list-none gap-2 p-0">
          {state.data.map((row) => (
            <li key={row.id} className="flex min-w-0 items-center gap-3 rounded-lg border border-border/60 px-3 py-2.5">
              <PhFileText className="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
              <div className="min-w-0 flex-1">
              <span className="block truncate text-sm" title={row.filename}>{row.filename}</span>
              <span className="block text-xs text-muted-foreground">{sizeLabel(row.size)}</span>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {canDownloadBinary && previewable(row) ? (
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={WORK_ITEMS_COPY.detail.previewAttachment}
                    title={WORK_ITEMS_COPY.detail.previewAttachment}
                    onClick={(event) => {
                      previewTriggerRef.current = event.currentTarget
                      void transfer(row, true)
                    }}
                  >
                    <PhEye aria-hidden="true" />
                  </Button>
                ) : null}
                {canDownloadBinary ? <Button type="button" size="icon-sm" variant="ghost" aria-label={WORK_ITEMS_COPY.detail.downloadAttachment} title={WORK_ITEMS_COPY.detail.downloadAttachment} onClick={() => void transfer(row, false)}><PhDownloadSimple aria-hidden="true" /></Button> : null}
                {canDelete ? <Button type="button" size="icon-sm" variant="danger-ghost" aria-label={WORK_ITEMS_COPY.detail.deleteAttachment} title={WORK_ITEMS_COPY.detail.deleteAttachment} onClick={() => setDeleteRow(row)}><PhTrash aria-hidden="true" /></Button> : null}
              </div>
            </li>
          ))}
        </ul>
      ) : !canUpload || !canUploadBinary ? <EmptyState title={WORK_ITEMS_COPY.detail.noAttachments} /> : null}
      <LazyMediaPreview
        open={Boolean(preview)}
        onOpenChange={(open) => { if (!open) closePreview() }}
        url={preview?.url || ''}
        title={preview?.title || WORK_ITEMS_COPY.detail.previewAttachment}
        alt={preview?.title}
        finalFocus={previewTriggerRef}
      />
      <ConfirmDialog
        open={Boolean(deleteRow)}
        onOpenChange={(open) => { if (!open) setDeleteRow(null) }}
        onConfirm={() => void remove()}
        title={WORK_ITEMS_COPY.detail.removeResourceTitle}
        description={WORK_ITEMS_COPY.detail.removeResourceDescription(deleteRow?.filename || '')}
        cancelLabel={WORK_ITEMS_COPY.mutation.cancel}
        confirmLabel={WORK_ITEMS_COPY.detail.deleteAttachment}
        pending={pending}
      />
    </DetailSection>
  )
}
