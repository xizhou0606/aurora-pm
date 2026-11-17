import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { PhWarningCircle } from '@/shared/ui/icons'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { dateInputValue } from '@/shared/lib/date-boundary'
import { workItemPriorityTone, workItemStatusTone } from '@/shared/model'
import { Button, Checkbox, StatusPill, TableActionButton } from '@/shared/ui'
import type {
  UserOptionsState,
  WorkItemChildResource,
  WorkItemRelated,
  WorkItemStatusOption,
} from '../../model/types'
import {
  formatDate,
  formatEstimatedMinutes,
  isWorkItemOverdue,
  kindLabel,
  priorityLabel,
  resolveWorkItemAssignee,
  statusLabel,
  WORK_ITEM_PRIORITY_OPTIONS,
} from '../../model/utils'
import {
  InlineDateField,
  InlineEstimatedField,
  InlineSelectField,
  InlineTextField,
  InlineUserField,
} from '../collection/InlineField'

export function statusOptions(
  row: WorkItemRelated,
  workflow: readonly WorkItemStatusOption[],
): WorkItemStatusOption[] {
  const labels = new Map(workflow.map((option) => [option.value, option.label]))
  return [...new Set([row.status, ...row.capabilities.allowedTransitions].filter(Boolean))]
    .map((value) => ({
      value: String(value),
      label: labels.get(String(value)) || statusLabel(String(value)),
    }))
}

export function useChildListColumns({
  resource,
  users,
  workflow,
  selectedIds,
  setSelectedIds,
  deletingId,
  pending,
  setDeleteRow,
}: {
  resource: WorkItemChildResource
  users: UserOptionsState
  workflow: readonly WorkItemStatusOption[]
  selectedIds: readonly number[]
  setSelectedIds: (updater: (current: readonly number[]) => readonly number[]) => void
  deletingId: number | undefined
  pending: boolean
  setDeleteRow: (row: WorkItemRelated | null) => void
}): ColumnDef<WorkItemRelated>[] {
  const readyUsers = users.status === 'ready' ? users.data : []

  return useMemo<ColumnDef<WorkItemRelated>[]>(() => [
    {
      id: 'selection', size: 48, header: WORK_ITEMS_COPY.list.selection,
      cell: ({ row }) => {
        const selectable = row.original.capabilities.canUpdate
          || row.original.capabilities.canDelete
          || row.original.capabilities.allowedTransitions.length > 0
        return (
          <Checkbox
            checked={selectedIds.includes(row.original.id)}
            disabled={!selectable}
            aria-label={WORK_ITEMS_COPY.list.selectRow(row.original.title || WORK_ITEMS_COPY.list.noTitle)}
            onCheckedChange={() => setSelectedIds((current) => current.includes(row.original.id)
              ? current.filter((id) => id !== row.original.id)
              : [...current, row.original.id])}
          />
        )
      },
    },
    {
      id: 'task_name', size: 320, header: WORK_ITEMS_COPY.list.taskName,
      cell: ({ row }) => (
        <InlineTextField
          id={`work-items-child-title-${row.original.id}`}
          label={WORK_ITEMS_COPY.list.taskName}
          rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
          value={row.original.title || ''}
          display={row.original.title || WORK_ITEMS_COPY.list.noTitle}
          editable={row.original.capabilities.canUpdate}
          mutation={resource.mutation}
          maxLength={200}
          onSave={(value) => resource.update(row.original, { title: value })}
        />
      ),
    },
    {
      id: 'task_number', size: 180, header: WORK_ITEMS_COPY.list.taskNumber,
      cell: ({ row }) => {
        const overdue = isWorkItemOverdue(row.original)
        return (
          <div className="grid min-w-0 gap-0.5">
            <Button
              type="button"
              variant="link"
              size="sm"
              className="h-auto min-w-0 max-w-full justify-start gap-1.5 p-0 font-mono text-xs font-semibold text-primary underline-offset-4 hover:underline"
              aria-label={WORK_ITEMS_COPY.list.openDetail(row.original.title || WORK_ITEMS_COPY.list.noTitle)}
              onClick={() => resource.open(row.original.id, 'view')}
            >
              <span className="min-w-0 truncate">
                {row.original.taskKey || WORK_ITEMS_COPY.list.fallbackId(row.original.id)}
              </span>
              {overdue ? (
                <span
                  className="inline-flex h-5 shrink-0 items-center gap-0.5 rounded-sm border border-destructive px-1 text-xs font-medium text-destructive"
                  title={WORK_ITEMS_COPY.list.overdueTitle}
                >
                  <PhWarningCircle aria-hidden="true" className="size-3" />
                  {WORK_ITEMS_COPY.list.overdue}
                </span>
              ) : null}
            </Button>
            <span className="truncate text-xs text-muted-foreground">{kindLabel(row.original.kind)}</span>
          </div>
        )
      },
    },
    {
      id: 'status', size: 128, header: WORK_ITEMS_COPY.list.status,
      cell: ({ row }) => {
        const options = statusOptions(row.original, workflow)
        return (
          <InlineSelectField
            id={`work-items-child-status-${row.original.id}`}
            label={WORK_ITEMS_COPY.list.status}
            rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
            value={row.original.status || ''}
            display={<StatusPill tone={workItemStatusTone(row.original.status)}>{statusLabel(row.original.status)}</StatusPill>}
            editable={row.original.capabilities.allowedTransitions.length > 0
              && options.some((option) => option.value !== row.original.status)}
            mutation={resource.mutation}
            options={options}
            placeholder={WORK_ITEMS_COPY.filter.status}
            contentClassName="min-w-36"
            onSave={(value) => resource.update(row.original, { status: value })}
          />
        )
      },
    },
    {
      id: 'assignee', size: 144, header: WORK_ITEMS_COPY.list.assignee,
      cell: ({ row }) => {
        const assignee = resolveWorkItemAssignee(row.original, readyUsers)
        return (
          <InlineUserField
            id={`work-items-child-assignee-${row.original.id}`}
            label={WORK_ITEMS_COPY.list.assignee}
            rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
            value={row.original.assigneeId}
            displayName={assignee.name}
            avatarUrl={assignee.user?.avatarUrl}
            editable={row.original.capabilities.canUpdate && users.status === 'ready'}
            mutation={resource.mutation}
            options={readyUsers}
            onSave={(value) => resource.update(row.original, { assigneeId: value })}
          />
        )
      },
    },
    {
      id: 'priority', size: 112, header: WORK_ITEMS_COPY.list.priority,
      cell: ({ row }) => (
        <InlineSelectField
          id={`work-items-child-priority-${row.original.id}`}
          label={WORK_ITEMS_COPY.list.priority}
          rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
          value={row.original.priority || ''}
          display={<StatusPill tone={workItemPriorityTone(row.original.priority)} className="min-w-0">{priorityLabel(row.original.priority)}</StatusPill>}
          editable={row.original.capabilities.canUpdate}
          mutation={resource.mutation}
          options={WORK_ITEM_PRIORITY_OPTIONS}
          placeholder={WORK_ITEMS_COPY.filter.priority}
          onSave={(value) => resource.update(row.original, { priority: value })}
        />
      ),
    },
    {
      id: 'estimated_minutes', size: 112, header: WORK_ITEMS_COPY.list.estimatedTime,
      cell: ({ row }) => (
        <InlineEstimatedField
          id={`work-items-child-estimated-${row.original.id}`}
          label={WORK_ITEMS_COPY.list.estimatedTime}
          rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
          value={row.original.estimatedMinutes}
          display={formatEstimatedMinutes(row.original.estimatedMinutes)}
          editable={row.original.capabilities.canUpdate}
          mutation={resource.mutation}
          onSave={(value) => resource.update(row.original, { estimatedMinutes: value })}
        />
      ),
    },
    {
      id: 'starts_at', size: 160, header: WORK_ITEMS_COPY.list.startsAt,
      cell: ({ row }) => (
        <InlineDateField
          id={`work-items-child-start-${row.original.id}`}
          label={WORK_ITEMS_COPY.list.startsAt}
          rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
          value={dateInputValue(row.original.startsAt)}
          display={formatDate(row.original.startsAt)}
          editable={row.original.capabilities.canUpdate}
          mutation={resource.mutation}
          onSave={(value) => resource.update(row.original, { startsAt: value })}
        />
      ),
    },
    {
      id: 'due_at', size: 160, header: WORK_ITEMS_COPY.list.due,
      cell: ({ row }) => (
        <InlineDateField
          id={`work-items-child-due-${row.original.id}`}
          label={WORK_ITEMS_COPY.list.due}
          rowTitle={row.original.title || WORK_ITEMS_COPY.list.noTitle}
          value={dateInputValue(row.original.dueAt)}
          display={formatDate(row.original.dueAt)}
          editable={row.original.capabilities.canUpdate}
          mutation={resource.mutation}
          onSave={(value) => resource.update(row.original, { dueAt: value })}
        />
      ),
    },
    {
      id: 'actions', size: 120, header: WORK_ITEMS_COPY.list.actions,
      cell: ({ row }) => (
        <div className="flex min-w-max flex-nowrap items-center justify-center gap-1">
          {row.original.capabilities.canUpdate ? (
            <TableActionButton
              text={WORK_ITEMS_COPY.mutation.edit}
              label={WORK_ITEMS_COPY.mutation.edit}
              disabled={pending}
              onClick={() => resource.open(row.original.id, 'edit')}
            />
          ) : null}
          {row.original.capabilities.canDelete ? (
            <TableActionButton
              text={WORK_ITEMS_COPY.mutation.delete}
              label={WORK_ITEMS_COPY.mutation.delete}
              disabled={deletingId === row.original.id || pending}
              onClick={() => setDeleteRow(row.original)}
            />
          ) : null}
        </div>
      ),
    },
  ], [deletingId, pending, readyUsers, resource, selectedIds, setSelectedIds, users.status, workflow, setDeleteRow])
}
