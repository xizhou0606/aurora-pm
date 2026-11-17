import { useMemo, useState } from 'react'

import type { ColumnDef } from '@tanstack/react-table'

import { PhGitBranch, PhPlus } from '@/shared/ui/icons'
import {
  AlertBanner,
  Button,
  DataTable,
  DetailSection,
  StatusPill,
  TableActionButton,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmRepositoryDTO } from '../api/contracts'
import {
  useConnectionsQuery,
  useDeleteRepositoryMutation,
  useRepositoriesQuery,
  useScmConfigAccess,
  useScmConfigError,
} from '../hooks'

import { RepositoryEditor } from './RepositoryEditor'
import { RepositoryScanPicker } from './RepositoryScanPicker'

export function RepositoriesSection() {
  const access = useScmConfigAccess()
  const canView = access.canView('repositories')
  const canManage = access.canManage('repositories')
  const repositoriesQuery = useRepositoriesQuery(canView)
  const connectionsQuery = useConnectionsQuery(canView)
  const { error, reportError, clearError } = useScmConfigError()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<ScmRepositoryDTO | null>(null)
  const [scanOpen, setScanOpen] = useState(false)

  const connections = connectionsQuery.data ?? []
  const deleteMutation = useDeleteRepositoryMutation(reportError)

  const openCreate = () => {
    setEditing(null)
    setEditorOpen(true)
  }
  const openEdit = (repository: ScmRepositoryDTO) => {
    setEditing(repository)
    setEditorOpen(true)
  }

  const columns = useMemo<ColumnDef<ScmRepositoryDTO>[]>(() => [
    {
      id: 'path',
      header: COPY.repositories.fields.path,
      cell: ({ row }) => <span className="font-mono text-xs">{row.original.path}</span>,
    },
    {
      id: 'connection',
      header: COPY.repositories.fields.connection,
      cell: ({ row }) =>
        connections.find((connection) => connection.id === row.original.connection_id)?.name ??
        row.original.connection_id,
    },
    {
      id: 'branch',
      header: COPY.repositories.fields.branch,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">{row.original.default_branch || '—'}</span>
      ),
    },
    {
      id: 'status',
      header: COPY.common.enabled,
      cell: ({ row }) => (
        <StatusPill tone={row.original.enabled ? 'success' : 'neutral'}>
          {row.original.enabled ? COPY.common.enabled : COPY.common.disabled}
        </StatusPill>
      ),
    },
  ], [connections])

  const rows = repositoriesQuery.data ?? []

  return (
    <DetailSection
      className="overflow-hidden"
      title={(
        <span className="flex items-center gap-2">
          <PhGitBranch size={16} tone="neutral" aria-hidden="true" />
          {COPY.repositories.title}
        </span>
      )}
      description={COPY.repositories.description}
      actions={canManage ? (
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setScanOpen(true)}>
            {COPY.repositories.scan}
          </Button>
          <Button onClick={openCreate}><PhPlus />{COPY.repositories.create}</Button>
        </div>
      ) : undefined}
    >
      {error ? (
        <AlertBanner tone="error" onDismiss={clearError}>
          {COPY.errors.requestFailed(error)}
        </AlertBanner>
      ) : null}
      {/* 表格通栏贴卡：外扩抵消 DetailSection 内边距，表头分隔线由 border-t 承担 */}
      <div className="-mx-4 -mb-4">
        <DataTable
          className="rounded-none border-0 border-t"
          aria-label={COPY.repositories.title}
          columns={canManage ? [...columns, {
            id: 'actions',
            header: COPY.common.edit,
            cell: ({ row }) => (
              <div className="flex flex-nowrap justify-end gap-0.5">
                <TableActionButton text={COPY.common.edit} label={COPY.common.edit} onClick={() => openEdit(row.original)} />
                <TableActionButton
                  text={COPY.common.delete}
                  label={COPY.common.delete}
                  onClick={() => {
                    clearError()
                    deleteMutation.mutate(row.original.id)
                  }}
                />
              </div>
            ),
          }] : columns}
          state={
            !canView
              ? { status: 'empty', title: COPY.common.empty }
              : repositoriesQuery.isPending
                ? { status: 'loading' }
                : repositoriesQuery.isError
                  ? { status: 'error', title: COPY.errors.loadFailed, retryLabel: COPY.common.retry, onRetry: () => void repositoriesQuery.refetch() }
                  : rows.length === 0
                    ? { status: 'empty', title: COPY.common.empty }
                    : { status: 'ready', rows }
          }
          getRowId={(row) => String(row.id)}
        />
      </div>
      {canManage ? (
        <>
          <RepositoryEditor
            key={`editor-${editorOpen}-${editing?.id ?? 'new'}`}
            open={editorOpen}
            editing={editing}
            connections={connections}
            onOpenChange={setEditorOpen}
            onError={reportError}
          />
          <RepositoryScanPicker
            key={`scan-${scanOpen}`}
            open={scanOpen}
            connections={connections}
            repositories={rows}
            onOpenChange={setScanOpen}
            onError={reportError}
          />
        </>
      ) : null}
    </DetailSection>
  )
}
