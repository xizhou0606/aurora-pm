import { PhArrowSquareOut } from '@/shared/ui/icons'
import { Fragment, type JSX, useState } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, buttonVariants, uploadPreviewKind } from '@/shared/ui'

import { materialDisplayName } from '../../model/presenters'
import { safeMaterialUrl } from '../../model/values'

const ACTIVITY_URL_PATTERN = /https?:\/\/[^\s；;，,、"'\\\]}]+/g

function activityExternalLinkLabel(url: string, name: string): string {
  try {
    const hostname = new URL(url).hostname.toLocaleLowerCase()
    return hostname === 'figma.com' || hostname.endsWith('.figma.com')
      ? QUALITY_COPY.activity.openDesignLink(name)
      : QUALITY_COPY.activity.openExternalLink(name)
  } catch {
    return QUALITY_COPY.activity.openExternalLink(name)
  }
}

export function activityActionLabel(action?: string): string {
  switch (action) {
    case 'created': return QUALITY_COPY.activity.created
    case 'assigned': return QUALITY_COPY.activity.assigned
    case 'resolved': return QUALITY_COPY.activity.resolved
    case 'returned': return QUALITY_COPY.activity.returned
    case 'comment': return QUALITY_COPY.activity.commented
    case 'status_transition': return QUALITY_COPY.activity.statusChanged
    case 'updated': return QUALITY_COPY.activity.fieldChanged
    default: return QUALITY_COPY.activity.unknown
  }
}

export function ActivitySummary({
  value,
  onPreview,
}: {
  value: string
  onPreview: (url: string, trigger: HTMLElement) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const expandable = value.length > 160 || value.split('\n').length > 2
  const segments: (string | JSX.Element)[] = []
  const renderedUrls = new Set<string>()
  let lastIndex = 0
  for (const match of value.matchAll(ACTIVITY_URL_PATTERN)) {
    const index = match.index ?? 0
    if (index > lastIndex) segments.push(value.slice(lastIndex, index))
    const safeUrl = safeMaterialUrl(match[0])
    const name = safeUrl ? materialDisplayName(safeUrl) : QUALITY_COPY.detail.materials
    if (safeUrl && renderedUrls.has(safeUrl)) {
      lastIndex = index + match[0].length
      continue
    }
    if (safeUrl) renderedUrls.add(safeUrl)
    const previewKind = safeUrl ? uploadPreviewKind({ url: safeUrl }) : undefined
    segments.push(safeUrl ? (
      previewKind === 'image' ? (
        <Button
          key={`${index}:${safeUrl}`}
          type="button"
          variant="ghost"
          aria-label={QUALITY_COPY.upload.previewNamed(name)}
          title={QUALITY_COPY.upload.previewNamed(name)}
          className="group mx-1 inline-flex size-10 overflow-hidden rounded border border-border bg-muted p-0 align-middle transition-shadow hover:bg-muted hover:ring-2 hover:ring-primary/20 focus-visible:ring-2 focus-visible:ring-ring"
          onClick={(event) => onPreview(safeUrl, event.currentTarget)}
        >
          <img src={safeUrl} alt="" className="size-full object-cover transition-transform group-hover:scale-105" />
        </Button>
      ) : previewKind === 'video' ? (
        <Button key={`${index}:${safeUrl}`} type="button" size="xs" variant="link" onClick={(event) => onPreview(safeUrl, event.currentTarget)}>
          {QUALITY_COPY.upload.preview} {name}
        </Button>
      ) : (
        <a
          key={`${index}:${safeUrl}`}
          href={safeUrl}
          target="_blank"
          rel="noopener noreferrer"
          title={activityExternalLinkLabel(safeUrl, name)}
          className={buttonVariants({
            variant: 'link',
            size: 'xs',
            className: 'mx-1 h-auto max-w-full whitespace-normal px-0 align-middle',
          })}
        >
          <PhArrowSquareOut aria-hidden="true" />
          <span className="min-w-0 truncate">{activityExternalLinkLabel(safeUrl, name)}</span>
        </a>
      )
    ) : QUALITY_COPY.detail.materials)
    lastIndex = index + match[0].length
  }
  if (lastIndex < value.length) segments.push(value.slice(lastIndex))
  return (
    <span className="grid min-w-0 gap-1">
      <span className={cn(
        'min-w-0 whitespace-pre-wrap break-words [overflow-wrap:anywhere]',
        expandable && !expanded && 'line-clamp-2',
      )}>
        {segments.length
          ? segments.map((segment, index) => <Fragment key={typeof segment === 'string' ? `${index}:${segment}` : index}>{segment}</Fragment>)
          : value}
      </span>
      {expandable ? (
        <Button
          type="button"
          size="xs"
          variant="link"
          className="w-fit p-0"
          aria-expanded={expanded}
          onClick={() => setExpanded((current) => !current)}
        >
          {expanded ? QUALITY_COPY.activity.collapse : QUALITY_COPY.activity.expand}
        </Button>
      ) : null}
    </span>
  )
}