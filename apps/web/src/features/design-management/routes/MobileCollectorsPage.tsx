import { useState } from 'react'
import {
  PhArrowLeft as ArrowLeft,
  PhArrowsClockwise as RefreshCw,
  PhDeviceMobile as Smartphone,
  PhKey as KeyRound,
  PhLaptop as Laptop,
  PhShieldWarning as ShieldX,
  PhTrash as Trash2,
} from '@/shared/ui/icons'
import { useNavigate } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'

import { copy } from '../copy'
import { useWorkspaceStore } from '@/shared/state'
import {
  AlertBanner,
  Button,
  ConfirmDialog,
  DataTable,
  LoadingSkeleton,
  OperationalPage,
  StatusPill,
  TableActionButton,
} from '@/shared/ui'
import {
  useCaptureDrafts,
  useCollectorMutations,
  useCollectors,
  useDesignReviewAccess,
  useReviewCapabilities,
} from '../hooks'
import type { MobileCaptureDraft, MobileCollector } from '../model'

type PendingDelete = { kind: 'collector' | 'capture'; id: string }

export function MobileCollectorsPage() {
  const navigate = useNavigate({ from: '/workspace/design-management/mobile-collectors' })
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const capabilities = useReviewCapabilities(workspaceId)
  const access = useDesignReviewAccess(capabilities.data)
  const collectors = useCollectors(workspaceId, access.canManageCollectors)
  const captures = useCaptureDrafts(workspaceId, access.canManageCollectors)
  const mutations = useCollectorMutations(workspaceId)
  const [pairing, setPairing] = useState<{ code: string; expires_at: string }>()
  const [pendingDelete, setPendingDelete] = useState<PendingDelete>()
  const pair = () => mutations.pair.mutate(undefined, { onSuccess: setPairing })
  const remove = () => {
    if (!pendingDelete) return
    const mutation = pendingDelete.kind === 'collector' ? mutations.revoke : mutations.removeCapture
    mutation.mutate(pendingDelete.id, { onSuccess: () => setPendingDelete(undefined) })
  }
  const refresh = () => void Promise.all([collectors.refetch(), captures.refetch()])
  const loading = collectors.isPending || captures.isPending
  const error = collectors.isError || captures.isError
  const back = <Button variant="outline" onClick={() => void navigate({ to: '/workspace/design-management' })}><ArrowLeft />{copy.designManagement.collectors.back}</Button>
  if (capabilities.isPending || access.status === 'loading') {
    return <OperationalPage title={copy.designManagement.collectors.title} actions={back}><div className="p-5"><LoadingSkeleton rows={8} /></div></OperationalPage>
  }
  if (capabilities.isError || access.status === 'error') {
    return <OperationalPage title={copy.designManagement.collectors.title} actions={back}><div className="p-5"><AlertBanner tone="error" action={<Button size="sm" variant="outline" onClick={() => { access.retry(); void capabilities.refetch() }}>{copy.designManagement.common.retry}</Button>}>{copy.designManagement.collectors.accessLoadFailed}</AlertBanner></div></OperationalPage>
  }
  if (!access.canManageCollectors) {
    return <OperationalPage title={copy.designManagement.collectors.title} actions={back}><div className="p-5"><AlertBanner tone="warning">{copy.designManagement.collectors.manageUnavailable}</AlertBanner></div></OperationalPage>
  }
  return (
    <OperationalPage
      title={copy.designManagement.collectors.title}

      refresh={collectors.isFetching || captures.isFetching ? 'refreshing' : 'idle'}
      actions={<>
        {back}
        <Button variant="outline" onClick={refresh}><RefreshCw />{copy.designManagement.common.refresh}</Button>
        {access.canManageCollectors ? <Button disabled={mutations.pair.isPending} onClick={pair}><KeyRound />{copy.designManagement.collectors.newPairing}</Button> : null}
      </>}
    >
      <div className="grid min-w-0 gap-6 p-3 sm:p-5">
        <section className="grid gap-3 border-b border-border pb-5">
          {pairing ? <AlertBanner tone="success"><div className="flex flex-wrap items-center justify-between gap-3"><div><span className="block text-xs">{copy.designManagement.collectors.pairingCode}</span><strong className="font-mono text-lg tracking-widest">{pairing.code}</strong></div><span className="text-xs">{copy.designManagement.collectors.pairingExpires(formatDate(pairing.expires_at))}<br />{copy.designManagement.collectors.pairingHint}</span></div></AlertBanner> : null}
          {mutations.pair.isError ? <AlertBanner tone="error">{errorMessage(mutations.pair.error)}</AlertBanner> : null}
        </section>

        {error ? <AlertBanner tone="error" action={<Button size="sm" variant="outline" onClick={refresh}>{copy.designManagement.common.retry}</Button>}>{copy.designManagement.collectors.loadFailed}</AlertBanner> : null}
        <section className="grid min-w-0 gap-3">
          <div className="flex items-center gap-2"><Laptop className="size-4" /><h2 className="text-sm font-semibold">{copy.designManagement.collectors.paired}</h2></div>
          <DataTable aria-label={copy.designManagement.collectors.paired} columns={collectorColumns((id) => setPendingDelete({ kind: 'collector', id }))} state={loading ? { status: 'loading', skeletonRows: 3 } : (collectors.data?.length ?? 0) > 0 ? { status: 'ready', rows: collectors.data ?? [] } : { status: 'empty', title: copy.designManagement.collectors.emptyCollectors }} />
        </section>
        <section className="grid min-w-0 gap-3">
          <div className="flex items-center gap-2"><Smartphone className="size-4" /><h2 className="text-sm font-semibold">{copy.designManagement.collectors.drafts}</h2></div>
          <DataTable aria-label={copy.designManagement.collectors.drafts} columns={captureColumns((id) => setPendingDelete({ kind: 'capture', id }))} state={loading ? { status: 'loading', skeletonRows: 3 } : (captures.data?.length ?? 0) > 0 ? { status: 'ready', rows: captures.data ?? [] } : { status: 'empty', title: copy.designManagement.collectors.emptyDrafts }} />
        </section>
      </div>
      <ConfirmDialog
        open={Boolean(pendingDelete)}
        onOpenChange={(open) => { if (!open) setPendingDelete(undefined) }}
        onConfirm={remove}
        title={pendingDelete?.kind === 'collector' ? copy.designManagement.collectors.confirmRevokeTitle : copy.designManagement.collectors.confirmDraftTitle}
        description={pendingDelete?.kind === 'collector' ? copy.designManagement.collectors.confirmRevokeDescription : copy.designManagement.collectors.confirmDraftDescription}
        cancelLabel={copy.designManagement.common.cancel}
        confirmLabel={copy.designManagement.common.confirm}
        pending={mutations.revoke.isPending || mutations.removeCapture.isPending}
        error={mutations.revoke.isError ? errorMessage(mutations.revoke.error) : mutations.removeCapture.isError ? errorMessage(mutations.removeCapture.error) : undefined}
      />
    </OperationalPage>
  )
}

function collectorColumns(onRevoke: (id: string) => void): ColumnDef<MobileCollector>[] {
  const text = copy.designManagement.collectors
  return [
    { accessorKey: 'name', header: text.name, cell: ({ row }) => <div><strong className="text-sm">{row.original.name}</strong><span className="block text-xs text-muted-foreground">{copy.designManagement.common.metadata(row.original.hostname, row.original.architecture)}</span></div> },
    { accessorKey: 'version', header: text.version, cell: ({ row }) => <span className="text-sm">{row.original.version}<span className="block text-xs text-muted-foreground">{copy.designManagement.common.mobilewrightVersion(row.original.mobilewright_version)}</span></span> },
    { id: 'doctor', header: text.doctor, cell: ({ row }) => <div className="max-w-64"><StatusPill tone={row.original.doctor_ready ? 'success' : 'warning'}>{row.original.doctor_ready ? text.active : copy.designManagement.common.unavailable}</StatusPill><span className="mt-1 block text-xs text-muted-foreground">{row.original.doctor_summary || copy.designManagement.common.none}</span></div> },
    { id: 'seen', header: text.lastSeen, cell: ({ row }) => <span className="whitespace-nowrap text-xs">{formatDate(row.original.last_seen_at)}</span> },
    { id: 'status', header: text.status, cell: ({ row }) => <StatusPill tone={row.original.revoked_at ? 'danger' : 'success'}>{row.original.revoked_at ? text.revoked : text.active}</StatusPill> },
    { id: 'actions', header: '', cell: ({ row }) => <TableActionButton label={text.revoke} icon={<ShieldX />} disabled={Boolean(row.original.revoked_at)} onClick={() => onRevoke(row.original.id)} /> },
  ]
}

function captureColumns(onRemove: (id: string) => void): ColumnDef<MobileCaptureDraft>[] {
  const text = copy.designManagement.collectors
  return [
    { id: 'device', header: text.device, cell: ({ row }) => <div><strong className="text-sm">{row.original.device_id}</strong><span className="block text-xs text-muted-foreground">{row.original.platform.toUpperCase()}</span></div> },
    { accessorKey: 'status', header: text.status, cell: ({ row }) => <StatusPill tone={row.original.status === 'succeeded' || row.original.status === 'consumed' ? 'success' : row.original.last_error ? 'danger' : 'info'}>{row.original.status}</StatusPill> },
    { id: 'app', header: text.app, cell: ({ row }) => <span className="text-sm">{row.original.app_id || copy.designManagement.common.none}</span> },
    { id: 'captured', header: text.capturedAt, cell: ({ row }) => <span className="whitespace-nowrap text-xs">{formatDate(row.original.captured_at)}</span> },
    { id: 'expires', header: text.expiresAt, cell: ({ row }) => <span className="whitespace-nowrap text-xs">{formatDate(row.original.expires_at)}</span> },
    { id: 'actions', header: '', cell: ({ row }) => <TableActionButton label={text.deleteDraft} icon={<Trash2 />} onClick={() => onRemove(row.original.id)} /> },
  ]
}

function formatDate(value?: string): string {
  return value ? new Date(value).toLocaleString('zh-CN') : copy.designManagement.common.none
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : copy.designManagement.collectors.loadFailed
}
