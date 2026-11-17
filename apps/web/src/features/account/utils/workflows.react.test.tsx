import type { ReactNode } from 'react'
import { act, renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const adapters = (() => ({
  startIdentityBinding: vi.fn(),
  unbindIdentity: vi.fn(),
  updatePassword: vi.fn(),
  updateProfile: vi.fn(),
}))()

mock.module('../api/api', () => ({
  ...adapters,
}))

import { pmKeys } from '@/shared/query'
import { copy } from '@/shared/copy'
import { useAccountActions } from './workflows'

describe('account workflows', () => {
  let queryClient: QueryClient
  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = new QueryClient()
  })

  function wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }

  it('validates password confirmation before transport', async () => {
    const hook = renderHook(() => useAccountActions(), { wrapper })
    await act(async () => {
      await hook.result.current.savePassword({
        currentPassword: 'old-private-12',
        newPassword: 'new-private-12',
        passwordConfirmation: 'different-value',
      }, true)
    })

    expect(adapters.updatePassword).not.toHaveBeenCalled()
    expect(hook.result.current.state).toEqual({
      status: 'error',
      action: 'password',
      message: copy.account.passwordMismatch,
    })
  })

  it('unbinds one provider and invalidates only the account query', async () => {
    adapters.unbindIdentity.mockResolvedValue(undefined)
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
      .mockResolvedValue(undefined)
    const hook = renderHook(() => useAccountActions(), { wrapper })

    await act(async () => {
      await hook.result.current.unbind('telegram')
    })

    expect(adapters.unbindIdentity).toHaveBeenCalledWith('telegram')
    expect(invalidate).toHaveBeenCalledWith({ queryKey: pmKeys.account() })
    expect(hook.result.current.state).toEqual({
      status: 'success',
      action: 'identity',
      message: copy.account.identityUnbound,
    })
  })
})
