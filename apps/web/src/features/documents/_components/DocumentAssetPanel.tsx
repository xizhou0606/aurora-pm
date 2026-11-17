import { useEffect, useRef, useState } from 'react'
import {
  PhDownloadSimple as Download,
  PhTrash as Trash2,
} from '@/shared/ui/icons'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import {
  AlertBanner,
  Button,
  ConfirmDialog,
  EmptyState,
  LazyFileUploader,
  LazyMediaPreview,
  LoadingSkeleton,
} from '@/shared/ui'
import {
  DOCUMENT_ASSET_MAX_BYTES,
  type DocumentAssetsControl,
  type DocumentAssetsViewState,
} from '../hooks'
import type { DocumentAsset } from '../model'
import { saveDocumentDownload } from '../utils/download'
import { DocumentAssetThumbnail } from './DocumentAssetThumbnail'

const PREVIEWABLE_IMAGE_TYPES = new Set([
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
])

function formatAssetSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function isPreviewable(asset: DocumentAsset): boolean {
  return PREVIEWABLE_IMAGE_TYPES.has(asset.contentType.split(';', 1)[0].trim().toLowerCase())
}

export function DocumentAssetPanel({
  state,
  control,
}: {
  state: DocumentAssetsViewState
  control: DocumentAssetsControl
}) {
  const previewTriggerRef = useRef<HTMLButtonElement>(null)
  const [preview, setPreview] = useState<{ url: string; title: string } | null>(null)
  const [deleteAsset, setDeleteAsset] = useState<DocumentAsset | null>(null)

  useEffect(() => () => {
    if (preview?.url.startsWith('blob:')) URL.revokeObjectURL(preview.url)
  }, [preview])

  if (state.status === 'hidden') {
    return <p className="m-0 text-sm text-muted-foreground">{DOCUMENTS_COPY.assets.noAccess}</p>
  }
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={4} label={DOCUMENTS_COPY.assets.loading} />
  }
  if (state.status === 'error') {
    return (
      <div className="grid gap-3">
        <EmptyState
          title={DOCUMENTS_COPY.assets.loadFailedTitle}
          action={<Button type="button" variant="outline" onClick={() => { void state.retry() }}>{DOCUMENTS_COPY.page.retry}</Button>}
        />
        <AlertBanner tone="error">{DOCUMENTS_COPY.assets.loadFailedDescription}</AlertBanner>
      </div>
    )
  }

  const pending = state.operation === 'uploading'
    || state.operation === 'downloading'
    || state.operation === 'deleting'
  const transfer = async (asset: DocumentAsset, previewAsset: boolean) => {
    const blob = await control.download(asset.id)
    if (!blob) return
    if (!previewAsset) {
      saveDocumentDownload(blob, asset.fileName)
      return
    }
    setPreview((current) => {
      if (current?.url.startsWith('blob:')) URL.revokeObjectURL(current.url)
      return { url: URL.createObjectURL(blob), title: asset.fileName }
    })
  }
  const remove = async () => {
    if (!deleteAsset) return
    if (await control.remove(deleteAsset.id)) setDeleteAsset(null)
  }

  return (
    <div className="grid min-w-0 gap-5">
      {state.capabilities.canUploadAssets ? (
        <LazyFileUploader
          label={DOCUMENTS_COPY.assets.upload}
          inputLabel={DOCUMENTS_COPY.assets.upload}
          hint={DOCUMENTS_COPY.assets.limit}
          maxBytes={DOCUMENT_ASSET_MAX_BYTES}
          multiple
          pending={state.operation === 'uploading'}
          disabled={pending}
          error={state.operation === 'error' ? DOCUMENTS_COPY.assets.operationFailed : undefined}
          onUpload={control.upload}
        />
      ) : null}
      {state.items.length ? (
        <ul className="m-0 grid min-w-0 grid-cols-3 list-none gap-3 p-0 sm:grid-cols-6">
          {state.items.map((asset) => (
            <li
              key={asset.id}
              className="min-w-0 overflow-hidden rounded-lg border border-border bg-card shadow-[var(--design-shadow-card)] transition-control hover:border-primary/50"
            >
              {state.capabilities.canDownloadAssets && isPreviewable(asset) ? (
                <button
                  type="button"
                  className="block aspect-square w-full cursor-zoom-in overflow-hidden bg-muted/30 transition-control outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring disabled:cursor-wait disabled:opacity-50"
                  aria-label={DOCUMENTS_COPY.assets.preview(asset.fileName)}
                  title={DOCUMENTS_COPY.assets.preview(asset.fileName)}
                  disabled={pending}
                  onClick={(event) => {
                    previewTriggerRef.current = event.currentTarget
                    void transfer(asset, true)
                  }}
                >
                  <DocumentAssetThumbnail assetId={asset.id} fileName={asset.fileName} load={control.loadThumbnail} />
                </button>
              ) : (
                <span className="block aspect-square w-full">
                  <DocumentAssetThumbnail assetId={asset.id} fileName={asset.fileName} />
                </span>
              )}
              <span className="block min-w-0 border-t border-border px-2 pt-2">
                <span className="block truncate text-xs font-medium" title={asset.fileName}>{asset.fileName}</span>
                <span className="block text-xs text-muted-foreground">{formatAssetSize(asset.sizeBytes)}</span>
              </span>
              <span className="flex items-center justify-end gap-1 px-1 pb-1">
                {state.capabilities.canDownloadAssets ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label={DOCUMENTS_COPY.assets.download(asset.fileName)}
                    title={DOCUMENTS_COPY.assets.download(asset.fileName)}
                    disabled={pending}
                    onClick={() => { void transfer(asset, false) }}
                  >
                    <Download aria-hidden="true" />
                  </Button>
                ) : null}
                {state.capabilities.canDeleteAssets ? (
                  <Button
                    type="button"
                    variant="destructive"
                    size="icon-xs"
                    aria-label={DOCUMENTS_COPY.assets.remove(asset.fileName)}
                    title={DOCUMENTS_COPY.assets.remove(asset.fileName)}
                    disabled={pending}
                    onClick={() => setDeleteAsset(asset)}
                  >
                    <Trash2 aria-hidden="true" />
                  </Button>
                ) : null}
              </span>
            </li>
          ))}
        </ul>
      ) : (
        <EmptyState name="hugeicons:file-01" title={DOCUMENTS_COPY.assets.empty} />
      )}
      {state.operation === 'error' ? (
        <p role="alert" className="m-0 text-xs text-destructive">{DOCUMENTS_COPY.assets.operationFailed}</p>
      ) : null}
      <LazyMediaPreview
        open={Boolean(preview)}
        url={preview?.url || ''}
        title={preview?.title || DOCUMENTS_COPY.assets.title}
        alt={preview?.title}
        finalFocus={previewTriggerRef}
        onOpenChange={(open) => { if (!open) setPreview(null) }}
      />
      <ConfirmDialog
        open={Boolean(deleteAsset)}
        onOpenChange={(open) => { if (!open) setDeleteAsset(null) }}
        onConfirm={() => { void remove() }}
        title={DOCUMENTS_COPY.assets.removeTitle}
        description={DOCUMENTS_COPY.assets.removeDescription(deleteAsset?.fileName || '')}
        cancelLabel={DOCUMENTS_COPY.assets.cancel}
        confirmLabel={DOCUMENTS_COPY.assets.confirmRemove}
        pending={state.operation === 'deleting'}
        error={state.operation === 'error' ? DOCUMENTS_COPY.assets.operationFailed : undefined}
      />
    </div>
  )
}
