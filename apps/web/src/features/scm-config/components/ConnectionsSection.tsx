import { useMemo, useState } from 'react'

import type { ColumnDef } from '@tanstack/react-table'

import { PhLink, PhPlus } from '@/shared/ui/icons'
import {
  AlertBanner,
  Button,
  Checkbox,
  DataTable,
  DetailSection,
  Input,
  Label,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
  TableActionButton,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmConnectionDTO, ScmConnectionInput } from '../api/contracts'
import {
  useConnectionsQuery,
  useCreateConnectionMutation,
  useDeleteConnectionMutation,
  useScmConfigAccess,
  useScmConfigError,
  useTestConnectionMutation,
  useUpdateConnectionMutation,
} from '../hooks'

type ConnectionFormState = {
  name: string
  provider: 'github' | 'gitlab'
  base_url: string
  token: string
  enabled: boolean
}

const EMPTY_FORM: ConnectionFormState = {
  name: '',
  provider: 'gitlab',
  base_url: '',
  token: '',
  enabled: true,
}

const PROVIDER_ITEMS = [
  { value: 'gitlab', label: COPY.connections.providers.gitlab },
  { value: 'github', label: COPY.connections.providers.github },
] as const

export function ConnectionsSection() {
  const access = useScmConfigAccess()
  const canView = access.canView('connections')
  const canManage = access.canManage('connections')
  const connectionsQuery = useConnectionsQuery(canView)
  const { error, reportError, clearError } = useScmConfigError()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<ScmConnectionDTO | null>(null)
  const [form, setForm] = useState<ConnectionFormState>(EMPTY_FORM)
  const [testedLogin, setTestedLogin] = useState('')

  const createMutation = useCreateConnectionMutation(reportError)
  const updateMutation = useUpdateConnectionMutation(reportError)
  const deleteMutation = useDeleteConnectionMutation(reportError)
  const testMutation = useTestConnectionMutation(reportError)

  const openCreate = () => {
    setEditing(null)
    setForm(EMPTY_FORM)
    setTestedLogin('')
    setEditorOpen(true)
  }
  const openEdit = (connection: ScmConnectionDTO) => {
    setEditing(connection)
    setForm({
      name: connection.name,
      provider: connection.provider,
      base_url: connection.base_url ?? '',
      token: '',
      enabled: connection.enabled,
    })
    setTestedLogin('')
    setEditorOpen(true)
  }
  const submit = () => {
    const body: ScmConnectionInput = {
      name: form.name,
      provider: form.provider,
      base_url: form.base_url,
      token: form.token,
      enabled: form.enabled,
    }
    if (editing) {
      updateMutation.mutate({ id: editing.id, body }, { onSuccess: () => setEditorOpen(false) })
    } else {
      createMutation.mutate(body, { onSuccess: () => setEditorOpen(false) })
    }
  }

  const columns = useMemo<ColumnDef<ScmConnectionDTO>[]>(() => [
    {
      id: 'name',
      header: COPY.connections.fields.name,
      cell: ({ row }) => (
        <div className="min-w-0">
          <span className="truncate font-medium">{row.original.name}</span>
        </div>
      ),
    },
    {
      id: 'provider',
      header: COPY.connections.fields.provider,
      cell: ({ row }) => (
        <span className="inline-flex rounded-md bg-muted px-1.5 py-0.5 font-mono text-xs uppercase text-muted-foreground">
          {row.original.provider}
        </span>
      ),
    },
    {
      id: 'baseUrl',
      header: COPY.connections.fields.baseUrl,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">{row.original.base_url || '—'}</span>
      ),
    },
    {
      id: 'token',
      header: COPY.connections.fields.token,
      cell: ({ row }) => <span className="font-mono text-xs">{row.original.token_masked}</span>,
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
  ], [])

  const rows = connectionsQuery.data ?? []

  return (
    <DetailSection
      className="overflow-hidden"
      title={(
        <span className="flex items-center gap-2">
          <PhLink size={16} tone="neutral" aria-hidden="true" />
          {COPY.connections.title}
        </span>
      )}
      description={COPY.connections.description}
      actions={canManage ? <Button onClick={openCreate}><PhPlus />{COPY.connections.create}</Button> : undefined}
    >
      {error ? (
        <AlertBanner tone="error" onDismiss={clearError}>
          {COPY.errors.requestFailed(error)}
        </AlertBanner>
      ) : null}
      {testedLogin ? (
        <AlertBanner tone="success" onDismiss={() => setTestedLogin('')}>
          {COPY.connections.testSuccess(testedLogin)}
        </AlertBanner>
      ) : null}
      {/* 表格通栏贴卡：外扩抵消 DetailSection 内边距，表头分隔线由 border-t 承担 */}
      <div className="-mx-4 -mb-4">
        <DataTable
          className="rounded-none border-0 border-t"
          aria-label={COPY.connections.title}
          columns={canManage ? [...columns, {
            id: 'actions',
            header: COPY.common.edit,
            cell: ({ row }) => (
              <div className="flex flex-nowrap justify-end gap-0.5">
                <TableActionButton
                  text={COPY.connections.test}
                  label={COPY.connections.test}
                  onClick={() =>
                    testMutation.mutate(row.original.id, {
                      onSuccess: (account) => setTestedLogin(account.display_name || account.login),
                    })
                  }
                />
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
              : connectionsQuery.isPending
                ? { status: 'loading' }
                : connectionsQuery.isError
                  ? { status: 'error', title: COPY.errors.loadFailed, retryLabel: COPY.common.retry, onRetry: () => void connectionsQuery.refetch() }
                  : rows.length === 0
                    ? { status: 'empty', title: COPY.common.empty }
                    : { status: 'ready', rows }
          }
          getRowId={(row) => String(row.id)}
        />
      </div>
      <ResponsivePanel
        open={editorOpen}
        onOpenChange={setEditorOpen}
        title={editing ? COPY.connections.edit : COPY.connections.create}
        closeLabel={COPY.common.close}
        footer={
          <>
            <Button variant="outline" onClick={() => setEditorOpen(false)}>
              {COPY.common.cancel}
            </Button>
            <Button
              onClick={submit}
              disabled={createMutation.isPending || updateMutation.isPending || form.name === ''}
            >
              {COPY.common.save}
            </Button>
          </>
        }
      >
        <div className="grid gap-4">
          <label className="grid gap-1.5">
            <Label htmlFor="scm-connection-name">{COPY.connections.fields.name}</Label>
            <Input
              id="scm-connection-name"
              value={form.name}
              onChange={(event) => setForm((prev) => ({ ...prev, name: event.target.value }))}
            />
          </label>
          <label className="grid gap-1.5">
            <Label htmlFor="scm-connection-provider">{COPY.connections.fields.provider}</Label>
            <Select
              items={[...PROVIDER_ITEMS]}
              value={form.provider}
              onValueChange={(value) =>
                setForm((prev) => ({ ...prev, provider: value as 'github' | 'gitlab' }))}
            >
              <SelectTrigger id="scm-connection-provider" name="scm-connection-provider" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROVIDER_ITEMS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          <label className="grid gap-1.5">
            <Label htmlFor="scm-connection-base-url">{COPY.connections.fields.baseUrl}</Label>
            <Input
              id="scm-connection-base-url"
              value={form.base_url}
              onChange={(event) => setForm((prev) => ({ ...prev, base_url: event.target.value }))}
            />
          </label>
          <label className="grid gap-1.5">
            <Label htmlFor="scm-connection-token">{COPY.connections.fields.token}</Label>
            <Input
              id="scm-connection-token"
              type="password"
              value={form.token}
              onChange={(event) => setForm((prev) => ({ ...prev, token: event.target.value }))}
            />
            {editing ? <p className="text-xs text-muted-foreground">{COPY.connections.fields.tokenKeep}</p> : null}
          </label>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={form.enabled}
              onCheckedChange={(checked) => setForm((prev) => ({ ...prev, enabled: checked === true }))}
            />
            {COPY.common.enabled}
          </label>
        </div>
      </ResponsivePanel>
    </DetailSection>
  )
}