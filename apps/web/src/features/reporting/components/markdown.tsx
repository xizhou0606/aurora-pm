import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { useMemo } from 'react'

import { cn } from '@/shared/lib/utils'

// GFM tables + hard line breaks match how the report body is composed.
marked.setOptions({ gfm: true, breaks: true })

type MarkdownViewProps = {
  source: string
  className?: string
  testId?: string
}

/**
 * Renders trusted-report Markdown (PM-composed facts + sanitized AI
 * narrative) as preview HTML. HTML in the source is sanitized by DOMPurify
 * before injection, so AI output can never carry script or event handlers.
 */
export function MarkdownView({ source, className, testId }: MarkdownViewProps) {
  const html = useMemo(
    () => DOMPurify.sanitize(marked.parse(source, { async: false })),
    [source],
  )
  return (
    <div
      data-slot="markdown-view"
      data-testid={testId}
      className={cn(
        'text-sm leading-6 text-foreground',
        '[&_h1]:mb-2 [&_h1]:text-base [&_h1]:font-semibold',
        '[&_h2]:mb-1 [&_h2]:mt-3 [&_h2]:text-sm [&_h2]:font-semibold',
        '[&_h3]:mb-1 [&_h3]:mt-2 [&_h3]:text-sm [&_h3]:font-medium',
        '[&_p]:my-1',
        '[&_ul]:my-1 [&_ul]:list-disc [&_ul]:pl-5',
        '[&_ol]:my-1 [&_ol]:list-decimal [&_ol]:pl-5',
        '[&_li]:my-0.5',
        '[&_strong]:font-semibold',
        '[&_table]:my-2 [&_table]:w-full [&_table]:border-collapse [&_table]:text-xs',
        '[&_th]:border [&_th]:border-border [&_th]:bg-muted/40 [&_th]:px-2 [&_th]:py-1 [&_th]:text-left [&_th]:font-medium',
        '[&_td]:border [&_td]:border-border [&_td]:px-2 [&_td]:py-1',
        '[&_blockquote]:my-1 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground',
        '[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:text-xs',
        className,
      )}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
