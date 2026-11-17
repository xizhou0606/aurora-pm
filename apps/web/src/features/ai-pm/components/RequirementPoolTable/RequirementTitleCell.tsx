import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { resolvePMDocumentHref } from '@/shared/runtime/microApp'
import { Button } from '@/shared/ui'

const poolCopy = copy.aiPM.requirementPool

export function RequirementTitleCell({
  requirement,
  onOpen,
}: {
  requirement: { title: string }
  onOpen: () => void
}) {
  const [tip, setTip] = useState<{ top: number; left: number } | null>(null)
  const openTimer = useRef(0)
  const className = 'block min-w-0 truncate font-medium text-foreground'
  const label = requirement.title
  const show = (event: { currentTarget: HTMLElement }) => {
    const rect = event.currentTarget.getBoundingClientRect()
    window.clearTimeout(openTimer.current)
    openTimer.current = window.setTimeout(() => {
      setTip({ top: rect.top, left: rect.left })
    }, 80)
  }
  const hide = () => {
    window.clearTimeout(openTimer.current)
    setTip(null)
  }
  useEffect(() => () => window.clearTimeout(openTimer.current), [])
  const hover = { onPointerEnter: show, onPointerLeave: hide }
  return (
    <>
      <Button
        type="button"
        variant="link"
        size="sm"
        className={cn(className, 'h-auto max-w-full justify-start rounded-sm px-0 py-0 text-left')}
        aria-label={poolCopy.openRow(label)}
        onClick={(event) => {
          event.stopPropagation()
          onOpen()
        }}
        {...hover}
      >
        {label}
      </Button>
      {tip ? createPortal(
        <span
          role="tooltip"
          className="pointer-events-none fixed top-(--tooltip-y) left-(--tooltip-x) z-80 -translate-y-full max-w-sm rounded-md border border-border bg-card px-2.5 py-1.5 text-xs font-medium leading-5 text-foreground shadow-[var(--design-shadow-popover)]"
          style={{ '--tooltip-y': `${Math.max(8, tip.top - 8)}px`, '--tooltip-x': `${tip.left}px` }}
        >
          {label}
        </span>,
        document.body,
      ) : null}
    </>
  )
}

export function deliverableHref(value?: string) {
  const raw = value?.trim() || ''
  return raw ? resolvePMDocumentHref(raw) : ''
}

export function requirementDocumentHref(requirement: { product_doc_url?: string; deliverable_overview?: { requirement_document_url?: string } }) {
  return deliverableHref(requirement.deliverable_overview?.requirement_document_url || requirement.product_doc_url)
}