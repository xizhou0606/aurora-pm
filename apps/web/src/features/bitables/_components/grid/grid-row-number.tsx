import type { DragEvent } from 'react'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button, Checkbox, TableCell } from '@/shared/ui'
import { PhDotsSixVertical } from '@/shared/ui/icons'
import { cn } from '@/shared/lib/utils'

export function GridRowNumber({ number, selected, disabled, canMove, onSelect, onDragStart, onDragEnd, onKeyboardMove }: {
  number: number; selected: boolean; disabled: boolean; canMove: boolean; onSelect: (selected: boolean) => void
  onDragStart: (event: DragEvent<HTMLButtonElement>) => void; onDragEnd: () => void; onKeyboardMove: (direction: -1 | 1) => void
}) {
  return <TableCell data-slot="bitable-row-number" className={cn('sticky left-0 z-10 w-16 min-w-16 max-w-16 border-r border-border/50 bg-card p-0 text-center text-xs tabular-nums text-muted-foreground/70 group-hover/record:bg-muted group-focus-within/record:bg-muted', selected && 'bg-primary/10')}>
    <div className="flex w-16 items-center gap-1 px-1">
      <Button size="icon-xs" variant="ghost" className="h-6 w-4 shrink-0 cursor-grab px-0 text-muted-foreground/60 opacity-0 group-hover/record:opacity-100 group-focus-within/record:opacity-100 focus-visible:opacity-100 disabled:cursor-not-allowed [@media(hover:none)]:opacity-100" draggable={canMove && !disabled} disabled={!canMove || disabled} aria-label={text.moveRow(number)} title={canMove ? text.moveRow(number) : text.moveRowUnavailable} onDragStart={onDragStart} onDragEnd={onDragEnd} onKeyDown={event => { event.stopPropagation(); if (event.altKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) { event.preventDefault(); onKeyboardMove(event.key === 'ArrowUp' ? -1 : 1) } }}><PhDotsSixVertical className="size-3" aria-hidden="true" /></Button>
      <span className="relative grid h-7 w-10 shrink-0 place-items-center">
        <span aria-hidden="true" className={cn('w-full text-center leading-none group-hover/record:invisible group-focus-within/record:invisible [@media(hover:none)]:invisible', selected && 'invisible')}>{number}</span>
        <Checkbox checked={selected} disabled={disabled} aria-label={text.selectRow(number)} className={cn('absolute opacity-0 group-hover/record:opacity-100 group-focus-within/record:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:opacity-100', selected && 'opacity-100')} onCheckedChange={value => onSelect(value === true)} />
      </span>
    </div>
  </TableCell>
}
