import { useRef, useState, type ComponentProps, type ReactNode } from 'react'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button, ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger, TableCell, TableRow } from '@/shared/ui'
import { BitableInput as Input } from '../controls'
import { PhArrowDown, PhArrowUp, PhArrowSquareOut, PhChatCenteredText, PhDotsThree, PhLink, PhShareNetwork, PhTrash } from '@/shared/ui/icons'
import type { GridProps } from '../../model/grid-port'
import type { Row } from '../../model/types'

type Props = Pick<GridProps, 'onInsertRows' | 'onRecordOpen' | 'onRecordLink'> & {
  row: Row; number: number; disabled: boolean; canCreate: boolean; canDelete: boolean; canComment?: boolean
  rowProps: ComponentProps<typeof TableRow>; children: ReactNode; onDelete: () => void
}

export function GridRowMenu({ row, number, disabled, canCreate, canDelete, canComment, rowProps, children, onDelete, onInsertRows, onRecordOpen, onRecordLink }: Props) {
  const trigger = useRef<HTMLDivElement>(null)
  const [count, setCount] = useState('1')
  const countValue = Number(count)
  const validCount = Number.isInteger(countValue) && countValue >= 1 && countValue <= 100
  const hasActions = canDelete || (canCreate && onInsertRows) || onRecordOpen || onRecordLink
  const insert = (direction: 'before' | 'after') => { if (!disabled && validCount) void onInsertRows?.(row, direction, countValue) }
  return <ContextMenu onOpenChange={(open, details) => { if (open && (!hasActions || disabled)) { details.cancel(); return }; if (open) setCount('1') }}>
    <ContextMenuTrigger ref={trigger} render={<TableRow {...rowProps} />} tabIndex={-1}>
      {children}
      <TableCell className="sticky right-0 z-10 w-10 border-l border-border/50 bg-card p-0 text-center group-hover/record:bg-muted group-focus-within/record:bg-muted">
        {hasActions ? <Button type="button" size="icon-xs" variant="ghost" aria-label={text.rowActions(number)} title={text.rowActions(number)}
          className="opacity-100 [@media(hover:hover)]:opacity-0 group-hover/record:opacity-100 group-focus-within/record:opacity-100 focus-visible:opacity-100" disabled={disabled}
          onKeyDown={(event) => event.stopPropagation()} onClick={(event) => {
            event.stopPropagation()
            const bounds = event.currentTarget.getBoundingClientRect()
            trigger.current?.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: bounds.left, clientY: bounds.bottom }))
          }}><PhDotsThree aria-hidden="true" /></Button> : null}
      </TableCell>
    </ContextMenuTrigger>
    <ContextMenuContent aria-label={text.rowActions(number)} className="w-56" onKeyDown={(event) => event.stopPropagation()} onPointerDown={(event) => event.stopPropagation()}>
      {canCreate && onInsertRows ? <>
        <div className="flex items-center gap-2 px-1.5 py-1 text-xs"><span className="flex-1">{text.insertCount}</span><Input type="number" min={1} max={100} step={1} value={count} aria-label={text.insertCount} className="h-7 w-16 px-2 text-xs" onChange={(event) => setCount(event.target.value)} onKeyDown={(event) => event.stopPropagation()} />{text.rowUnit}</div>
        <ContextMenuItem disabled={disabled || !validCount} onClick={() => insert('before')}><PhArrowUp aria-hidden="true" />{text.insertAbove}</ContextMenuItem>
        <ContextMenuItem disabled={disabled || !validCount} onClick={() => insert('after')}><PhArrowDown aria-hidden="true" />{text.insertBelow}</ContextMenuItem>
        <ContextMenuSeparator />
      </> : null}
      {onRecordLink ? <>
        <ContextMenuItem disabled={disabled} onClick={() => onRecordLink(row, true)}><PhShareNetwork aria-hidden="true" />{text.shareRecord}</ContextMenuItem>
        <ContextMenuItem disabled={disabled} onClick={() => onRecordLink(row, false)}><PhLink aria-hidden="true" />{text.copyRecordLink}</ContextMenuItem>
        <ContextMenuSeparator />
      </> : null}
      {onRecordOpen ? <>
        <ContextMenuItem disabled={disabled} onClick={() => onRecordOpen(row, 'details')}><PhArrowSquareOut aria-hidden="true" />{text.recordDetails}</ContextMenuItem>
        {canComment ? <ContextMenuItem disabled={disabled} onClick={() => onRecordOpen(row, 'comments')}><PhChatCenteredText aria-hidden="true" />{text.recordComment}</ContextMenuItem> : null}
      </> : null}
      {canDelete ? <ContextMenuItem destructive disabled={disabled} onClick={onDelete}><PhTrash aria-hidden="true" />{text.deleteRow}</ContextMenuItem> : null}
    </ContextMenuContent>
  </ContextMenu>
}
