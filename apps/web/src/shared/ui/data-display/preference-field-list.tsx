import { PhCaretDown, PhCaretUp, PhDotsSixVertical } from '@/shared/ui/icons'
import { useState } from 'react'

import { copy } from '@/shared/copy'
import type { TableColumnOption, TableColumnPreference } from '@/shared/model/table-column-preferences'

import { Button } from '../primitives/button'
import { Checkbox } from '../primitives/checkbox'

const text = copy.ui.tableColumns

export function PreferenceFieldList<TId extends string>({
  options,
  preference,
  onVisibleChange,
  onMove,
  onReorder,
}: {
  options: readonly TableColumnOption<TId>[]
  preference: TableColumnPreference<TId>
  onVisibleChange: (id: TId, visible: boolean) => void
  onMove: (id: TId, direction: -1 | 1) => void
  onReorder: (id: TId, targetId: TId) => void
}) {
  const [dragging, setDragging] = useState<TId | null>(null)
  const [dropTarget, setDropTarget] = useState<TId | null>(null)
  const byId = new Map(options.map((option) => [option.id, option]))
  const ordered = preference.order.flatMap((id) => {
    const option = byId.get(id)
    return option ? [option] : []
  })

  return (
    <ol className="m-0 grid list-none gap-2 p-0">
      {ordered.map((option, index) => {
        const visible = !preference.hidden.includes(option.id)
        return (
          <li
            key={option.id}
            className={`flex min-h-11 items-center gap-3 rounded-lg border px-3 py-2 transition-colors ${
              dragging === option.id
                ? 'border-primary/40 bg-muted/40 opacity-60'
                : dropTarget === option.id
                  ? 'border-primary bg-primary/5'
                  : 'border-border'
            }`}
            onDragOver={(event) => {
              if (!dragging || dragging === option.id || option.fixed) return
              event.preventDefault()
              event.dataTransfer.dropEffect = 'move'
              setDropTarget(option.id)
            }}
            onDragLeave={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                setDropTarget((current) => current === option.id ? null : current)
              }
            }}
            onDrop={(event) => {
              event.preventDefault()
              if (dragging && dragging !== option.id && !option.fixed) onReorder(dragging, option.id)
              setDragging(null)
              setDropTarget(null)
            }}
          >
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              draggable={!option.fixed}
              disabled={option.fixed}
              aria-label={text.drag(option.label)}
              className="cursor-grab touch-none text-muted-foreground active:cursor-grabbing"
              onDragStart={(event) => {
                event.dataTransfer.effectAllowed = 'move'
                event.dataTransfer.setData('text/plain', option.id)
                setDragging(option.id)
                setDropTarget(null)
              }}
              onDragEnd={() => {
                setDragging(null)
                setDropTarget(null)
              }}
            >
              <PhDotsSixVertical aria-hidden="true" />
            </Button>
            <Checkbox
              checked={visible}
              disabled={option.required}
              aria-label={text.toggle(option.label)}
              onCheckedChange={(checked) => onVisibleChange(option.id, checked === true)}
            />
            <span className="min-w-0 flex-1 truncate text-sm">{option.label}</span>
            {option.required ? <span className="text-xs text-muted-foreground">{text.required}</span> : null}
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={option.fixed || index === 0 || ordered[index - 1]?.fixed}
              aria-label={text.moveUp(option.label)}
              onClick={() => onMove(option.id, -1)}
            >
              <PhCaretUp aria-hidden="true" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={option.fixed || index === ordered.length - 1 || ordered[index + 1]?.fixed}
              aria-label={text.moveDown(option.label)}
              onClick={() => onMove(option.id, 1)}
            >
              <PhCaretDown aria-hidden="true" />
            </Button>
          </li>
        )
      })}
    </ol>
  )
}
