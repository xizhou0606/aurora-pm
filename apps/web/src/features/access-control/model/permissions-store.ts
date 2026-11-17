import { createStore } from 'zustand/vanilla'
import { useStore } from 'zustand'

import type { PMActionCode } from '@/generated/access-catalog'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { listMyActionPermissions } from '../api/api'

export type PermissionsStatus = 'idle' | 'loading' | 'ready' | 'error'

export type PermissionsStoreState = {
  scopeKey: string
  status: PermissionsStatus
  actionCodes: readonly PMActionCode[]
  request: Promise<void> | null
  ensure: (scopeKey: string) => Promise<void>
  reload: (scopeKey: string, background?: boolean) => Promise<void>
  seed: (scopeKey: string, actionCodes: readonly PMActionCode[]) => void
  reset: () => void
}

/**
 * The `/access/me/action-permissions` response depends on the bearer token and
 * the `X-Workspace-ID` header, so grants are cached per `revision:workspaceId`
 * scope and only served while that scope still matches the live runtime state.
 * A mismatch (logout, identity change, workspace switch) is surfaced as
 * `loading` until the next `ensure`, which keeps stale grants unservable.
 */
export function currentPermissionsScopeKey(): string {
  const session = runtimeSessionStore.getState()
  const workspaceId = runtimeWorkspaceStore.getState().currentWorkspaceId
  return `${session.revision}:${workspaceId}`
}

export const permissionsStore = createStore<PermissionsStoreState>()((set, get) => ({
  scopeKey: '',
  status: 'idle',
  actionCodes: [],
  request: null,
  ensure: (scopeKey) => {
    const state = get()
    if (state.scopeKey === scopeKey && state.request) return state.request
    if (state.scopeKey === scopeKey && state.status === 'ready') return Promise.resolve()
    return state.reload(scopeKey)
  },
  reload: (scopeKey, background = false) => {
    const state = get()
    if (state.scopeKey === scopeKey && state.request) return state.request
    const request = listMyActionPermissions()
      .then((actionCodes) => {
        if (get().scopeKey === scopeKey) {
          set({ status: 'ready', actionCodes, request: null })
        }
      })
      .catch(() => {
        if (get().scopeKey === scopeKey) {
          set({ status: 'error', request: null })
        }
      })
    const preserveReady = background && state.scopeKey === scopeKey && state.status === 'ready'
    set({ scopeKey, status: preserveReady ? 'ready' : 'loading', request })
    return request
  },
  seed: (scopeKey, actionCodes) => {
    set({ scopeKey, status: 'ready', actionCodes, request: null })
  },
  reset: () => {
    set({ scopeKey: '', status: 'idle', actionCodes: [], request: null })
  },
}))

export function usePermissionsStore<T>(
  selector: (state: PermissionsStoreState) => T,
): T {
  return useStore(permissionsStore, selector)
}

export function seedActionPermissions(actionCodes: readonly PMActionCode[]): void {
  permissionsStore.getState().seed(currentPermissionsScopeKey(), actionCodes)
}

export function reloadActionPermissions(): Promise<void> {
  return permissionsStore.getState().reload(currentPermissionsScopeKey())
}
