import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const clientMocks = (() => ({
  get: vi.fn(),
}))()

mock.module('@/shared/api', () => ({
    coreApi: { get: clientMocks.get },
  }))

import { PMSafeApiError } from '@/shared/api'
import { fetchWorkspaces } from './api'

describe('workspace adapter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('forwards cancellation and removes disabled, invalid and duplicate options', async () => {
    const signal = new AbortController().signal
    clientMocks.get.mockResolvedValue({
      data: {
        data: {
          items: [
            { id: 1, name: 'Alpha' },
            { id: 1, name: 'Duplicate' },
            { id: 2, name: 'Disabled', enabled: false },
            { id: 3, name: '', slug: 'from-slug' },
            { id: 0, name: 'Invalid' },
          ],
        },
      },
    })

    await expect(fetchWorkspaces(signal)).resolves.toEqual([
      { id: 1, name: 'Alpha' },
      { id: 3, name: 'from-slug', slug: 'from-slug' },
    ])
    expect(clientMocks.get).toHaveBeenCalledWith('/auth/me/workspaces', {
      signal,
      pmContext: { scope: 'session' },
    })
  })

  it('exposes only a safe error to Query consumers', async () => {
    clientMocks.get.mockRejectedValue({
      config: { headers: { Authorization: 'private-token' } },
      response: { status: 503, data: { code: 'WORKSPACES_UNAVAILABLE' } },
    })

    const error = await fetchWorkspaces().catch((reason: unknown) => reason)

    expect(error).toBeInstanceOf(PMSafeApiError)
    expect(error).toMatchObject({
      status: 503,
      code: 'WORKSPACES_UNAVAILABLE',
    })
    expect(error).not.toHaveProperty('config')
    expect(JSON.stringify(error)).not.toContain('private-token')
  })
})
