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
import type { AccessCapabilities, AccountStatus, ManagedUser, UserStatusIntent } from '../model'

const statusTone: Readonly<Record<AccountStatus, StatusPillTone>> = {
  pending_approval: 'warning',
  active: 'success',
  rejected: 'danger',
  banned: 'neutral',
}

type PeopleTableProps = {
  state: DataTableState<ManagedUser>
  capabilities: AccessCapabilities
  onStatusIntent: (user: ManagedUser, intent: UserStatusIntent) => void
  onEditAccess: (user: ManagedUser) => void
  onResetPassword: (user: ManagedUser) => void
}

export function PeopleTable({
  state,
  capabilities,
  onStatusIntent,
  onEditAccess,
  onResetPassword,
}: PeopleTableProps) {
  const columns = useMemo<ColumnDef<ManagedUser>[]>(() => [
    {
      id: 'user',
      header: COPY.people.columns.user,
      cell: ({ row }) => (
        <UserIdentity
          name={row.original.name || row.original.username || COPY.common.unnamed}
          nameSuffix={row.original.username ? COPY.people.loginSuffix(row.original.username) : undefined}
          secondary={row.original.email}
          avatarUrl={row.original.avatarUrl}
        />
      ),
    },
    {
      id: 'status',
      header: COPY.people.columns.status,
      cell: ({ row }) => (
        <StatusPill tone={statusTone[row.original.status]}>
          {COPY.people.statuses[row.original.status]}
        </StatusPill>
      ),
    },
    {
      id: 'group',
      header: COPY.people.columns.group,
      cell: ({ row }) => row.original.group?.name || COPY.people.noGroup,
    },
    {
      id: 'roles',
      header: COPY.people.columns.roles,
      cell: ({ row }) => row.original.roles.map((role) => role.name).join('、') || COPY.people.noRoles,
    },
    {
      id: 'actions',
      header: COPY.people.columns.actions,
      cell: ({ row }) => {
        const user = row.original
        return (
          <div className="flex flex-nowrap items-center justify-end gap-0.5">
            {user.status === 'pending_approval' && capabilities.approve ? (
              <TableActionButton text={COPY.people.approve} label={COPY.people.approve} onClick={() => onStatusIntent(user, 'approve')} />
            ) : null}
            {user.status === 'pending_approval' && capabilities.reject ? (
              <TableActionButton text={COPY.people.reject} label={COPY.people.reject} onClick={() => onStatusIntent(user, 'reject')} />
            ) : null}
            {user.status === 'active' && capabilities.ban ? (
              <TableActionButton text={COPY.people.ban} label={COPY.people.ban} onClick={() => onStatusIntent(user, 'ban')} />
            ) : null}
            {user.status === 'banned' && capabilities.unban ? (
              <TableActionButton text={COPY.people.unban} label={COPY.people.unban} onClick={() => onStatusIntent(user, 'unban')} />
            ) : null}
            {capabilities.manageAssignments ? (
              <TableActionButton text={COPY.people.editAccess} label={COPY.people.editAccess} onClick={() => onEditAccess(user)} />
            ) : null}
            {capabilities.resetPassword ? (
              <TableActionButton
                text={COPY.people.resetPassword}
                label={COPY.people.resetPassword}
                onClick={() => onResetPassword(user)}
              />
            ) : null}
          </div>
        )
      },
    },
  ], [capabilities, onEditAccess, onResetPassword, onStatusIntent])

  return (
    <DataTable
      aria-label={COPY.people.sectionLabel}
      columns={columns}
      state={state}
      getRowId={(row) => String(row.id)}
    />
  )
}
