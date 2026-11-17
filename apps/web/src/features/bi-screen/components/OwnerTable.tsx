import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { DataTable, UserPreview } from '@/shared/ui'

import type { QualityOwnerLoadItem } from '../types'

function LoadCell({ value, maximum, suffix = '', tone = 'blue' }: {
  value: number
  maximum: number
  suffix?: string
  tone?: 'blue' | 'red' | 'orange'
}) {
  const width = Math.max(0, Math.min(100, Math.round((value / Math.max(1, maximum)) * 100)))
  return (
    <span className="grid min-w-14 gap-1 text-xs tabular-nums">
      <span>{value}{suffix}</span>
      <span aria-hidden="true" className="h-1.5 overflow-hidden rounded-full bg-muted">
        <span
          className={cn(
            'block h-full w-(--load-w) rounded-full',
            tone === 'blue' && 'bg-primary',
            tone === 'red' && 'bg-red-500',
            tone === 'orange' && 'bg-orange-500',
          )}
          style={{ '--load-w': `${width}%` }}
        />
      </span>
    </span>
  )
}

export function OwnerTable({ items }: { items: readonly QualityOwnerLoadItem[] }) {
  const maximum = Math.max(1, ...items.map((item) => item.open_bugs))
  const columns = useMemo<ColumnDef<QualityOwnerLoadItem>[]>(() => [
    {
      accessorKey: 'owner_name',
      size: 148,
      header: BI_SCREEN_COPY.ownerTable.owner,
      cell: ({ row }) => (
        <UserPreview
          name={row.original.owner_name}
          avatarUrl={row.original.owner_avatar_url}
        />
      ),
    },
    {
      accessorKey: 'pending_bugs',
      size: 92,
      header: BI_SCREEN_COPY.ownerTable.pending,
      cell: ({ row }) => <LoadCell value={row.original.pending_bugs} maximum={maximum} tone="red" />,
    },
    {
      accessorKey: 'review_bugs',
      size: 92,
      header: BI_SCREEN_COPY.ownerTable.review,
      cell: ({ row }) => <LoadCell value={row.original.review_bugs} maximum={maximum} tone="orange" />,
    },
    {
      accessorKey: 'overdue_bugs',
      size: 92,
      header: BI_SCREEN_COPY.ownerTable.overdue,
      cell: ({ row }) => <LoadCell value={row.original.overdue_bugs} maximum={maximum} tone="red" />,
    },
    {
      accessorKey: 'open_bugs',
      size: 92,
      header: BI_SCREEN_COPY.ownerTable.openTotal,
      cell: ({ row }) => <LoadCell value={row.original.open_bugs} maximum={maximum} />,
    },
    {
      accessorKey: 'high_severity_pct',
      size: 84,
      header: BI_SCREEN_COPY.ownerTable.highSeverityPercent,
      cell: ({ row }) => (
        <span className="text-xs tabular-nums text-muted-foreground">
          {BI_SCREEN_COPY.ownerTable.percent(row.original.high_severity_pct)}
        </span>
      ),
    },
  ], [maximum])
  return (
    <DataTable
      aria-label={BI_SCREEN_COPY.ownerTable.ariaLabel}
      columns={columns}
      tableClassName="table-fixed"
      state={{ status: 'ready', rows: [...items] }}
      getRowId={(row) => String(row.owner_id || row.owner_name)}
    />
  )
}
