import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { PhCalendarCheck, PhWarningCircle, PhClock } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { workItemStatusTone } from '@/shared/model'
import {
  DataTable,
  StatusPill,
  TableActionButton,
  UserPreview,
} from '@/shared/ui'
import type { TaskListState, TaskRowModel } from '../model'
import type { TaskListActions } from './ContentLists'

const contentCopy = copy.projectManagement.content

const TASK_TABLE_LAYOUT = 'min-w-max'

type TaskRowAction = {
  edit?: () => void
  delete?: () => void
}

function TaskTable({
  rows,
  requirementLabels,
  actions,
}: {
  rows: TaskRowModel[]
  requirementLabels: ReadonlyMap<number, string>
  actions: TaskListActions
}) {
  const columns = useMemo<ColumnDef<TaskRowModel>[]>(() => [
    {
      id: 'task_number',
      size: 180,
      header: contentCopy.task.columnKey,
      cell: ({ row }) => {
        const overdue = row.original.overdue
        return (
          <div className="flex min-w-0 items-center gap-1.5">
            {overdue ? (
              <PhWarningCircle aria-hidden="true" className="size-3.5 shrink-0 text-destructive" />
            ) : null}
            <span className="truncate font-mono text-xs font-semibold text-foreground">
              {row.original.key}
            </span>
          </div>
        )
      },
    },
    {
      id: 'task_name',
      size: 320,
      header: contentCopy.task.columnTitle,
      cell: ({ row }) => {
        const t = row.original
        const requirementLabel = requirementLabels.get(t.requirementId)
        return (
          <div className="grid min-w-0 gap-0.5">
            <span className="truncate text-sm font-semibold text-foreground">
              {t.title}
            </span>
            <span className="truncate text-xs text-muted-foreground">
              {requirementLabel ?? ''}
            </span>
          </div>
        )
      },
    },
    {
      id: 'status',
      size: 130,
      header: contentCopy.task.columnStatus,
      cell: ({ row }) => {
        const t = row.original
        return (
          <StatusPill tone={workItemStatusTone(t.statusKey)}>
            {t.status}
          </StatusPill>
        )
      },
    },
    {
      id: 'assignee',
      size: 200,
      header: contentCopy.task.columnAssignee,
      cell: ({ row }) => {
        const t = row.original
        return (
          <div className="flex min-w-0 items-center">
            <UserPreview
              name={t.assignee.name}
              secondary={t.assignee.secondary}
              avatarUrl={t.assignee.avatarUrl}
              size="sm"
              className={cn('max-w-40', t.unassigned && 'text-amber-700 dark:text-amber-400')}
            />
          </div>
        )
      },
    },
    {
      id: 'priority',
      size: 110,
      header: contentCopy.task.columnPriority,
      cell: ({ row }) => {
        const t = row.original
        return (
          <StatusPill tone={['high', 'urgent'].includes(t.priorityKey) ? 'warning' : 'neutral'}>
            {t.priority}
          </StatusPill>
        )
      },
    },
    {
      id: 'due_at',
      size: 150,
      header: contentCopy.task.columnDue,
      cell: ({ row }) => {
        const t = row.original
        return (
          <span className={cn(
            'inline-flex items-center gap-1 text-xs text-muted-foreground',
            t.overdue && 'font-medium text-destructive',
          )}>
            <PhCalendarCheck aria-hidden="true" className="size-3.5" />
            {t.dueAt ? t.due : contentCopy.task.dueUnset}
            {t.overdue ? ` · ${contentCopy.task.dueOverdue}` : ''}
          </span>
        )
      },
    },
    {
      id: 'estimate',
      size: 110,
      header: contentCopy.task.estimateLabel,
      cell: ({ row }) => {
        const t = row.original
        return t.estimate ? (
          <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
            <PhClock aria-hidden="true" className="size-3.5" />
            {t.estimate}
          </span>
        ) : null
      },
    },
    {
      id: 'actions',
      size: 120,
      header: contentCopy.common.actions,
      cell: ({ row }) => {
        const t = row.original
        const item = actions.items.get(t.id) as TaskRowAction | undefined
        return item ? (
          <div className="flex items-center justify-end gap-1">
            {item.edit ? (
              <TableActionButton
                label={contentCopy.task.editLabel(t.title)}
                text={contentCopy.common.edit}
                onClick={item.edit}
              />
            ) : null}
            {item.delete ? (
              <TableActionButton
                label={contentCopy.task.deleteLabel(t.title)}
                text={contentCopy.common.delete}
                onClick={item.delete}
              />
            ) : null}
          </div>
        ) : null
      },
    },
  ], [actions.items, requirementLabels])

  return (
    <DataTable
      aria-label={contentCopy.task.tableLabel}
      columns={columns}
      getRowId={(row) => String(row.id)}
      tableClassName={TASK_TABLE_LAYOUT}
      state={{ status: 'ready', rows }}
    />
  )
}

export function TaskCards({
  state,
  requirementLabels,
  actions,
}: {
  state: Extract<TaskListState, { status: 'ready' }>
  requirementLabels: ReadonlyMap<number, string>
  actions: TaskListActions
}) {
  const overdue = state.rows.filter((row) => row.overdue).length
  const unassigned = state.rows.filter((row) => row.unassigned).length
  return (
    <div className="overflow-hidden rounded-lg border border-border bg-card">
      <header className="flex flex-col gap-2 border-b border-border bg-muted/20 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <span className="flex flex-wrap items-center gap-2">
          <strong className="text-sm font-semibold text-foreground">
            {contentCopy.task.listSummary(state.total)}
          </strong>
          <span className="text-xs text-muted-foreground">{contentCopy.task.currentPageEvidence}</span>
        </span>
        <span className="flex flex-wrap gap-2">
          {overdue > 0 ? <StatusPill tone="danger">{contentCopy.task.overdueCount(overdue)}</StatusPill> : null}
          {unassigned > 0 ? <StatusPill tone="warning">{contentCopy.task.unassignedCount(unassigned)}</StatusPill> : null}
          {overdue === 0 && unassigned === 0 ? (
            <span className="text-xs text-emerald-700 dark:text-emerald-400">{contentCopy.task.noExecutionIssues}</span>
          ) : null}
        </span>
      </header>
      <TaskTable rows={state.rows} requirementLabels={requirementLabels} actions={actions} />
    </div>
  )
}
