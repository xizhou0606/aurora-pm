import { beforeEach, expect, it, vi } from 'bun:test'
import { listInbox, markInboxRead } from './index'
const api = (() => ({ get: vi.fn(), post: vi.fn() }))()
vi.mock('@/shared/api', () => ({ platformApi: api, unwrapApiData: (value: unknown) => value, toSafeApiError: (error: unknown) => error }))
beforeEach(() => { vi.clearAllMocks() })
it('sends message filtering to the server and preserves the cursor watermark', async () => {
 const page = { items: [], unread_count: 8, unread_by_category: { message: 2 }, high_watermark: 19, next_cursor: 12 }
 api.get.mockResolvedValue(page)
 expect(await listInbox('message', 17)).toEqual(page)
 expect(api.get).toHaveBeenCalledWith('/notifications/inbox', { signal: undefined, params: { category: 'message', before: 17 } })
 await markInboxRead(19, true, 'message')
 expect(api.post).toHaveBeenCalledWith('/notifications/inbox/read-all', { through_id: 19, category: 'message' })
})
it('propagates failed read mutations instead of reporting them as success', async () => {
 api.post.mockRejectedValue(new Error('network unavailable'))
 await expect(markInboxRead(19, true, 'message')).rejects.toThrow('network unavailable')
})
