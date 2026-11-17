import { reloadActionPermissions } from '@/features/access-control'
import { PMSafeApiError } from '@/shared/api'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useWorkspaceStore } from '@/shared/state'
import { pmKeys } from '@/shared/query'
import { copy } from '@/shared/copy'
import type { DataTableState } from '@/shared/ui'
import { fetchAuthorizationRecords, revokeAuthorizationRecord } from '../api/authorization-records'
import type { AuthorizationRecord } from '../model/authorization-records'

export function useAuthorizationRecords(enabled: boolean, canRevoke: boolean) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [selected, setSelected] = useState<AuthorizationRecord | null>(null)
  const key = pmKeys.resource(workspaceId > 0 ? workspaceId : 1, 'workspace', 'externalTooling', 'authorizationRecords')
  const query = useQuery({
    queryKey: [...key, page, pageSize],
    queryFn: ({ signal }) => fetchAuthorizationRecords(page, pageSize, signal),
    enabled: enabled && workspaceId > 0,
    retry: false,
    refetchInterval: enabled ? 10_000 : false,
  })
  const revoke = useMutation({
    mutationFn: ({ id }: { id: string; workspaceId: number }) => revokeAuthorizationRecord(id),
    onSuccess: (_data, variables) => {
      setSelected(null)
      void queryClient.invalidateQueries({ queryKey: pmKeys.resource(variables.workspaceId, 'workspace', 'externalTooling') })
    },
  })
  useEffect(() => { setPage(1); setSelected(null); revoke.reset() }, [workspaceId])
  const denied = query.error instanceof PMSafeApiError && query.error.status === 403
  useEffect(() => { if (denied) void reloadActionPermissions() }, [denied])
  useEffect(() => { if (!enabled || !canRevoke) setSelected(null) }, [enabled, canRevoke])
  const state: DataTableState<AuthorizationRecord> = !enabled || denied || workspaceId <= 0
    ? { status: 'empty', title: copy.app.pageAccess.deniedTitle }
    : query.isPending
    ? { status: 'loading', label: copy.mcpCli.grants.loading }
    : query.isError
      ? { status: 'error', title: copy.mcpCli.grants.loadFailedTitle, onRetry: () => { void query.refetch() } }
      : !query.data?.items.length
        ? { status: 'empty', title: copy.mcpCli.grants.emptyTitle }
        : { status: 'ready', rows: query.data.items.map((record) => ({ ...record, can_revoke: canRevoke && record.can_revoke })), refreshing: query.isFetching }
  return {
    state, page, pageSize, total: enabled && !denied ? query.data?.total ?? 0 : 0,
    setPage,
    setPageSize: (value: number) => { setPageSize(value); setPage(1) },
    selected,
    select: (record: AuthorizationRecord) => { revoke.reset(); setSelected(record) },
    close: () => { if (!revoke.isPending) { setSelected(null); revoke.reset() } },
    confirm: () => { if (enabled && selected && canRevoke && selected.can_revoke && !revoke.isPending) revoke.mutate({ id: selected.id, workspaceId }) },
    pending: revoke.isPending,
    error: revoke.isError ? copy.mcpCli.grants.revokeFailed : undefined,
  }
}
