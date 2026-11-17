import { PhUploadSimple } from '@/shared/ui/icons'
import {
  useId,
  useRef,
  useState,
  type ChangeEvent,
  type DragEvent,
  type ReactNode,
} from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { LazyMediaPreview } from './lazy-media-preview'
import { UploadFileCard } from './upload-file-card'
import { clipboardImageFiles } from './rich-text-images'

export interface UploadFileItem {
  url: string
  name?: string
  label?: string
  file?: File
}

export interface LazyFileUploaderProps<TItem extends UploadFileItem = UploadFileItem> {
  items?: readonly TItem[]
  label: ReactNode
  hint?: ReactNode
  inputLabel?: string
  accept?: string
  maxBytes?: number
  multiple?: boolean
  disabled?: boolean
  pending?: boolean
  error?: ReactNode
  onUpload: (file: File) => Promise<unknown> | unknown
  onRemove?: (item: TItem) => void
  compact?: { dropLabel: string; addLabel?: string }
  onPreview?: (item: TItem, trigger: HTMLButtonElement) => void
}

function itemLabel(item: UploadFileItem): string {
  if (item.name || item.label) return item.name || item.label || copy.ui.fileUploader.unnamed
  try {
    return decodeURIComponent(new URL(item.url, window.location.origin).pathname.split('/').pop() || '')
      || copy.ui.fileUploader.unnamed
  } catch {
    return copy.ui.fileUploader.unnamed
  }
}

export function LazyFileUploader<TItem extends UploadFileItem = UploadFileItem>({
  items = [],
  label,
  hint,
  inputLabel,
  accept,
  maxBytes,
  multiple = false,
  disabled = false,
  pending = false,
  error,
  onUpload,
  onRemove,
  onPreview,
  compact,
}: LazyFileUploaderProps<TItem>) {
  const inputRef = useRef<HTMLInputElement>(null)
  const previewTriggerRef = useRef<HTMLButtonElement | null>(null)
  const inputId = useId()
  const [internalPending, setInternalPending] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [validationError, setValidationError] = useState<string | null>(null)
  const [preview, setPreview] = useState({ url: '', title: '' })
  const isPending = pending || internalPending
  const isDisabled = disabled || isPending

  const uploadFiles = async (files: File[]) => {
    if (!files.length) return

    const oversized = maxBytes ? files.find((file) => file.size > maxBytes) : undefined
    if (oversized && maxBytes) {
      setValidationError(copy.ui.fileUploader.tooLarge(oversized.name, maxBytes))
      return
    }

    setValidationError(null)
    setInternalPending(true)
    try {
      for (const file of files) await onUpload(file)
    } catch {
      setValidationError(copy.ui.fileUploader.uploadFailed)
    } finally {
      setInternalPending(false)
    }
  }

  const handleChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.currentTarget.files || [])
    event.currentTarget.value = ''
    await uploadFiles(files)
  }

  const handleDrop = (event: DragEvent<HTMLButtonElement>) => {
    event.preventDefault()
    setDragging(false)
    if (isDisabled) return
    const files = Array.from(event.dataTransfer.files || [])
    void uploadFiles(multiple ? files : files.slice(0, 1))
  }

  return (
    <div data-slot="lazy-file-uploader" className="grid min-w-0 gap-2" onPaste={compact ? (event) => {
      if (isDisabled) return
      const files = clipboardImageFiles(event.clipboardData, file => file.type.startsWith('image/'))
      if (!files.length) return
      event.preventDefault(); event.stopPropagation()
      void uploadFiles(multiple ? files : files.slice(0, 1))
    } : undefined}>
      <label htmlFor={inputId} className={compact ? 'sr-only' : 'text-sm font-medium'}>{label}</label>
      <button
        type="button"
        className={cn(
          'group flex min-h-36 w-full flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border bg-background px-6 py-5 text-center outline-none transition-colors',
          'hover:border-primary/60 hover:bg-primary/[0.03] focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30',
          dragging && 'border-primary bg-primary/[0.06]',
          isDisabled && 'cursor-not-allowed bg-muted/30 opacity-60 hover:border-border hover:bg-muted/30',
          compact && 'min-h-24 cursor-pointer rounded-md bg-muted/40 px-4 py-3',
        )}
        disabled={isDisabled}
        aria-controls={inputId}
        onClick={() => inputRef.current?.click()}
        onDragEnter={(event) => {
          event.preventDefault()
          if (!isDisabled) setDragging(true)
        }}
        onDragOver={(event) => event.preventDefault()}
        onDragLeave={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragging(false)
        }}
        onDrop={handleDrop}
      >
        {!compact ? <PhUploadSimple aria-hidden="true" className="size-8 text-primary transition-transform group-hover:-translate-y-0.5" strokeWidth={1.8} /> : null}
        <span className="text-sm font-medium text-foreground">
          {isPending ? copy.ui.fileUploader.uploading : compact?.dropLabel ?? copy.ui.fileUploader.dropPrompt}
        </span>
        {hint ? <span className="max-w-2xl text-xs leading-5 text-muted-foreground">{hint}</span> : null}
      </button>
      {compact?.addLabel ? <button type="button" disabled={isDisabled} className="flex w-full cursor-pointer items-center justify-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50" onClick={() => inputRef.current?.click()}><span aria-hidden="true">{copy.ui.fileUploader.addGlyph}</span>{compact.addLabel}</button> : null}
      <input
        ref={inputRef}
        id={inputId}
        aria-label={inputLabel || (typeof label === 'string' ? label : copy.ui.fileUploader.choose)}
        className="sr-only"
        type="file"
        tabIndex={-1}
        accept={accept}
        multiple={multiple}
        disabled={isDisabled}
        onChange={(event) => void handleChange(event)}
      />
      {items.length ? (
        <ul className="m-0 flex min-w-0 list-none flex-wrap gap-3 p-0">
          {items.map((item, index) => (
            <UploadFileCard
              key={`${item.url}:${index}`}
              item={item}
              name={itemLabel(item)}
              disabled={isDisabled}
              onPreview={(previewItem, url, trigger) => {
                previewTriggerRef.current = trigger
                if (onPreview) onPreview(previewItem, trigger)
                else setPreview({ url, title: itemLabel(previewItem) })
              }}
              onRemove={onRemove}
            />
          ))}
        </ul>
      ) : null}
      {validationError || error ? (
        <p role="alert" className="m-0 text-xs text-destructive">{validationError || error}</p>
      ) : null}
      <LazyMediaPreview
        open={Boolean(preview.url)}
        url={preview.url}
        title={preview.title}
        finalFocus={previewTriggerRef}
        onOpenChange={(open) => { if (!open) setPreview({ url: '', title: '' }) }}
      />
    </div>
  )
}
