import { createStore } from 'zustand/vanilla'

import type { PMHostData, PMHostUser } from './microApp'
import { logPMAuthDiagnostic } from './authDiagnostics'

export const PM_STANDALONE_TOKEN_KEY = 'pm_standalone_token'
export const PM_STANDALONE_WORKSPACE_KEY = 'pm_workspace_id'

export type PMRuntimeMode = 'standalone' | 'embedded'

export type PMRuntimeSessionState = {
  mode: PMRuntimeMode
  token: string
  user: PMHostUser | null
  timezone: string
  revision: number
  setStandaloneToken: (token: string) => void
  applyEmbeddedSession: (input: {
    token: string
    workspaceId: number
    timezone?: string
  }) => void
  applyHostData: (data: PMHostData) => void
  setIdentity: (user: PMHostUser | null) => void
  clearSession: (reason?: string) => void
}

export type PMRuntimeWorkspaceState = {
  currentWorkspaceId: number
  selectWorkspace: (workspaceId: number) => void
  applyHostWorkspace: (workspaceId: number) => void
  clearWorkspace: () => void
}

const embeddedAtStartup = Boolean(
  window.__MICRO_APP_ENVIRONMENT__ ||
    (window as Window & { microApp?: unknown }).microApp,
)

function positiveId(value: unknown): number {
  const id = Number(value || 0)
  return Number.isFinite(id) && id > 0 ? id : 0
}

function sessionReset(revision: number) {
  return {
    user: null,
    revision: revision + 1,
  }
}

function sameIdentity(current: PMHostUser | null, next: PMHostUser | null): boolean {
  return (
    current?.id === next?.id &&
    current?.username === next?.username &&
    current?.role === next?.role &&
    current?.display_name === next?.display_name &&
    current?.avatar_url === next?.avatar_url &&
    current?.status === next?.status &&
    current?.password_reset_required === next?.password_reset_required
  )
}

function restoreStandaloneWorkspace(): void {
  const workspaceId = positiveId(
    localStorage.getItem(PM_STANDALONE_WORKSPACE_KEY),
  )
  runtimeWorkspaceStore.setState({ currentWorkspaceId: workspaceId })
}

const initialWorkspaceId = embeddedAtStartup
  ? 0
  : positiveId(localStorage.getItem(PM_STANDALONE_WORKSPACE_KEY))

export const runtimeWorkspaceStore = createStore<PMRuntimeWorkspaceState>((set) => ({
  currentWorkspaceId: initialWorkspaceId,
  selectWorkspace: (workspaceId) => {
    const id = positiveId(workspaceId)
    set({ currentWorkspaceId: id })
    if (id) localStorage.setItem(PM_STANDALONE_WORKSPACE_KEY, String(id))
    else localStorage.removeItem(PM_STANDALONE_WORKSPACE_KEY)
  },
  applyHostWorkspace: (workspaceId) => {
    const id = positiveId(workspaceId)
    set({ currentWorkspaceId: id })
  },
  clearWorkspace: () => {
    set({ currentWorkspaceId: 0 })
    localStorage.removeItem(PM_STANDALONE_WORKSPACE_KEY)
  },
}))

export const runtimeSessionStore = createStore<PMRuntimeSessionState>((set, get) => ({
  mode: embeddedAtStartup ? 'embedded' : 'standalone',
  token: embeddedAtStartup ? '' : localStorage.getItem(PM_STANDALONE_TOKEN_KEY) || '',
  user: null,
  timezone: '',
  revision: 0,
  setStandaloneToken: (rawToken) => {
    const token = String(rawToken || '')
    const current = get()
    if (token) localStorage.setItem(PM_STANDALONE_TOKEN_KEY, token)
    else localStorage.removeItem(PM_STANDALONE_TOKEN_KEY)
    if (current.mode !== 'standalone') restoreStandaloneWorkspace()
    set({
      mode: 'standalone',
      token,
      ...(current.mode !== 'standalone' || current.token !== token
        ? sessionReset(current.revision)
        : {}),
    })
  },
  applyEmbeddedSession: (input) => {
    const current = get()
    const identityChanged =
      current.mode !== 'embedded' || current.token !== input.token
    set({
      mode: 'embedded',
      token: input.token,
      timezone: input.timezone || current.timezone,
      ...(identityChanged
        ? sessionReset(current.revision)
        : {}),
    })
    runtimeWorkspaceStore.getState().applyHostWorkspace(input.workspaceId)
  },
  applyHostData: (data) => {
    const current = get()
    const identityChanged =
      current.mode !== 'embedded' ||
      current.token !== data.auth.token
    set({
      mode: 'embedded',
      token: data.auth.token,
      timezone: data.timezone || current.timezone,
      ...(identityChanged
        ? sessionReset(current.revision)
        : {}),
      // One2All supplies only the bearer bootstrap identity. Once /auth/me has
      // resolved the PM-owned user for this token, subsequent host route/workspace
      // messages must not overwrite PM admission state or profile fields.
      user: identityChanged || !current.user ? data.auth.user : current.user,
    })
    runtimeWorkspaceStore.getState().applyHostWorkspace(data.workspace.id)
  },
  setIdentity: (user) => {
    const current = get()
    set({
      ...(sameIdentity(current.user, user)
        ? {}
        : sessionReset(current.revision)),
      user,
    })
  },
  clearSession: (reason = 'direct_clear') => {
    const current = get()
    const workspaceId = runtimeWorkspaceStore.getState().currentWorkspaceId
    if (current.token || current.user || workspaceId > 0) {
      logPMAuthDiagnostic('session.clear', {
        reason,
        mode: current.mode,
        hadToken: Boolean(current.token),
        userId: current.user?.id || 0,
        workspaceId,
        revision: current.revision,
      })
    }
    if (current.mode === 'standalone') {
      localStorage.removeItem(PM_STANDALONE_TOKEN_KEY)
      runtimeWorkspaceStore.getState().clearWorkspace()
    } else {
      runtimeWorkspaceStore.getState().applyHostWorkspace(0)
    }
    set({
      token: '',
      timezone: '',
      ...sessionReset(current.revision),
    })
  },
}))

export function isRuntimeLoggedIn(state: PMRuntimeSessionState): boolean {
  return Boolean(state.token)
}
