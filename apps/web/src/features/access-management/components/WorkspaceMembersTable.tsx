import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  DataTable,
  StatusPill,
  TableActionButton,
  UserIdentity,
  type DataTableState,
  type StatusPillTone,
} from '@/shared/ui'
import type { AccountStatus, ManagedWorkspaceMember } from '../model'

const accountTone: Readonly<Record<AccountStatus, StatusPillTone>> = {
  pending_approval: 'warning',
  active: 'success',
  rejected: 'danger',
  banned: 'neutral',
}

type WorkspaceMembersTableProps = {
  state: DataTableState<ManagedWorkspaceMember>
  canEdit: boolean
  canRemove: boolean
  onEdit: (member: ManagedWorkspaceMember) => void
  onRemove: (member: ManagedWorkspaceMember) => void
}

export function WorkspaceMembersTable({ state, canEdit, canRemove, onEdit, onRemove }: WorkspaceMembersTableProps) {
  const columns = useMemo<ColumnDef<ManagedWorkspaceMember>[]>(() => [
    {
      id: 'user',
      header: COPY.workspaces.memberColumns.user,
      cell: ({ row }) => (
        <UserIdentity
          name={row.original.displayName || row.original.username || COPY.common.unnamed}
          secondary={row.original.email || row.original.username}
          avatarUrl={row.original.avatarUrl}
        />
      ),
    },
    {
      id: 'role',
      header: COPY.workspaces.memberColumns.role,
      cell: ({ row }) => COPY.workspaces.roles[row.original.role],
    },
    {
      id: 'memberStatus',
      header: COPY.workspaces.memberColumns.status,
      cell: ({ row }) => (
        <StatusPill tone={row.original.enabled ? 'success' : 'neutral'}>
          {row.original.enabled ? COPY.workspaces.memberEnabled : COPY.workspaces.memberDisabled}
        </StatusPill>
      ),
    },
    {
      id: 'account',
      header: COPY.workspaces.memberColumns.account,
      cell: ({ row }) => (
        <StatusPill tone={accountTone[row.original.status]}>{COPY.people.statuses[row.original.status]}</StatusPill>
      ),
    },
    {
      id: 'actions',
      header: COPY.workspaces.memberColumns.actions,
      cell: ({ row }) => (
        <div className="flex flex-nowrap justify-end gap-0.5">
          {canEdit ? <TableActionButton text={COPY.workspaces.editMember} label={COPY.workspaces.editMember} onClick={() => onEdit(row.original)} /> : null}
          {canRemove ? <TableActionButton text={COPY.workspaces.removeMember} label={COPY.workspaces.removeMember} onClick={() => onRemove(row.original)} /> : null}
        </div>
      ),
    },
  ], [canEdit, canRemove, onEdit, onRemove])

  return (
    <DataTable
      aria-label={COPY.workspaces.membersTableLabel}
      className="h-150 [&_[data-slot=table-scroll-shell]]:h-full"
      tableClassName="[&_tbody_tr]:h-14"
      scrollAreaClassName="h-full"
      columns={columns}
      state={state}
      getRowId={(row) => String(row.userId)}
    />
  )
}
