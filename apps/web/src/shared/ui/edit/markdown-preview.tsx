import { useMemo } from 'react'

import { cn } from '@/shared/lib/utils'

import { renderMarkdown } from './markdown'

export type MarkdownPreviewProps = {
  source: string
  className?: string
}

/**
 * Read-only rendered view of markdown source. Output is sanitized in
 * `renderMarkdown`; typography styles live under `.pm-markdown` in
 * `shared/styles.css` and follow semantic tokens in both ui modes.
 */
export function MarkdownPreview({ source, className }: MarkdownPreviewProps) {
  const html = useMemo(() => renderMarkdown(source), [source])
  return (
    <div
      data-slot="markdown-preview"
      role="document"
      className={cn('pm-markdown text-sm text-foreground', className)}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
