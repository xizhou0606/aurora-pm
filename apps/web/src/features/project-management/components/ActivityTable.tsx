import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'

import { copy } from '@/shared/copy'
import { Button, DataTable, UserPreview, type DataTableState } from '@/shared/ui'
import {
  formatPMDateTime,
  type ActivityState,
  type AuditDisplayRow,
} from '../model'

const activityCopy = copy.projectManagement.activity

function assertNever(value: never): never {
  throw new Error(`Unhandled activity state: ${JSON.stringify(value)}`)
}

function tableState(state: ActivityState): DataTableState<AuditDisplayRow> {
  const scopedCopy = state.scope === 'plan-unit'
    ? activityCopy.planUnit
    : activityCopy.project
  switch (state.status) {
    case 'loading':
      return { status: 'loading' }
    case 'error':
      return {
        status: 'error',
        title: activityCopy.errorTitle,
        onRetry: state.retry,
        retryLabel: copy.projectManagement.common.retry,
      }
    case 'empty':
      return {
        status: 'empty',
        title: scopedCopy.emptyTitle
      }
    case 'ready':
      return {
        status: 'ready',
        rows: state.rows,
      }
    default:
      return assertNever(state)
  }
}

export function ActivityTable({ state }: { state: ActivityState }) {
  const scopedCopy = state.scope === 'plan-unit'
    ? activityCopy.planUnit
    : activityCopy.project
  const refresh = state.status === 'empty' || state.status === 'ready'
    ? state.refresh
    : undefined
  const columns = useMemo<ColumnDef<AuditDisplayRow>[]>(() => [
    { id: 'time', header: activityCopy.columnTime, cell: ({ row }) => <time dateTime={row.original.created_at || undefined}>{formatPMDateTime(row.original.created_at)}</time> },
    {
      id: 'actor',
      header: activityCopy.columnUser,
      cell: ({ row }) => (
        <UserPreview
          name={row.original.actor.name}
          secondary={row.original.actor.secondary}
          avatarUrl={row.original.actor.avatarUrl}
          size="sm"
        />
      ),
    },
    { accessorKey: 'actionLabel', header: activityCopy.columnAction },
    { id: 'target', header: activityCopy.columnTarget, cell: ({ row }) => activityCopy.targetLabel(row.original.resourceLabel, row.original.target || copy.projectManagement.common.unknownValue) },
    { id: 'detail', header: activityCopy.columnDetail, cell: ({ row }) => row.original.detail || copy.projectManagement.common.unknownValue },
    { accessorKey: 'resultLabel', header: activityCopy.columnResult },
  ], [])

  return (
    <section
      aria-labelledby="pm-activity-heading"
      aria-busy={refresh?.status === 'refreshing'}
      className="grid min-w-0 gap-3"
    >
      <div><h2 id="pm-activity-heading" className="text-sm font-semibold">{activityCopy.heading}</h2><p className="mt-0.5 text-xs text-muted-foreground">{scopedCopy.description}</p></div>
      {refresh?.status === 'refreshing' ? (
        <span role="status" className="sr-only">{activityCopy.refreshing}</span>
      ) : null}
      {refresh?.status === 'stale_error' ? (
        <div role="alert" className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-muted px-3 py-2 text-sm">
          <span>{activityCopy.staleError}</span>
          <Button type="button" size="xs" variant="outline" onClick={refresh.retry}>
            {copy.projectManagement.common.retry}
          </Button>
        </div>
      ) : null}
      <DataTable aria-label={scopedCopy.tableLabel} columns={columns} getRowId={(row) => String(row.id)} state={tableState(state)} />
    </section>
  )
}
