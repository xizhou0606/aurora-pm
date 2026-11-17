import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  DataTable,
  StatusPill,
  TableActionButton,
  type DataTableState,
} from '@/shared/ui'
import type { ManagedWorkspace } from '../model'

type WorkspaceTableProps = {
  state: DataTableState<ManagedWorkspace>
  canEdit: boolean
  canDelete: boolean
  canSetDefault: boolean
  canViewMembers: boolean
  selectedId: number | null
  onSelect: (workspace: ManagedWorkspace) => void
  onEdit: (workspace: ManagedWorkspace) => void
  onDelete: (workspace: ManagedWorkspace) => void
  onSetDefault: (workspace: ManagedWorkspace) => void
}

export function WorkspaceTable({
  state,
  canEdit,
  canDelete,
  canSetDefault,
  canViewMembers,
  selectedId,
  onSelect,
  onEdit,
  onDelete,
  onSetDefault,
}: WorkspaceTableProps) {
  const columns = useMemo<ColumnDef<ManagedWorkspace>[]>(() => [
    {
      id: 'workspace',
      header: COPY.workspaces.columns.workspace,
      cell: ({ row }) => (
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate font-medium">{row.original.name}</span>
            {row.original.isDefault ? <StatusPill tone="info">{COPY.workspaces.defaultTag}</StatusPill> : null}
          </div>
          {row.original.description ? (
            <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">{row.original.description}</p>
          ) : null}
        </div>
      ),
    },
    { id: 'slug', header: COPY.workspaces.columns.slug, accessorKey: 'slug' },
    {
      id: 'status',
      header: COPY.workspaces.columns.status,
      cell: ({ row }) => (
        <StatusPill tone={row.original.enabled ? 'success' : 'neutral'}>
          {row.original.enabled ? COPY.workspaces.enabled : COPY.workspaces.disabled}
        </StatusPill>
      ),
    },
    {
      id: 'members',
      header: COPY.workspaces.columns.members,
      cell: ({ row }) => COPY.workspaces.membersCount(row.original.memberCount),
    },
    {
      id: 'actions',
      header: COPY.workspaces.columns.actions,
      cell: ({ row }) => (
        <div className="flex flex-nowrap justify-end gap-0.5">
          {canViewMembers ? (
            <TableActionButton
              text={COPY.workspaces.manageMembers}
              label={COPY.workspaces.manageMembers}
              onClick={() => onSelect(row.original)}
            />
          ) : null}
          {canEdit ? (
            <TableActionButton text={COPY.workspaces.edit} label={COPY.workspaces.edit} onClick={() => onEdit(row.original)} />
          ) : null}
          {canSetDefault && !row.original.isDefault ? (
            <TableActionButton
              text={COPY.workspaces.setDefault}
              label={COPY.workspaces.setDefault}
              onClick={() => onSetDefault(row.original)}
            />
          ) : null}
          {canDelete && !row.original.isDefault ? (
            <TableActionButton text={COPY.workspaces.delete} label={COPY.workspaces.delete} onClick={() => onDelete(row.original)} />
          ) : null}
        </div>
      ),
    },
  ], [canDelete, canEdit, canSetDefault, canViewMembers, onDelete, onEdit, onSelect, onSetDefault])

  return (
    <DataTable
      aria-label={COPY.workspaces.sectionLabel}
      columns={columns}
      state={state}
      getRowId={(row) => String(row.id)}
      // 返回值是静态类名三元；规则无法穿透函数形式的 className，行内豁免
      // eslint-disable-next-line shadcn/require-static-classes
      getRowClassName={(row) => row.id === selectedId ? 'bg-muted/60' : ''}
      rowAction={canViewMembers ? { label: (row) => `${COPY.workspaces.manageMembers}：${row.name}`, onActivate: onSelect } : undefined}
    />
  )
}
