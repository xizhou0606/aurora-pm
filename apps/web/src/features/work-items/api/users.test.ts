import { describe, expect, it, vi } from 'bun:test'

import type { WorkItemsClient } from './client'
import { requestPMWorkItemUsers } from './users'

describe('work-items PM user directory', () => {
  it('uses the PM core user directory and normalizes display labels', async () => {
    const requestPMUsers = vi.fn().mockResolvedValue([
      { id: 7, display_name: 'Teo', username: 'teo', avatar_url: '/teo.png' },
      { id: 8, username: 'alice' },
    ])
    const client = { requestPMUsers } as unknown as WorkItemsClient

    await expect(requestPMWorkItemUsers(client)).resolves.toEqual([
      { id: 7, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' },
      { id: 8, label: 'alice', username: 'alice', avatarUrl: undefined },
    ])
    expect(requestPMUsers).toHaveBeenCalledTimes(1)
  })

  it('rejects malformed users rather than widening the assignee scope', async () => {
    const client = {
      requestPMUsers: vi.fn().mockResolvedValue([{ id: 0, display_name: 'invalid' }]),
    } as unknown as WorkItemsClient

    await expect(requestPMWorkItemUsers(client)).rejects.toThrow(
      'invalid_work_item_user_directory',
    )
  })
})
