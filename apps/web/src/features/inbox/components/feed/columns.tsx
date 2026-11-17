import type { ColumnDef } from '@tanstack/react-table'

import { INBOX_COPY } from '@/shared/copy'
import { workItemStatusTone } from '@/shared/model'
import { StatusPill } from '@/shared/ui'
import {
  bugScope,
  bugStatusLabel,
  formatDate,
  formatDateTime,
  priorityLabel,
  severityLabel,
  taskStatusLabel,
  workflowActionLabel,
  workflowScope,
  type InboxBug,
  type InboxTask,
  type InboxWorkflowTodo,
} from '../../model'
import { FeedIdentity } from './Identity'

export function taskColumns(
  id: 'task' | 'mention',
  onOpen: (row: InboxTask) => void,
): ColumnDef<InboxTask>[] {
  return [
    {
      id: 'identity',
      header: INBOX_COPY.feed[id].item,
      cell: ({ row }) => (
        <FeedIdentity
          itemKey={row.original.taskKey || INBOX_COPY.feed.fallbackId(row.original.id)}
          title={row.original.title || INBOX_COPY.feed.unnamedTask}
          scope={row.original.requirementTitle || INBOX_COPY.feed.noRequirement}
          meta={id === 'mention'
            ? formatDateTime(row.original.mentionAt)
            : priorityLabel(row.original.priority)}
          onOpen={() => onOpen(row.original)}
        />
      ),
    },
    {
      accessorKey: 'status',
      header: INBOX_COPY.feed.status,
      cell: ({ row }) => (
        <StatusPill tone={workItemStatusTone(row.original.status)}>
          {taskStatusLabel(row.original.status)}
        </StatusPill>
      ),
    },
    ...(id === 'task' ? [{
      accessorKey: 'priority',
      header: INBOX_COPY.feed.priority,
      cell: ({ row }: { row: { original: InboxTask } }) => (
        priorityLabel(row.original.priority)
      ),
    }, {
      accessorKey: 'dueAt',
      header: INBOX_COPY.feed.due,
      cell: ({ row }: { row: { original: InboxTask } }) => formatDate(row.original.dueAt),
    }] : [{
      accessorKey: 'mentionAt',
      header: INBOX_COPY.feed.mentionAt,
      cell: ({ row }: { row: { original: InboxTask } }) => (
        formatDateTime(row.original.mentionAt)
      ),
    }]),
  ]
}

export function workflowColumns(
  onOpen: (row: InboxWorkflowTodo) => void,
): ColumnDef<InboxWorkflowTodo>[] {
  return [
    {
      id: 'identity',
      header: INBOX_COPY.feed.workflow.item,
      cell: ({ row }) => (
        <FeedIdentity
          itemKey={row.original.templateName || INBOX_COPY.feed.workflowFallback(row.original.workflowInstanceId)}
          title={row.original.title}
          scope={workflowScope(row.original)}
          meta={workflowActionLabel(row.original.actionKind)}
          onOpen={() => onOpen(row.original)}
        />
      ),
    },
    {
      accessorKey: 'actionKind',
      header: INBOX_COPY.feed.action,
      cell: ({ row }) => (
        <StatusPill tone={row.original.actionKind === 'ai_retry'
          ? 'danger'
          : row.original.actionKind === 'ai_confirm'
            ? 'accent'
            : row.original.actionKind === 'approval'
              ? 'warning'
              : 'info'}>
          {workflowActionLabel(row.original.actionKind)}
        </StatusPill>
      ),
    },
    {
      accessorKey: 'dueAt',
      header: INBOX_COPY.feed.due,
      cell: ({ row }) => formatDate(row.original.dueAt),
    },
  ]
}

export function bugColumns(onOpen: (row: InboxBug) => void): ColumnDef<InboxBug>[] {
  return [
    {
      id: 'identity',
      header: INBOX_COPY.feed.bug.item,
      cell: ({ row }) => (
        <FeedIdentity
          itemKey={row.original.bugKey || INBOX_COPY.feed.fallbackId(row.original.id)}
          title={row.original.title || INBOX_COPY.feed.unnamedBug}
          scope={bugScope(row.original)}
          meta={severityLabel(row.original.severity)}
          onOpen={() => onOpen(row.original)}
        />
      ),
    },
    {
      accessorKey: 'status',
      header: INBOX_COPY.feed.status,
      cell: ({ row }) => <StatusPill>{bugStatusLabel(row.original.status)}</StatusPill>,
    },
    {
      accessorKey: 'severity',
      header: INBOX_COPY.feed.severity,
      cell: ({ row }) => <StatusPill tone="warning">{severityLabel(row.original.severity)}</StatusPill>,
    },
    {
      accessorKey: 'dueDate',
      header: INBOX_COPY.feed.due,
      cell: ({ row }) => formatDate(row.original.dueDate),
    },
  ]
}
