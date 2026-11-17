import type { ComponentProps, ReactNode } from 'react'
import type { ColumnDef, RowData, TableOptions } from '@tanstack/react-table'

import type { AriaAttributes } from 'react'

declare module '@tanstack/react-table' {
  interface ColumnMeta<TData extends RowData, TValue> {
    ariaSort?: AriaAttributes['aria-sort']
    align?: 'start' | 'center'
  }
}

type LoadingState = {
  status: 'loading'
  label?: string
  skeletonRows?: number
}

type ErrorState = {
  status: 'error'
  title?: string
  onRetry?: () => void
  retryLabel?: string
}

type EmptyStateValue = {
  status: 'empty'
  title?: string
  action?: ReactNode
}

type ReadyState<TData> = {
  status: 'ready'
  rows: TData[]
  refreshing?: boolean
  refresh?: 'idle' | 'refreshing' | 'stale_error'
  refreshingLabel?: string
  staleErrorLabel?: string
}

export type DataTableState<TData> =
  | LoadingState
  | ErrorState
  | EmptyStateValue
  | ReadyState<TData>

export type DataTableProps<TData, TValue = unknown> = {
  columns: ColumnDef<TData, TValue>[]
  state: DataTableState<TData>
  className?: string
  tableClassName?: string
  scrollAreaClassName?: string
  caption?: ReactNode
  getRowId?: TableOptions<TData>['getRowId']
  getRowClassName?: (row: TData) => string | undefined
  rowAction?: {
    label: (row: TData) => string
    onActivate: (row: TData) => void
  }
  'aria-label'?: string
  'aria-labelledby'?: string
}

export type DataTableSortButtonProps = ComponentProps<
  typeof import('../primitives/button').Button
>