import { beforeEach, describe, expect, it, mock, vi } from 'bun:test'

const feature = (() => ({
  InboxPage: vi.fn(() => null),
}))()

mock.module('@/features/inbox', () => feature)

import {
  loadLocalInbox,
  resetLocalInboxLoaderForTests,
} from './local-inbox-loader'

describe('local Inbox loader', () => {
  beforeEach(() => {
    resetLocalInboxLoaderForTests()
  })

  it('loads and caches the PM-owned feature without a remote manifest', async () => {
    const first = await loadLocalInbox()
    const second = await loadLocalInbox()

    expect(first).toBe(feature.InboxPage)
    expect(second).toBe(first)
  })
})
