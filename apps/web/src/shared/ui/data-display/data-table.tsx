import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type RowData,
} from '@tanstack/react-table'
import type { ComponentProps, ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'
import {
  Table,
  TableBody,
  TableCaption,
  TableHead,
  TableHeader,
  TableRow,
} from './table'
import { DataTableBody } from './data-table-body'
import { useDataTableScrollbar } from './data-table-scrollbar'
import { DataTableScrollbarElements } from './data-table-scrollbar-elements'
import type { DataTableProps, DataTableSortButtonProps, DataTableState } from './data-table-types'

// Re-export types for consumers
export type { DataTableProps, DataTableState }

// Module augmentation for column meta
declare module '@tanstack/react-table' {
  interface ColumnMeta<TData extends RowData, TValue> {
    ariaSort?: 'ascending' | 'descending' | 'none' | 'other'
    align?: 'start' | 'center'
  }
}

function DataTableSortButton({ className, ...props }: DataTableSortButtonProps) {
  return (
    <Button
      type="button"
      size="default"
      variant="ghost"
      className={cn('-ml-2 h-8 px-2 text-xs font-semibold text-muted-foreground', className)}
      {...props}
    />
  )
}

const dataTableCopy = copy.ui.dataTable

function DataTable<TData, TValue = unknown>({
  className,
  tableClassName,
  scrollAreaClassName,
  caption,
  columns,
  getRowClassName,
  getRowId,
  rowAction,
  state,
  'aria-label': ariaLabel,
  'aria-labelledby': ariaLabelledBy,
}: DataTableProps<TData, TValue>) {
  'use no memo'

  const data = state.status === 'ready' ? state.rows : []
  const bodyScrollEnabled = Boolean(scrollAreaClassName?.trim())
  const table = useReactTable({
    columns,
    data,
    getCoreRowModel: getCoreRowModel(),
    getRowId,
  })
  const columnCount = Math.max(table.getVisibleLeafColumns().length, 1)
  const refreshing = state.status === 'ready' && (state.refreshing || state.refresh === 'refreshing')
  const staleError = state.status === 'ready' && state.refresh === 'stale_error'
  const busy = state.status === 'loading' || refreshing
  const hasActionColumn = table.getAllLeafColumns().some((column) => column.id === 'actions')

  const scrollbar = useDataTableScrollbar({
    bodyScrollEnabled,
    columnCount,
    hasActionColumn,
    status: state.status,
  })

  return (
    <div
      ref={scrollbar.rootRef}
      data-slot="data-table"
      data-action-shadow={hasActionColumn ? scrollbar.actionShadowVisible ? 'visible' : 'hidden' : undefined}
      aria-busy={busy}
      className={cn('w-full min-w-0 max-w-full overflow-hidden rounded-xl border border-border/70 bg-card shadow-xs', className)}
    >
      {refreshing ? (
        <span role="status" className="sr-only">
          {state.refreshingLabel ?? dataTableCopy.refreshingLabel}
        </span>
      ) : null}
      {staleError ? (
        <div role="alert" className="border-b bg-muted/40 px-3 py-2 text-sm text-foreground">
          {state.staleErrorLabel ?? dataTableCopy.staleErrorLabel}
        </div>
      ) : null}

      <div data-slot="table-scroll-shell" className="relative w-full min-w-0 @container">
        <Table
          // eslint-disable-next-line shadcn/require-static-classes -- pass-through of the DataTable tableClassName prop; the rule cannot trace non-className parameters across components
          className={tableClassName}
          containerClassName={cn(
            bodyScrollEnabled && 'pm-data-table-scroll-area overflow-auto',
            // eslint-disable-next-line shadcn/require-static-classes -- pass-through of the DataTable scrollAreaClassName prop; the rule cannot trace non-className parameters across components
            scrollAreaClassName,
          )}
          aria-label={ariaLabel}
          aria-labelledby={ariaLabelledBy}
        >
          {caption ? <TableCaption>{caption}</TableCaption> : null}
          <TableHeader className="bg-muted">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    colSpan={header.colSpan}
                    aria-sort={header.column.columnDef.meta?.ariaSort}
                    data-column={header.column.id}
                    data-action-column={header.column.id === 'actions' ? 'true' : undefined}
                    style={header.column.columnDef.size
                      ? { '--data-table-column-size': `${header.column.columnDef.size}px` }
                      : undefined}
                    className={cn(
                      'bg-muted text-xs font-semibold',
                      bodyScrollEnabled && 'sticky top-0 z-20 bg-muted',
                      header.column.columnDef.meta?.align === 'center' && 'text-center',
                      header.column.id === 'actions'
                        && cn(
                          'pm-data-table-action-column sticky right-0 min-w-[4.5rem] bg-muted text-center',
                          bodyScrollEnabled ? 'z-30' : 'z-20',
                        ),
                      header.column.columnDef.size && 'w-(--data-table-column-size) min-w-(--data-table-column-size)',
                    )}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>

          <TableBody>
            <DataTableBody
              state={state}
              table={table}
              columnCount={columnCount}
              rowAction={rowAction}
              // eslint-disable-next-line shadcn/require-static-classes -- pass-through of the DataTable getRowClassName callback; the rule cannot trace function-valued class props
              getRowClassName={getRowClassName}
            />
          </TableBody>
        </Table>
        <DataTableScrollbarElements
          bodyScrollEnabled={bodyScrollEnabled}
          verticalTrackRef={scrollbar.verticalTrackRef}
          verticalThumbRef={scrollbar.verticalThumbRef}
          horizontalTrackRef={scrollbar.horizontalTrackRef}
          horizontalThumbRef={scrollbar.horizontalThumbRef}
          onVerticalKeyDown={scrollbar.handleVerticalKeyDown}
          onVerticalPointerDown={scrollbar.handleVerticalPointerDown}
          onVerticalPointerMove={scrollbar.handleVerticalPointerMove}
          onVerticalPointerUp={scrollbar.handleVerticalPointerUp}
          onHorizontalKeyDown={scrollbar.handleHorizontalKeyDown}
          onHorizontalPointerDown={scrollbar.handleHorizontalPointerDown}
          onHorizontalPointerMove={scrollbar.handleHorizontalPointerMove}
          onHorizontalPointerUp={scrollbar.handleHorizontalPointerUp}
        />
      </div>
    </div>
  )
}

export { DataTable, DataTableSortButton }