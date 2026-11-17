import { createStore } from 'zustand/vanilla'
import { useStore } from 'zustand'

import { runtimeSessionStore } from '@/shared/runtime'
import { fetchWorkspaces } from '../api/api'
import type { WorkspaceOption } from './types'

export type WorkspacesStatus = 'idle' | 'loading' | 'ready' | 'error'

export type WorkspacesStoreState = {
  revision: number
  status: WorkspacesStatus
  items: readonly WorkspaceOption[]
  request: Promise<void> | null
  ensure: (revision: number) => Promise<void>
  reload: (revision: number) => Promise<void>
  seed: (revision: number, items: readonly WorkspaceOption[]) => void
  reset: () => void
}

/**
 * The `/auth/me/workspaces` list only depends on the bearer session, so it is
 * cached per session revision and reused by the route guard, the shell
 * switcher and the workspace selection page. `reload` always refetches and
 * rejects with the API error so the login/switch workflows can own failures;
 * `ensure` resolves from the cache whenever the revision already has data.
 */
export const workspacesStore = createStore<WorkspacesStoreState>()((set, get) => ({
  revision: -1,
  status: 'idle',
  items: [],
  request: null,
  ensure: (revision) => {
    const state = get()
    if (state.revision === revision && state.request) return state.request
    if (state.revision === revision && state.status === 'ready') return Promise.resolve()
    return state.reload(revision)
  },
  reload: (revision) => {
    const state = get()
    if (state.revision === revision && state.request) return state.request
    const request = fetchWorkspaces()
      .then((items) => {
        if (get().revision === revision) {
          set({ status: 'ready', items, request: null })
        }
      })
      .catch((error) => {
        if (get().revision === revision) {
          set({ status: 'error', request: null })
        }
        throw error
      })
    set({ revision, status: 'loading', request })
    return request
  },
  seed: (revision, items) => {
    set({ revision, status: 'ready', items, request: null })
  },
  reset: () => {
    set({ revision: -1, status: 'idle', items: [], request: null })
  },
}))

export function useWorkspacesStore<T>(
  selector: (state: WorkspacesStoreState) => T,
): T {
  return useStore(workspacesStore, selector)
}

export function seedWorkspacesForCurrentSession(
  items: readonly WorkspaceOption[],
): void {
  workspacesStore.getState().seed(runtimeSessionStore.getState().revision, items)
}
