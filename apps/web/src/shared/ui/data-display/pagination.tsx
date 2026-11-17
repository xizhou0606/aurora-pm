import * as React from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'
import { Label } from '../primitives/label'

const DEFAULT_PAGE_SIZES = [10, 20, 50, 100] as const

type PaginationProps = Omit<React.ComponentProps<'nav'>, 'children'> & {
  page: number
  pageSize: number
  total: number
  pageSizes?: readonly number[]
  showPageSize?: boolean
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}

function positiveInteger(value: number, fallback: number): number {
  return Number.isFinite(value) && value > 0 ? Math.floor(value) : fallback
}

function normalizedPageSizes(pageSizes: readonly number[], pageSize: number): number[] {
  const values = pageSizes
    .map((value) => positiveInteger(value, 0))
    .filter((value, index, source) => value > 0 && source.indexOf(value) === index)
  const fallback = values[0] ?? DEFAULT_PAGE_SIZES[0]
  const current = positiveInteger(pageSize, fallback)
  if (!values.includes(current)) values.push(current)
  return values.sort((left, right) => left - right)
}

function Pagination({
  page,
  pageSize,
  total,
  pageSizes = DEFAULT_PAGE_SIZES,
  showPageSize = true,
  onPageChange,
  onPageSizeChange,
  className,
  'aria-label': ariaLabel = copy.ui.pagination.label,
  ...props
}: PaginationProps) {
  const sizes = normalizedPageSizes(pageSizes, pageSize)
  const normalizedPageSize = positiveInteger(pageSize, sizes[0])
  const normalizedTotal = Number.isFinite(total) && total > 0 ? Math.floor(total) : 0
  const pageCount = Math.max(1, Math.ceil(normalizedTotal / normalizedPageSize))
  const currentPage = Math.min(positiveInteger(page, 1), pageCount)
  const pageSizeControlId = React.useId()
  const previousDisabled = currentPage <= 1
  const nextDisabled = currentPage >= pageCount

  return (
    <nav
      data-slot="pagination"
      aria-label={ariaLabel}
      className={cn(
        'flex flex-col gap-2 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between',
        className,
      )}
      {...props}
    >
      <span data-slot="pagination-total" className="tabular-nums">
        {copy.ui.pagination.total(normalizedTotal)}
      </span>

      <div className="flex flex-wrap items-center gap-2">
        {showPageSize ? <div className="flex items-center gap-1.5">
          <Label htmlFor={pageSizeControlId}>{copy.ui.pagination.pageSizeLabel}</Label>
          <select
            id={pageSizeControlId}
            data-slot="pagination-page-size"
            value={normalizedPageSize}
            onChange={(event) => onPageSizeChange(Number(event.currentTarget.value))}
            className="h-8 cursor-pointer rounded-lg border border-input bg-background px-2 text-xs text-foreground shadow-[var(--design-shadow-card)] outline-none transition-control hover:bg-muted focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/25 focus-visible:shadow-none disabled:cursor-not-allowed dark:bg-input/30"
          >
            {sizes.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </div> : null}

        <span data-slot="pagination-page" className="min-w-16 text-center tabular-nums">
          {copy.ui.pagination.page(currentPage, pageCount)}
        </span>

        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={previousDisabled}
            onClick={() => onPageChange(Math.max(1, currentPage - 1))}
          >
            {copy.ui.pagination.previous}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={nextDisabled}
            onClick={() => onPageChange(Math.min(pageCount, currentPage + 1))}
          >
            {copy.ui.pagination.next}
          </Button>
        </div>
      </div>
    </nav>
  )
}

export { Pagination }
export type { PaginationProps }
