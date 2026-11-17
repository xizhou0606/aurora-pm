import { useCallback, useRef, useState } from 'react'
import {
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'

import { waitForPMCacheIsolation } from '@/shared/query'
import { PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { workspacesStore } from '../model/workspaces-store'
import {
  normalizeWorkspaceOptions,
  resolveWorkspaceSelection,
} from '../hooks/selection'
import type {
  WorkspaceRouteResolution,
  WorkspaceSelection,
} from '../model/types'

export type WorkspaceSwitchOptions = {
  navigate: () => Promise<unknown> | unknown
}

export type WorkspaceSwitcher = {
  errorMessage: string
  pending: boolean
  switchWorkspace: (workspaceId: number) => Promise<boolean>
}

async function resolveStandaloneWorkspace(
  queryClient: QueryClient,
): Promise<WorkspaceSelection> {
  const session = runtimeSessionStore.getState()
  if (session.mode !== 'standalone' || !session.token) {
    throw new PMSafeApiError('WORKSPACE_SESSION_UNAVAILABLE', 401)
  }
  const ticket = {
    token: session.token,
    revision: session.revision,
  }
  await waitForPMCacheIsolation(queryClient)
  const cached = workspacesStore.getState()
  if (cached.revision !== ticket.revision || cached.status !== 'ready') {
    await workspacesStore.getState().ensure(ticket.revision)
  }
  const options = workspacesStore.getState().items
  const latest = runtimeSessionStore.getState()
  if (
    latest.mode !== 'standalone' ||
    latest.token !== ticket.token ||
    latest.revision !== ticket.revision
  ) {
    throw new PMSafeApiError('STALE_SESSION')
  }

  const resolution = resolveWorkspaceSelection(
    options,
    runtimeWorkspaceStore.getState().currentWorkspaceId,
  )
  if (resolution.status === 'ok') {
    const workspace = runtimeWorkspaceStore.getState()
    if (workspace.currentWorkspaceId !== resolution.workspaceId) {
      workspace.selectWorkspace(resolution.workspaceId)
      await waitForPMCacheIsolation(queryClient)
    }
  }
  return resolution
}

export async function resolveWorkspaceAfterLogin(
  queryClient: QueryClient,
): Promise<WorkspaceSelection> {
  return resolveStandaloneWorkspace(queryClient)
}

export async function resolveWorkspaceForRoute(
  queryClient: QueryClient,
): Promise<WorkspaceRouteResolution> {
  const session = runtimeSessionStore.getState()
  const currentWorkspaceId = runtimeWorkspaceStore.getState().currentWorkspaceId
  if (session.mode === 'embedded') {
    return currentWorkspaceId > 0
      ? { status: 'ok', workspaceId: currentWorkspaceId, reason: 'embedded' }
      : { status: 'error' }
  }
  try {
    return await resolveStandaloneWorkspace(queryClient)
  } catch {
    return { status: 'error' }
  }
}

export async function switchWorkspaceWorkflow(
  queryClient: QueryClient,
  workspaceId: number,
  options: WorkspaceSwitchOptions,
): Promise<boolean> {
  const id = Number(workspaceId || 0)
  if (!Number.isInteger(id) || id <= 0) return false

  const session = runtimeSessionStore.getState()
  if (session.mode !== 'standalone' || !session.token) return false
  const ticket = { token: session.token, revision: session.revision }
  await waitForPMCacheIsolation(queryClient)
  await workspacesStore.getState().reload(ticket.revision)
  const workspaces = normalizeWorkspaceOptions(workspacesStore.getState().items)
  const latest = runtimeSessionStore.getState()
  if (
    latest.mode !== 'standalone' ||
    latest.token !== ticket.token ||
    latest.revision !== ticket.revision ||
    !workspaces.some((workspace) => workspace.id === id)
  ) {
    return false
  }

  runtimeWorkspaceStore.getState().selectWorkspace(id)
  await waitForPMCacheIsolation(queryClient)
  if (runtimeWorkspaceStore.getState().currentWorkspaceId !== id) return false

  await options.navigate()
  return runtimeWorkspaceStore.getState().currentWorkspaceId === id
}

export function useWorkspaceSwitcher(
  options: WorkspaceSwitchOptions,
): WorkspaceSwitcher {
  const queryClient = useQueryClient()
  const pendingRef = useRef(false)
  const [pending, setPending] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const { navigate } = options

  const switchWorkspace = useCallback(
    async (workspaceId: number): Promise<boolean> => {
      if (pendingRef.current) return false
      pendingRef.current = true
      setPending(true)
      setErrorMessage('')
      try {
        return await switchWorkspaceWorkflow(queryClient, workspaceId, {
          navigate,
        })
      } catch {
        setErrorMessage(copy.workspace.loadFailed)
        return false
      } finally {
        pendingRef.current = false
        setPending(false)
      }
    },
    [navigate, queryClient],
  )

  return { errorMessage, pending, switchWorkspace }
}
