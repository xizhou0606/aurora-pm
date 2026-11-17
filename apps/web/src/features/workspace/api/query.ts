import { useEffect } from 'react'

import { useSessionStore } from '@/shared/state'
import { workspacesStore, useWorkspacesStore } from '../model/workspaces-store'
import type { WorkspaceOption } from '../model/types'

export type WorkspacesQueryState = {
  data: readonly WorkspaceOption[] | undefined
  isPending: boolean
  isSuccess: boolean
  isError: boolean
  refetch: () => Promise<void>
}

export function useWorkspacesQuery(): WorkspacesQueryState {
  const mode = useSessionStore((state) => state.mode)
  const token = useSessionStore((state) => state.token)
  const revision = useSessionStore((state) => state.revision)
  const enabled = mode === 'standalone' && Boolean(token)
  const storeRevision = useWorkspacesStore((state) => state.revision)
  const storeStatus = useWorkspacesStore((state) => state.status)
  const items = useWorkspacesStore((state) => state.items)

  useEffect(() => {
    if (!enabled) return
    void workspacesStore.getState().ensure(revision).catch(() => undefined)
  }, [enabled, revision])

  const scoped = storeRevision === revision
  const loaded = scoped && (storeStatus === 'ready' || storeStatus === 'error')
  return {
    data: loaded ? items : undefined,
    isPending: !loaded,
    isSuccess: scoped && storeStatus === 'ready',
    isError: scoped && storeStatus === 'error',
    refetch: () => workspacesStore.getState().reload(revision).then(() => undefined),
  }
}
