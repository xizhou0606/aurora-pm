import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const coreApiGet = (() => vi.fn())()

mock.module('@/shared/api', () => ({
  coreApi: { get: coreApiGet },
}))

import { aipmClient } from './client'

describe('AI PM user directory client', () => {
  beforeEach(() => {
    coreApiGet.mockReset()
  })

  it('requests one bounded workspace-user page', async () => {
    const controller = new AbortController()
    coreApiGet.mockResolvedValue({
      data: {
        data: [{ id: 21, username: 'reviewer', display_name: '评审同学' }],
        total: 45,
      },
    })

    await expect(aipmClient.workflowAssignees({
      limit: 20,
      offset: 20,
    }, controller.signal)).resolves.toEqual({
      items: [{ id: 21, username: 'reviewer', display_name: '评审同学' }],
      total: 45,
    })
    expect(coreApiGet).toHaveBeenCalledWith('/directory/users', {
      params: { limit: 20, offset: 20 },
      signal: controller.signal,
    })
  })
})
