import type { ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const adapterMocks = (() => ({
  completeExternalRegistration: vi.fn(),
  exchangeLoginCode: vi.fn(),
  fetchCurrentUser: vi.fn(),
  loginWithCredentials: vi.fn(),
  logoutFromHost: vi.fn(),
}))()

mock.module('../api/api', () => ({
  ...adapterMocks,
}))

import {
  installPMCacheIsolation,
  waitForPMCacheIsolation,
  type PMCacheIsolation,
} from '@/shared/query'
import { PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import {
  runtimeSessionStore,
  runtimeWorkspaceStore,
  type PMHostUser,
} from '@/shared/runtime'
import { ensureStandaloneIdentity } from './session'
import {
  useLoginWorkflow,
  useLogoutWorkflow,
} from './workflows'

const user: PMHostUser = { id: 1, username: 'teo', role: 'user' }

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => { resolve = resolvePromise })
  return { promise, resolve }
}

describe('auth workflows', () => {
  let queryClient: QueryClient
  let isolation: PMCacheIsolation

  beforeEach(() => {
    vi.clearAllMocks()
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
    runtimeWorkspaceStore.getState().clearWorkspace()
    queryClient = new QueryClient()
    isolation = installPMCacheIsolation(queryClient)
  })

  afterEach(() => {
    isolation.dispose()
    queryClient.clear()
  })

  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }

  it('validates a standalone token and stores the current identity', async () => {
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    await waitForPMCacheIsolation(queryClient)
    adapterMocks.fetchCurrentUser.mockResolvedValue(user)

    await expect(ensureStandaloneIdentity(queryClient)).resolves.toEqual(user)
    expect(runtimeSessionStore.getState().user).toEqual(user)
    expect(adapterMocks.fetchCurrentUser).toHaveBeenCalledTimes(1)
  })

  it('finishes a Feishu handoff with the same standalone session flow', async () => {
    adapterMocks.exchangeLoginCode.mockResolvedValue('feishu-session')
    adapterMocks.fetchCurrentUser.mockResolvedValue(user)
    const resolveDestination = vi.fn().mockResolvedValue('destination')
    const loginHook = renderHook(
      () => useLoginWorkflow({ resolveDestination }),
      { wrapper },
    )

    await expect(loginHook.result.current.exchangeLoginCode('one-time-code'))
      .resolves.toBe('destination' as never)

    expect(adapterMocks.exchangeLoginCode).toHaveBeenCalledWith('one-time-code')
    expect(runtimeSessionStore.getState().token).toBe('feishu-session')
    expect(runtimeSessionStore.getState().user).toEqual(user)
    expect(resolveDestination).toHaveBeenCalledWith(queryClient, user)
    expect(JSON.stringify(queryClient.getQueryCache().getAll())).not.toContain('one-time-code')
  })

  it('maps a failed Google handoff to the Google-specific safe message', async () => {
    adapterMocks.exchangeLoginCode.mockRejectedValue(
      new PMSafeApiError('AUTH_LOGIN_CODE_INVALID', 401),
    )
    const loginHook = renderHook(() => useLoginWorkflow(), { wrapper })

    await act(async () => {
      await loginHook.result.current.exchangeLoginCode('one-time-code', 'google')
    })

    expect(loginHook.result.current.errorMessage).toBe(copy.auth.googleLoginFailed)
    expect(runtimeSessionStore.getState().token).toBe('')
  })

  it('maps a failed Telegram handoff to the Telegram-specific safe message', async () => {
    adapterMocks.exchangeLoginCode.mockRejectedValue(
      new PMSafeApiError('AUTH_LOGIN_CODE_INVALID', 401),
    )
    const loginHook = renderHook(() => useLoginWorkflow(), { wrapper })

    await act(async () => {
      await loginHook.result.current.exchangeLoginCode('one-time-code', 'telegram')
    })

    expect(loginHook.result.current.errorMessage).toBe(copy.auth.telegramLoginFailed)
    expect(runtimeSessionStore.getState().token).toBe('')
  })

  it('completes guided registration and reuses login-code session exchange', async () => {
    adapterMocks.completeExternalRegistration.mockResolvedValue('fresh-login-code')
    adapterMocks.exchangeLoginCode.mockResolvedValue('registered-session')
    adapterMocks.fetchCurrentUser.mockResolvedValue(user)
    const resolveDestination = vi.fn().mockResolvedValue('destination')
    const loginHook = renderHook(() => useLoginWorkflow({ resolveDestination }), { wrapper })

    await expect(loginHook.result.current.registerExternal({
      registrationCode: 'registration-once',
      provider: 'google',
      username: 'teo.user',
      displayName: '铁奥',
      password: 'private-value-12',
      passwordConfirmation: 'private-value-12',
    })).resolves.toBe('destination' as never)

    expect(adapterMocks.completeExternalRegistration).toHaveBeenCalledWith({
      registrationCode: 'registration-once',
      username: 'teo.user',
      displayName: '铁奥',
      password: 'private-value-12',
      passwordConfirmation: 'private-value-12',
    })
    expect(adapterMocks.exchangeLoginCode).toHaveBeenCalledWith('fresh-login-code')
    expect(runtimeSessionStore.getState().token).toBe('registered-session')
    expect(JSON.stringify(queryClient.getQueryCache().getAll())).not.toContain('registration-once')
  })

  it('rejects a stale identity response without clearing the newer session', async () => {
    const identity = deferred<PMHostUser>()
    runtimeSessionStore.getState().setStandaloneToken('session-old')
    await waitForPMCacheIsolation(queryClient)
    adapterMocks.fetchCurrentUser.mockReturnValue(identity.promise)

    const hydration = ensureStandaloneIdentity(queryClient)
    await waitFor(() => expect(adapterMocks.fetchCurrentUser).toHaveBeenCalled())
    runtimeSessionStore.getState().setStandaloneToken('session-new')
    identity.resolve(user)

    await expect(hydration).rejects.toMatchObject({ code: 'STALE_SESSION' })
    expect(runtimeSessionStore.getState().token).toBe('session-new')
  })

  it('clears the new session when login workspace resolution fails', async () => {
    adapterMocks.loginWithCredentials.mockResolvedValue('session-a')
    adapterMocks.fetchCurrentUser.mockResolvedValue(user)
    const resolveDestination = vi.fn().mockRejectedValue(
      new PMSafeApiError('WORKSPACES_FAILED', 503),
    )
    const loginHook = renderHook(() => useLoginWorkflow({ resolveDestination }), { wrapper })

    await act(async () => { await loginHook.result.current.login('teo', 'private-value') })

    expect(runtimeSessionStore.getState().token).toBe('')
    expect(loginHook.result.current.errorMessage).toBe(copy.auth.loginFailed)
  })

  it('keeps credentials out of caches and logs out locally on remote failure', async () => {
    adapterMocks.loginWithCredentials.mockResolvedValue('session-a')
    adapterMocks.fetchCurrentUser.mockResolvedValue(user)
    const loginHook = renderHook(() => useLoginWorkflow(), { wrapper })

    await act(async () => { await loginHook.result.current.login('teo', 'private-value') })
    expect(queryClient.getMutationCache().getAll()).toHaveLength(0)
    expect(JSON.stringify(queryClient.getQueryCache().getAll())).not.toContain('private-value')

    adapterMocks.logoutFromHost.mockRejectedValue(new PMSafeApiError('LOGOUT_FAILED', 503))
    const logoutHook = renderHook(() => useLogoutWorkflow(), { wrapper })
    await act(async () => { await logoutHook.result.current.logout() })

    expect(runtimeSessionStore.getState().token).toBe('')
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(0)
  })
})
