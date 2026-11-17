import {
  flexRender,
  type Table as TanStackTable,
} from '@tanstack/react-table'
import type { KeyboardEvent, MouseEvent, ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'
import { EmptyState } from '../feedback/empty-state'
import { LoadingSkeleton } from '../feedback/loading-skeleton'
import {
  TableCell,
  TableRow,
} from './table'

import type { DataTableState } from './data-table-types'

type DataTableBodyProps<TData> = {
  state: DataTableState<TData>
  table: TanStackTable<TData>
  columnCount: number
  rowAction?: {
    label: (row: TData) => string
    onActivate: (row: TData) => void
  }
  getRowClassName?: (row: TData) => string | undefined
}

const dataTableCopy = copy.ui.dataTable

// Wide tables (w-max/table-fixed) overflow the viewport even with zero rows, so a
// state cell spanning every column is wider than what is visible. Pin the state
// panel to the visible left edge and size it to the scroll shell (100cqw) so
// loading/empty/error content centers within the viewport, not the table.
const statePanelClassName = 'sticky left-0 w-[100cqw] p-4'

function interactiveElement(target: EventTarget | null): Element | null {
  if (target instanceof Element) return target
  return target instanceof Node ? target.parentElement : null
}

function isInteractiveTarget(target: EventTarget | null): boolean {
  return Boolean(interactiveElement(target)?.closest(
    'a,button,input,select,textarea,[role="button"],[role="link"],[role="menuitem"],[contenteditable="true"]',
  ))
}

export function DataTableBody<TData>({
  state,
  table,
  columnCount,
  rowAction,
  getRowClassName,
}: DataTableBodyProps<TData>) {
  'use no memo'

  switch (state.status) {
    case 'loading':
      return (
        <TableRow className="hover:bg-transparent">
          <td colSpan={columnCount} className="h-auto p-0 whitespace-normal align-middle">
            <div className={statePanelClassName}>
              <LoadingSkeleton
                rows={state.skeletonRows}
                label={state.label ?? dataTableCopy.loadingLabel}
              />
            </div>
          </td>
        </TableRow>
      )

    case 'error':
      return (
        <TableRow className="hover:bg-transparent">
          <td colSpan={columnCount} className="h-auto p-0 whitespace-normal align-middle">
            <div className={statePanelClassName}>
              <div role="alert">
                <EmptyState
                  name="hugeicons:alert-circle"
                  title={state.title ?? dataTableCopy.errorTitle}
                  action={
                    state.onRetry ? (
                      <Button variant="outline" size="sm" onClick={state.onRetry}>
                        {state.retryLabel ?? dataTableCopy.retryLabel}
                      </Button>
                    ) : undefined
                  }
                />
              </div>
            </div>
          </td>
        </TableRow>
      )

    case 'empty':
      return (
        <TableRow className="hover:bg-transparent">
          <td colSpan={columnCount} className="h-auto p-0 whitespace-normal align-middle">
            <div className={statePanelClassName}>
              <div role="status">
                <EmptyState
                  title={state.title ?? dataTableCopy.emptyTitle}
                  action={state.action}
                />
              </div>
            </div>
          </td>
        </TableRow>
      )

    case 'ready':
      return table.getRowModel().rows.map((row) => {
        const activate = () => rowAction?.onActivate(row.original)
        const onClick = rowAction
          ? (event: MouseEvent<HTMLTableRowElement>) => {
              if (!isInteractiveTarget(event.target)) activate()
            }
          : undefined
        const onKeyDown = rowAction
          ? (event: KeyboardEvent<HTMLTableRowElement>) => {
              if (event.target !== event.currentTarget) return
              if (event.key !== 'Enter' && event.key !== ' ') return
              event.preventDefault()
              activate()
            }
          : undefined
        return (
          <TableRow
            key={row.id}
            aria-label={rowAction?.label(row.original)}
            className={cn(
              'group/row bg-background',
              // eslint-disable-next-line shadcn/require-static-classes -- runtime getRowClassName callback result; the rule cannot trace function calls
              getRowClassName?.(row.original),
              rowAction
                ? 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring'
                : undefined,
            )}
            data-row-activatable={rowAction ? 'true' : undefined}
            tabIndex={rowAction ? 0 : undefined}
            onClick={onClick}
            onKeyDown={onKeyDown}
          >
            {row.getVisibleCells().map((cell) => (
              <TableCell
                key={cell.id}
                data-column={cell.column.id}
                data-action-column={cell.column.id === 'actions' ? 'true' : undefined}
                style={cell.column.columnDef.size
                  ? { '--data-table-column-size': `${cell.column.columnDef.size}px` }
                  : undefined}
                className={cn(
                  cell.column.columnDef.meta?.align === 'center' && 'text-center',
                  cell.column.id === 'actions'
                    && 'pm-data-table-action-column sticky right-0 z-10 min-w-[4.5rem] bg-inherit px-2 text-center transition-colors',
                  cell.column.columnDef.size && 'w-(--data-table-column-size) min-w-(--data-table-column-size)',
                )}
              >
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        )
      })
  }
}