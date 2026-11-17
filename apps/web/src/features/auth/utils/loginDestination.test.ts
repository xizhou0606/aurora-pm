import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it } from 'bun:test'

import type { PMHostUser } from '@/shared/runtime'
import { resolveLoginDestination } from './loginDestination'

describe('resolveLoginDestination', () => {
  it('routes an active forced-reset identity directly to password setup', async () => {
    const queryClient = new QueryClient()
    const user: PMHostUser = {
      id: 19,
      username: 'forced-reset-user',
      role: 'user',
      status: 'active',
      password_reset_required: true,
    }

    await expect(resolveLoginDestination(queryClient, user)).resolves.toEqual({
      status: 'password_reset_required',
    })
    expect(queryClient.getQueryCache().getAll()).toHaveLength(0)
  })

  it.each(['pending_approval', 'rejected', 'banned'] as const)(
    'routes a %s auto-provisioned identity to the account status guide',
    async (status) => {
      const queryClient = new QueryClient()
      const user: PMHostUser = {
        id: 18,
        username: 'feishu-user',
        role: 'user',
        status,
      }

      await expect(resolveLoginDestination(queryClient, user)).resolves.toEqual({
        status: 'identity_status',
      })
      expect(queryClient.getQueryCache().getAll()).toHaveLength(0)
    },
  )
})
