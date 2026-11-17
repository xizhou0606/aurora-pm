import { DatePicker } from '@/shared/ui'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useRef } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { DataTable, Input, TableActionButton, UserSelect } from '@/shared/ui'
import type {
  UserOptionsState,
  WorkItemChildDraft,
  WorkItemChildDraftError,
  WorkItemStatusOption,
} from '../../model/types'
import { WORK_ITEM_PRIORITY_OPTIONS, workItemUserSelectOption } from '../../model/utils'
import { WorkItemsSelect } from '../collection/SelectField'

type DraftField = keyof Omit<WorkItemChildDraft, 'clientId'>

export function ChildDraftTable({
  rows,
  errors,
  disabled,
  users,
  statuses,
  onUpdate,
  onRemove,
}: {
  rows: WorkItemChildDraft[]
  errors: Readonly<Record<string, WorkItemChildDraftError>>
  disabled: boolean
  users: UserOptionsState
  statuses: readonly WorkItemStatusOption[]
  onUpdate: (clientId: string, key: DraftField, value: string | number | undefined) => void
  onRemove: (clientId: string) => void
}) {
  const errorsRef = useRef(errors)
  errorsRef.current = errors
  const readyUsers = users.status === 'ready' ? users.data : []
  const userOptions = useMemo(
    () => readyUsers.map(workItemUserSelectOption),
    [readyUsers],
  )
  const statusOptions = useMemo(
    () => statuses.map(({ value, label }) => ({ value, label })),
    [statuses],
  )
  const columns = useMemo<ColumnDef<WorkItemChildDraft>[]>(() => [
    {
      accessorKey: 'title', header: WORK_ITEMS_COPY.detail.childName,
      cell: ({ row }) => (
        <Input
          value={row.original.title}
          maxLength={200}
          className="min-w-56"
          aria-label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childName, row.index + 1)}
          aria-invalid={errorsRef.current[row.original.clientId] === 'title' || undefined}
          disabled={disabled}
          onChange={(event) => onUpdate(row.original.clientId, 'title', event.currentTarget.value)}
        />
      ),
    },
    {
      accessorKey: 'reporterUserId', header: WORK_ITEMS_COPY.detail.childReporter,
      cell: ({ row }) => (
        <UserSelect
          label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childReporter, row.index + 1)}
          value={row.original.reporterUserId}
          options={userOptions}
          emptyLabel={WORK_ITEMS_COPY.detail.childOptional}
          disabled={disabled || users.status !== 'ready'}
          className="min-w-48"
          onValueChange={(value) => onUpdate(row.original.clientId, 'reporterUserId', value)}
        />
      ),
    },
    {
      accessorKey: 'assigneeId', header: WORK_ITEMS_COPY.detail.childAssignee,
      cell: ({ row }) => (
        <UserSelect
          label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childAssignee, row.index + 1)}
          value={row.original.assigneeId}
          options={userOptions}
          emptyLabel={WORK_ITEMS_COPY.detail.childOptional}
          disabled={disabled || users.status !== 'ready'}
          className="min-w-48"
          onValueChange={(value) => onUpdate(row.original.clientId, 'assigneeId', value)}
        />
      ),
    },
    {
      accessorKey: 'status', header: WORK_ITEMS_COPY.detail.childStatus,
      cell: ({ row }) => (
        <div className="min-w-44">
          <WorkItemsSelect
            id={`${row.original.clientId}-status`}
            label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childStatus, row.index + 1)}
            value={row.original.status}
            placeholder={WORK_ITEMS_COPY.detail.childOptional}
            disabled={disabled || !statusOptions.length}
            options={statusOptions}
            onValueChange={(value) => onUpdate(row.original.clientId, 'status', value)}
          />
        </div>
      ),
    },
    {
      accessorKey: 'priority', header: WORK_ITEMS_COPY.detail.childPriority,
      cell: ({ row }) => (
        <div className="min-w-32">
          <WorkItemsSelect
            id={`${row.original.clientId}-priority`}
            label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childPriority, row.index + 1)}
            value={row.original.priority}
            placeholder={WORK_ITEMS_COPY.detail.childOptional}
            disabled={disabled}
            options={WORK_ITEM_PRIORITY_OPTIONS}
            onValueChange={(value) => onUpdate(row.original.clientId, 'priority', value)}
          />
        </div>
      ),
    },
    {
      accessorKey: 'startsAt', header: WORK_ITEMS_COPY.detail.childStartsAt,
      cell: ({ row }) => (
        <DatePicker

          value={row.original.startsAt}
          className="min-w-36"
          aria-label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childStartsAt, row.index + 1)}
          disabled={disabled}
          onChange={(value) => onUpdate(row.original.clientId, 'startsAt', value)}
        />
      ),
    },
    {
      accessorKey: 'dueAt', header: WORK_ITEMS_COPY.detail.childDueAt,
      cell: ({ row }) => (
        <DatePicker

          value={row.original.dueAt}
          className="min-w-36"
          aria-label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childDueAt, row.index + 1)}
          disabled={disabled}
          onChange={(value) => onUpdate(row.original.clientId, 'dueAt', value)}
        />
      ),
    },
    {
      accessorKey: 'estimatedHours', header: WORK_ITEMS_COPY.detail.childEstimatedHours,
      cell: ({ row }) => (
        <Input
          type="number"
          min="0"
          max="16666"
          step="0.5"
          value={row.original.estimatedHours}
          className="min-w-32"
          aria-label={WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childEstimatedHours, row.index + 1)}
          disabled={disabled}
          onChange={(event) => onUpdate(row.original.clientId, 'estimatedHours', event.currentTarget.value)}
        />
      ),
    },
    {
      id: 'actions', size: 72, header: WORK_ITEMS_COPY.detail.childActions,
      cell: ({ row }) => (
        <div className="flex min-w-max flex-nowrap justify-center">
          <TableActionButton
            text={WORK_ITEMS_COPY.detail.childRemove}
            label={WORK_ITEMS_COPY.detail.childRemoveRow(row.index + 1)}
            disabled={disabled}
            onClick={() => onRemove(row.original.clientId)}
          />
        </div>
      ),
    },
  ], [disabled, onRemove, onUpdate, statusOptions, userOptions, users.status])
  return (
    <DataTable
      aria-label={WORK_ITEMS_COPY.detail.createChildren}
      columns={columns}
      state={{ status: 'ready', rows }}
      tableClassName="min-w-[1540px]"
      getRowId={(row) => row.clientId}
      // eslint-disable-next-line shadcn/require-static-classes -- 按行回调返回静态类名；规则无法窥入函数体
      getRowClassName={() => 'h-12 [&_[data-slot=table-cell]]:py-2'}
    />
  )
}
