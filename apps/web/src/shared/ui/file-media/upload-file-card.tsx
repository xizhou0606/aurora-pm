import { PhEye, PhFileText, PhPlay } from '@/shared/ui/icons'
import { useEffect, useState } from 'react'

import { copy } from '@/shared/copy'

import { Button, buttonVariants } from '../primitives/button'

export interface UploadPreviewItem {
  url: string
  name?: string
  label?: string
  file?: File
}

export function uploadPreviewKind(item: UploadPreviewItem): 'image' | 'video' | 'file' {
  const source = item.file?.type || item.url
  if (item.file?.type.startsWith('image/') || /\.(?:avif|gif|jpe?g|png|svg|webp)(?:[?#].*)?$/i.test(source)) return 'image'
  if (item.file?.type.startsWith('video/') || /\.(?:mp4|ogg|webm)(?:[?#].*)?$/i.test(source)) return 'video'
  return 'file'
}

export function UploadFileCard<TItem extends UploadPreviewItem>({
  item,
  name,
  disabled,
  onPreview,
  onRemove,
}: {
  item: TItem
  name: string
  disabled: boolean
  onPreview: (item: TItem, url: string, trigger: HTMLButtonElement) => void
  onRemove?: (item: TItem) => void
}) {
  const [localUrl, setLocalUrl] = useState('')
  const kind = uploadPreviewKind(item)

  useEffect(() => {
    if (!item.file || typeof URL.createObjectURL !== 'function') {
      setLocalUrl('')
      return
    }
    const url = URL.createObjectURL(item.file)
    setLocalUrl(url)
    return () => URL.revokeObjectURL(url)
  }, [item.file])

  const remoteUrl = item.url.startsWith('pending:') ? '' : item.url
  const previewUrl = localUrl || remoteUrl
  const typeLabel = item.label || (
    kind === 'image'
      ? copy.ui.fileUploader.image
      : kind === 'video'
        ? copy.ui.fileUploader.video
        : name || copy.ui.fileUploader.file
  )
  const previewLabel = kind === 'video'
    ? copy.ui.fileUploader.playVideo
    : copy.ui.fileUploader.previewImage
  const media = kind === 'image' ? (
    <img src={previewUrl} alt="" className="size-full object-cover transition-transform duration-200 group-hover/media:scale-[1.02]" />
  ) : (
    <video
      src={`${previewUrl}#t=0.001`}
      aria-hidden="true"
      muted
      playsInline
      preload="metadata"
      className="size-full object-cover transition-transform duration-200 group-hover/media:scale-[1.02]"
    />
  )

  return (
    <li className="grid w-40 min-w-0 overflow-hidden rounded-md border border-border bg-card">
      {previewUrl && kind !== 'file' ? (
        <button
          type="button"
          aria-label={previewLabel}
          title={previewLabel}
          className="group/media relative flex h-24 items-center justify-center overflow-hidden bg-muted outline-none focus-visible:ring-3 focus-visible:ring-inset focus-visible:ring-ring/40"
          onClick={(event) => onPreview(item, previewUrl, event.currentTarget)}
        >
          {media}
          <span className="absolute inset-0 flex items-center justify-center gap-1.5 bg-foreground/55 px-2 text-xs font-medium text-background opacity-0 transition-opacity group-hover/media:opacity-100 group-focus-visible/media:opacity-100">
            {kind === 'video' ? <PhPlay aria-hidden="true" className="size-5 fill-current" /> : <PhEye aria-hidden="true" className="size-5" />}
            {previewLabel}
          </span>
        </button>
      ) : (
        <span className="flex h-24 items-center justify-center overflow-hidden bg-muted">
          <PhFileText aria-hidden="true" className="size-8 text-muted-foreground" strokeWidth={1.5} />
        </span>
      )}
      <span className="truncate border-t border-border px-2 py-1.5 text-xs" title={name}>
        {typeLabel}
      </span>
      <div className="mx-1 mb-1 flex justify-end gap-1">
        {previewUrl && kind === 'file' ? (
          <a
            href={previewUrl}
            target="_blank"
            rel="noreferrer"
            className={buttonVariants({ variant: 'ghost', size: 'xs' })}
          >
            {copy.ui.fileUploader.openAttachment}
          </a>
        ) : null}
        {onRemove ? (
          <Button
            type="button"
            size="xs"
            variant="ghost"
            disabled={disabled}
            onClick={() => onRemove(item)}
          >
            {copy.ui.fileUploader.remove}
          </Button>
        ) : null}
      </div>
    </li>
  )
}
