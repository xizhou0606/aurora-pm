import { lazy, Suspense, useRef, useState } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { PhEye, PhPencil, PhUploadSimple } from '../icons'
import { Button } from '../primitives/button'
import { Label } from '../primitives/label'
import { Textarea } from '../primitives/textarea'

// The markdown pipeline (marked + DOMPurify) stays in its own async chunk so
// route chunks that merely import this field keep their bundle budget.
const LazyMarkdownPreview = lazy(() => import('./markdown-preview').then((m) => ({ default: m.MarkdownPreview })))

export type MarkdownTextFieldProps = {
  label: string
  name: string
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  placeholder?: string
  id?: string
  /** Applied to the textarea and the preview frame (e.g. min height). */
  className?: string
}

type MarkdownFieldMode = 'edit' | 'preview'

/**
 * Markdown-backed text field: source editing with an in-place rendered
 * preview and `.md` file import. The stored value stays plain markdown
 * text, so it is backend-compatible with existing description fields.
 */
export function MarkdownTextField({
  label,
  name,
  value,
  onChange,
  disabled = false,
  placeholder,
  id,
  className,
}: MarkdownTextFieldProps) {
  const [mode, setMode] = useState<MarkdownFieldMode>('edit')
  const [importFailed, setImportFailed] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const inputId = id ?? name

  const importFile = async (file: File | undefined) => {
    if (!file) return
    try {
      const text = await file.text()
      setImportFailed(false)
      onChange(text)
    } catch {
      setImportFailed(true)
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  return (
    <div className="grid gap-1.5" role="group" aria-label={label}>
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
        <Label htmlFor={inputId}>{label}</Label>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            size="xs"
            variant={mode === 'edit' ? 'secondary' : 'ghost'}
            aria-pressed={mode === 'edit'}
            disabled={disabled}
            onClick={() => setMode('edit')}
          >
            <PhPencil aria-hidden="true" />
            {copy.ui.markdownEdit}
          </Button>
          <Button
            type="button"
            size="xs"
            variant={mode === 'preview' ? 'secondary' : 'ghost'}
            aria-pressed={mode === 'preview'}
            disabled={disabled}
            onClick={() => setMode('preview')}
          >
            <PhEye aria-hidden="true" />
            {copy.ui.markdownPreview}
          </Button>
          {mode === 'edit' ? (
            <Button
              type="button"
              size="xs"
              variant="ghost"
              disabled={disabled}
              onClick={() => fileInputRef.current?.click()}
            >
              <PhUploadSimple aria-hidden="true" />
              {copy.ui.markdownImport}
            </Button>
          ) : null}
        </div>
      </div>
      {mode === 'edit' ? (
        <Textarea
          id={inputId}
          name={name}
          value={value}
          disabled={disabled}
          placeholder={placeholder}
          className={className}
          onChange={(event) => onChange(event.target.value)}
        />
      ) : (
        <div
          className={cn(
            'overflow-y-auto rounded-md border border-input bg-background px-2.5 py-2 shadow-xs',
            className,
          )}
        >
          {value.trim() ? (
            <Suspense fallback={<div className="pm-markdown min-h-24 animate-pulse rounded-sm bg-muted" aria-hidden="true" />}>
              <LazyMarkdownPreview source={value} />
            </Suspense>
          ) : (
            <p className="m-0 text-sm text-muted-foreground">{placeholder ?? copy.ui.markdownPreviewEmpty}</p>
          )}
        </div>
      )}
      {importFailed ? <p role="alert" className="m-0 text-xs text-destructive">{copy.ui.markdownImportFailed}</p> : null}
      <input
        ref={fileInputRef}
        type="file"
        accept=".md,.markdown,text/markdown"
        aria-label={copy.ui.markdownImportFile}
        className="sr-only"
        tabIndex={-1}
        onChange={(event) => void importFile(event.target.files?.[0])}
      />
    </div>
  )
}
