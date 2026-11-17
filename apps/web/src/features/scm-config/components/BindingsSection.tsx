import { useMemo, useState } from 'react'

import type { ColumnDef } from '@tanstack/react-table'

import { PhUsers } from '@/shared/ui/icons'
import {
  AlertBanner,
  Button,
  DataTable,
  DetailSection,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
  UserSelect,
  ViewTabs,
  type ViewTab,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmBindingDTO } from '../api/contracts'
import {
  useBindingsQuery,
  useConfirmBindingMutation,
  useMemberOptionsQuery,
  useRejectBindingMutation,
  useScmConfigAccess,
  useScmConfigError,
} from '../hooks'

import { MemberBindingRoster } from './MemberBindingRoster'

const STATUS_FILTERS = ['unmatched', 'suggested', 'matched', 'bot', ''] as const

const VIEW_TABS: readonly ViewTab[] = [
  { id: 'member', label: COPY.bindings.viewByMember },
  { id: 'author', label: COPY.bindings.viewByAuthor },
]

type BindingView = 'member' | 'author'

export function BindingsSection() {
  const access = useScmConfigAccess()
  const canView = access.canView('bindings')
  const canManage = access.canManage('bindings')
  const [view, setView] = useState<BindingView>('member')
  const [statusFilter, setStatusFilter] = useState<string>('unmatched')
  const bindingsQuery = useBindingsQuery(statusFilter || undefined, canView && view === 'author')
  const { error, reportError, clearError } = useScmConfigError()
  const [confirming, setConfirming] = useState<ScmBindingDTO | null>(null)
  const [userId, setUserId] = useState<number | undefined>(undefined)

  // 成员目录驱动选人：平台全员（姓名+头像），建议成员自动预选；目录加载
  // 失败时退回手填成员 ID 的备用输入，确认接口仍会校验成员身份。
  const memberOptions = useMemberOptionsQuery(canView)
  const memberNames = useMemo(
    () => new Map(memberOptions.options.map((option) => [option.id, option.name])),
    [memberOptions.options],
  )

  const confirmMutation = useConfirmBindingMutation(reportError)
  const rejectMutation = useRejectBindingMutation(reportError)

  const openConfirm = (binding: ScmBindingDTO) => {
    setConfirming(binding)
    setUserId(binding.suggested_user_id ?? undefined)
  }

  const columns = useMemo<ColumnDef<ScmBindingDTO>[]>(() => [
    {
      id: 'author',
      header: COPY.bindings.fields.author,
      cell: ({ row }) => (
        <div className="min-w-0">
          <span className="truncate font-medium">{row.original.author_key}</span>
          {row.original.display_name ? (
            <p className="text-xs text-muted-foreground">{COPY.bindings.gitNamePrefix}{row.original.display_name}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'kind',
      header: COPY.bindings.fields.kind,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-muted-foreground">{row.original.key_kind}</span>
      ),
    },
    {
      id: 'provider',
      header: COPY.bindings.fields.provider,
      cell: ({ row }) => (
        <span className="inline-flex rounded-md bg-muted px-1.5 py-0.5 font-mono text-xs uppercase text-muted-foreground">
          {row.original.provider}
        </span>
      ),
    },
    {
      id: 'status',
      header: COPY.bindings.fields.status,
      cell: ({ row }) => (
        <StatusPill
          tone={
            row.original.status === 'matched'
              ? 'success'
              : row.original.status === 'bot'
                ? 'neutral'
                : row.original.status === 'suggested'
                  ? 'info'
                  : 'warning'
          }
        >
          {COPY.bindings.statuses[row.original.status] ?? row.original.status}
        </StatusPill>
      ),
    },
    {
      id: 'user',
      header: COPY.bindings.fields.user,
      cell: ({ row }) => {
        const id = row.original.user_id
        if (!id) return <span className="text-muted-foreground">{COPY.common.placeholderEmpty}</span>
        return memberNames.get(id) ?? `ID ${id}`
      },
    },
  ], [memberNames])

  const rows = bindingsQuery.data ?? []
  const suggestedName = confirming?.suggested_user_id
    ? memberNames.get(confirming.suggested_user_id) ?? `ID ${confirming.suggested_user_id}`
    : undefined

  const statusItems = useMemo(() => STATUS_FILTERS.map((status) => ({
    value: status || 'all',
    label: status ? (COPY.bindings.statuses[status] ?? status) : COPY.bindings.statusAll,
  })), [])

  return (
    <DetailSection
      className="overflow-hidden"
      title={(
        <span className="flex items-center gap-2">
          <PhUsers size={16} tone="neutral" aria-hidden="true" />
          {COPY.bindings.title}
        </span>
      )}
      description={COPY.bindings.description}
      actions={(
        <div className="flex items-center gap-2">
          <ViewTabs
            label={COPY.bindings.title}
            tabs={VIEW_TABS}
            value={view}
            onValueChange={(value) => setView(value as BindingView)}
            density="compact"
          />
          {view === 'author' ? (
            <Select
              items={statusItems}
              value={statusFilter || 'all'}
              onValueChange={(value) => setStatusFilter(value === 'all' || value == null ? '' : String(value))}
            >
              <SelectTrigger aria-label={COPY.bindings.fields.status} className="w-28">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {statusItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : null}
        </div>
      )}
    >
      {error ? (
        <AlertBanner tone="error" onDismiss={clearError}>
          {COPY.errors.requestFailed(error)}
        </AlertBanner>
      ) : null}
      {view === 'member' ? (
        <MemberBindingRoster
          members={memberOptions.options}
          membersPending={memberOptions.pending}
          canView={canView}
          canManage={canManage}
          onError={reportError}
        />
      ) : (
        <>
          {/* 表格通栏贴卡：外扩抵消 DetailSection 内边距，表头分隔线由 border-t 承担 */}
          <div className="-mx-4 -mb-4">
            <DataTable
              className="rounded-none border-0 border-t"
              aria-label={COPY.bindings.title}
              columns={columns}
              state={
                !canView
                  ? { status: 'empty', title: COPY.common.empty }
                  : bindingsQuery.isPending
                    ? { status: 'loading' }
                    : bindingsQuery.isError
                      ? { status: 'error', title: COPY.errors.loadFailed, retryLabel: COPY.common.retry, onRetry: () => void bindingsQuery.refetch() }
                      : rows.length === 0
                        ? { status: 'empty', title: COPY.common.empty }
                        : { status: 'ready', rows }
              }
              getRowId={(row) => String(row.id)}
              rowAction={
                canManage
                  ? {
                      label: () => COPY.bindings.confirm,
                      onActivate: (row) => {
                        if (row.status === 'matched' || row.status === 'bot') return
                        openConfirm(row)
                      },
                    }
                  : undefined
              }
            />
            {canManage && confirming !== null ? (
              <div className="flex flex-wrap items-center gap-3 border-t bg-muted/40 p-4">
                <div className="min-w-40 flex-1">
                  <p className="truncate text-sm font-medium">{confirming.author_key}</p>
                  <p className="text-xs text-muted-foreground">
                    {COPY.bindings.identityLine(confirming.key_kind, confirming.provider)}
                  </p>
                  {confirming.suggested_user_id ? (
                    <p className="text-xs text-muted-foreground">
                      {COPY.bindings.suggestedLine(suggestedName ?? '')}
                    </p>
                  ) : null}
                </div>
                {memberOptions.error ? (
                  <div className="grid gap-1.5">
                    <Label htmlFor="scm-binding-user">{COPY.bindings.memberIdFallback}</Label>
                    <Input
                      id="scm-binding-user"
                      className="w-32"
                      inputMode="numeric"
                      value={userId ?? ''}
                      onChange={(event) => {
                        const parsed = Number(event.target.value)
                        setUserId(Number.isInteger(parsed) && parsed > 0 ? parsed : undefined)
                      }}
                    />
                    <p className="text-xs text-muted-foreground">{COPY.bindings.memberLoadFailed}</p>
                    <Button variant="ghost" size="sm" onClick={memberOptions.retry}>
                      {COPY.common.retry}
                    </Button>
                  </div>
                ) : (
                  <div className="grid gap-1.5">
                    <Label htmlFor="scm-binding-member">{COPY.bindings.member}</Label>
                    <UserSelect
                      id="scm-binding-member"
                      label={COPY.bindings.member}
                      className="w-56"
                      value={userId}
                      options={memberOptions.options}
                      disabled={memberOptions.pending}
                      onValueChange={setUserId}
                    />
                  </div>
                )}
                <div className="flex items-center gap-2">
                  <Button
                    disabled={confirmMutation.isPending || !userId}
                    onClick={() =>
                      confirmMutation.mutate(
                        { id: confirming.id, userId: userId as number },
                        { onSuccess: () => setConfirming(null) },
                      )
                    }
                  >
                    {COPY.common.confirm}
                  </Button>
                  {confirming.status === 'suggested' ? (
                    <Button
                      variant="outline"
                      disabled={rejectMutation.isPending}
                      onClick={() => rejectMutation.mutate(confirming.id, { onSuccess: () => setConfirming(null) })}
                    >
                      {COPY.bindings.reject}
                    </Button>
                  ) : null}
                  <Button variant="ghost" onClick={() => setConfirming(null)}>
                    {COPY.common.cancel}
                  </Button>
                </div>
              </div>
            ) : null}
          </div>
        </>
      )}
    </DetailSection>
  )
}
