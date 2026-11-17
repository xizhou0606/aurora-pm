import { useRef } from 'react'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { Button } from '@/shared/ui'
import { PhUploadSimple as Upload } from '@/shared/ui/icons'
import type {
  DocumentMarkdownImportControl,
  DocumentMarkdownImportState,
} from '../../hooks'

function isPending(state: DocumentMarkdownImportState): boolean {
  return state.status === 'reading'
    || state.status === 'creating'
    || state.status === 'uploading'
    || state.status === 'saving'
}

export function DocumentMarkdownImportButton({
  state,
  control,
}: {
  state: DocumentMarkdownImportState
  control: DocumentMarkdownImportControl
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const pending = isPending(state)
  return (
    <>
      <input
        ref={inputRef}
        type="file"
        className="sr-only"
        accept=".md,.markdown,.zip,text/markdown,application/zip"
        aria-label={DOCUMENTS_COPY.markdownImport.inputLabel}
        disabled={pending}
        onChange={(event) => {
          const file = event.currentTarget.files?.[0]
          event.currentTarget.value = ''
          if (file) void control.importFile(file)
        }}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={DOCUMENTS_COPY.markdownImport.button}
        title={DOCUMENTS_COPY.markdownImport.button}
        disabled={pending}
        onClick={() => inputRef.current?.click()}
      >
        <Upload aria-hidden="true" />
      </Button>
    </>
  )
}

export function DocumentMarkdownImportStatus({
  state,
  control,
}: {
  state: DocumentMarkdownImportState
  control: DocumentMarkdownImportControl
}) {
  if (state.status === 'idle') return null
  const copy = DOCUMENTS_COPY.markdownImport
  const message = state.status === 'reading'
    ? copy.reading
    : state.status === 'creating'
      ? copy.creating
      : state.status === 'uploading'
        ? copy.uploading(state.completed, state.total)
        : state.status === 'saving'
          ? copy.saving
          : state.status === 'success'
            ? copy.success(state.title, state.imageCount, state.externalImageCount)
            : copy.errors[state.reason]
  const dismissable = state.status === 'success' || state.status === 'error'
  return (
    <div
      role={state.status === 'error' ? 'alert' : 'status'}
      className={state.status === 'error'
        ? 'flex items-start gap-2 border-t border-destructive/30 px-3 py-2 text-xs text-destructive'
        : 'flex items-start gap-2 border-t border-border px-3 py-2 text-xs text-muted-foreground'}
    >
      <span className="min-w-0 flex-1">{message}</span>
      {dismissable ? (
        <Button type="button" variant="ghost" size="xs" onClick={control.dismiss}>
          {copy.dismiss}
        </Button>
      ) : null}
    </div>
  )
}
