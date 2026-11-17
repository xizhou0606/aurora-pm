import { QueryClient } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

const apiMocks = (() => ({
  fetchCurrentUser: vi.fn(),
}))()

vi.mock('../api/query', () => ({
  currentUserQueryOptions: () => ({
    queryKey: ['pm', 'session', 'auth', 'me'],
    queryFn: ({ signal }: { signal?: AbortSignal }) =>
      apiMocks.fetchCurrentUser(signal),
    retry: false,
  }),
}))

import type { PMHostUser } from '@/shared/runtime'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  ensureAuthenticatedIdentity,
  ensureStandaloneIdentity,
} from './session'

const pmUser: PMHostUser = {
  id: 7,
  username: 'pm-user',
  display_name: 'PM User',
  role: 'member',
  status: 'active',
}

const hostUser: PMHostUser = {
  id: 99,
  username: 'host-user',
  role: 'viewer',
}

function resetRuntime() {
  runtimeSessionStore.getState().setStandaloneToken('')
  runtimeSessionStore.getState().clearSession()
  runtimeWorkspaceStore.getState().clearWorkspace()
}

describe('identity hydration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetRuntime()
  })

  it('resolves /auth/me once per session and serves later routes from the runtime store', async () => {
    apiMocks.fetchCurrentUser.mockResolvedValue(pmUser)
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    const queryClient = new QueryClient()

    await expect(ensureAuthenticatedIdentity(queryClient)).resolves.toEqual(pmUser)
    await expect(ensureAuthenticatedIdentity(queryClient)).resolves.toEqual(pmUser)

    expect(apiMocks.fetchCurrentUser).toHaveBeenCalledTimes(1)
    expect(runtimeSessionStore.getState().user).toEqual(pmUser)
  })

  it('refetches the identity after the session revision changes', async () => {
    apiMocks.fetchCurrentUser.mockResolvedValue(pmUser)
    const queryClient = new QueryClient()

    runtimeSessionStore.getState().setStandaloneToken('session-a')
    await ensureAuthenticatedIdentity(queryClient)

    runtimeSessionStore.getState().setStandaloneToken('session-b')
    await ensureAuthenticatedIdentity(queryClient)

    expect(apiMocks.fetchCurrentUser).toHaveBeenCalledTimes(2)
    expect(runtimeSessionStore.getState().user).toEqual(pmUser)
  })

  it('resolves the PM-owned identity even when the host already supplied a provisional user', async () => {
    apiMocks.fetchCurrentUser.mockResolvedValue(pmUser)
    const queryClient = new QueryClient()

    runtimeSessionStore.getState().applyHostData({
      contractVersion: 1,
      embedded: true,
      parentOrigin: 'https://host.example',
      auth: { token: 'host-token', user: hostUser },
      workspace: { id: 5 },
      route: { path: '/', fullPath: '/' },
    })

    await expect(ensureStandaloneIdentity(queryClient)).rejects.toMatchObject({
      code: 'STANDALONE_SESSION_MISSING',
    })
    await expect(ensureAuthenticatedIdentity(queryClient)).resolves.toEqual(pmUser)

    expect(apiMocks.fetchCurrentUser).toHaveBeenCalledTimes(1)
    expect(runtimeSessionStore.getState().user).toEqual(pmUser)
  })

  it('fails closed by clearing the session when hydration fails', async () => {
    apiMocks.fetchCurrentUser.mockRejectedValue(new Error('network down'))
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    const queryClient = new QueryClient()

    await expect(ensureAuthenticatedIdentity(queryClient)).rejects.toMatchObject({
      code: 'AUTH_ME_FAILED',
    })
    expect(runtimeSessionStore.getState().token).toBe('')
    expect(runtimeSessionStore.getState().user).toBeNull()
  })
})
