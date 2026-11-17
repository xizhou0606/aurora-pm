import type { RefObject } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { copy } from '@/shared/copy'
import { Button, ConfirmDialog, DataTable, Pagination, ResponsivePanel, StatusPill } from '@/shared/ui'
import type { AuthorizationRecord } from '../model/authorization-records'
import { formatToolingDateTime } from '../model/model'
import type { AuthorizationRecordsControl } from '../model/authorization-records'

function RecordList({ model }: { model: AuthorizationRecordsControl }) {
  const labels = copy.mcpCli.grants
  const columns: ColumnDef<AuthorizationRecord>[] = [
    { accessorKey: 'user_code', header: labels.code, cell: ({ row }) => <div><code className="whitespace-nowrap text-xs">{row.original.user_code ?? labels.noCode}</code><p className="m-0 text-xs text-muted-foreground">{labels.clientChannel(row.original.client_name, copy.mcpCli.channel[row.original.channel])}</p></div> },
    { id: 'user', header: labels.user, size: 140, cell: ({ row }) => row.original.principal_id && row.original.status !== 'pending' && row.original.status !== 'denied' ? <div className="w-32 min-w-0 whitespace-normal break-words"><span>{row.original.principal_name ?? row.original.principal_id}</span>{row.original.principal_username ? <p className="m-0 break-all text-xs text-muted-foreground">{row.original.principal_username}</p> : null}</div> : labels.noUser },
    { id: 'tools', header: labels.tools, cell: ({ row }) => <details className="max-w-64"><summary className="cursor-pointer whitespace-nowrap text-xs">{labels.scopes(row.original.grant_tools.length)}</summary><ul className="my-1 list-none space-y-1 p-0">{row.original.grant_tools.map((name) => <li key={name}><code className="break-all text-xs">{name}</code></li>)}</ul></details> },
    { id: 'time', header: labels.time, cell: ({ row }) => <span className="whitespace-nowrap text-xs">{formatToolingDateTime(row.original.created_at)}</span> },
    { id: 'status', header: labels.status, cell: ({ row }) => <StatusPill tone={row.original.status === 'approved' ? 'success' : row.original.status === 'pending' ? 'info' : 'neutral'}>{labels.recordStatus[row.original.status]}</StatusPill> },
    { id: 'valid', header: labels.credentialValid, cell: ({ row }) => <span className="whitespace-nowrap text-xs">{row.original.credential_valid ? labels.valid : labels.invalid}</span> },
    { id: 'actions', header: labels.actions, cell: ({ row }) => row.original.can_revoke ? <Button variant="link" size="xs" onClick={() => model.select(row.original)}>{labels.revoke}</Button> : null },
  ]
  return <>
    <DataTable columns={columns} state={model.state} getRowId={(row) => row.id} aria-label={labels.title} />
    <Pagination className="mt-3" page={model.page} pageSize={model.pageSize} total={model.total} onPageChange={model.setPage} onPageSizeChange={model.setPageSize} />
    <ConfirmDialog open={Boolean(model.selected)} onOpenChange={(value) => { if (!value) model.close() }} onConfirm={model.confirm}
      title={labels.revokeTitle} description={labels.revokeDescription} cancelLabel={labels.cancel}
      confirmLabel={model.pending ? labels.revoking : labels.revoke} pending={model.pending} error={model.error} />
  </>
}

export function GrantPanel({ open, model, finalFocus, onOpenChange }: {
  open: boolean
  model: AuthorizationRecordsControl
  finalFocus: RefObject<HTMLButtonElement | null>
  onOpenChange: (open: boolean) => void
}) {
  return <ResponsivePanel open={open} onOpenChange={onOpenChange} title={copy.mcpCli.grants.title}
    description={copy.mcpCli.grants.description} closeLabel={copy.mcpCli.grants.close} finalFocus={finalFocus} className="max-w-6xl">
    <RecordList model={model} />
  </ResponsivePanel>
}
