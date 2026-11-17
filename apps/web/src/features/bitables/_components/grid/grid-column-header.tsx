import { useRef, useState } from 'react'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { TableHead } from '@/shared/ui'
import { cn } from '@/shared/lib/utils'
import type { Field } from '../../model/types'
import { fieldIcons } from '../../model/field-icons'

export function GridColumnHeader({ field, width, disabled, onWidthChange, stretch = false }: {
  field: Field; width: number; disabled: boolean; stretch?: boolean; onWidthChange?: (id: string, width: number) => void
}) {
  const drag = useRef<{ x: number; width: number } | null>(null)
  const [draftWidth, setDraftWidth] = useState<number | null>(null)
  const size = draftWidth ?? width
  const FieldIcon = fieldIcons[field.type]
  const finish = (element: HTMLElement, pointerId: number) => {
    if (!drag.current) return
    drag.current = null
    if (element.hasPointerCapture(pointerId)) element.releasePointerCapture(pointerId)
    if (draftWidth !== null && draftWidth !== width) onWidthChange?.(field.id, draftWidth)
    setDraftWidth(null)
  }
  return <TableHead className={cn('group/column relative h-9 border-r border-border/50 bg-muted/40 px-3 text-xs font-medium', stretch ? 'min-w-(--col-w)' : 'w-(--col-w) min-w-(--col-w) max-w-(--col-w)')} style={{ '--col-w': `${size}px` }}>
    <span className="flex min-w-0 items-center gap-2" title={field.name}><FieldIcon className="size-3.5 shrink-0 text-muted-foreground/80" aria-hidden="true" /><span className="truncate">{field.name}</span></span>
    {!disabled && onWidthChange ? <span role="separator" aria-label={text.resizeColumn(field.name)} aria-orientation="vertical" aria-valuemin={80} aria-valuemax={640} aria-valuenow={size} tabIndex={0}
      className="absolute inset-y-0 right-0 w-1 cursor-col-resize touch-none hover:bg-primary focus-visible:bg-primary focus-visible:outline-none"
      onPointerDown={(event) => { event.preventDefault(); event.stopPropagation(); drag.current = { x: event.clientX, width }; event.currentTarget.setPointerCapture(event.pointerId) }}
      onPointerMove={(event) => { if (drag.current) setDraftWidth(Math.round(Math.max(80, Math.min(640, drag.current.width + event.clientX - drag.current.x)))) }}
      onPointerUp={(event) => finish(event.currentTarget, event.pointerId)} onPointerCancel={() => { drag.current = null; setDraftWidth(null) }}
      onKeyDown={(event) => { if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); onWidthChange(field.id, Math.round(Math.max(80, Math.min(640, width + (event.key === 'ArrowLeft' ? -16 : 16))))) } }} /> : null}
  </TableHead>
}
