import { beforeEach, describe, expect, it, mock, vi } from 'bun:test'

const feature = (() => ({
  WorkItemsPage: vi.fn(() => null),
}))()

mock.module('@/features/work-items', () => feature)

import {
  loadLocalWorkItems,
  resetLocalWorkItemsLoaderForTests,
} from './local-work-items-loader'

describe('local Work-items loader', () => {
  beforeEach(() => {
    resetLocalWorkItemsLoaderForTests()
  })

  it('loads and caches the PM-owned feature without a remote manifest', async () => {
    const first = await loadLocalWorkItems()
    const second = await loadLocalWorkItems()

    expect(first).toBe(feature.WorkItemsPage)
    expect(second).toBe(first)
  })
})
