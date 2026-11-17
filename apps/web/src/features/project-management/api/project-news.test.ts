import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const platformApiGet = (() => vi.fn())()

mock.module('@/shared/api', () => ({
  platformApi: { get: platformApiGet },
}))

import { listScrumProjectNewsNew } from './project-sets'

describe('project news API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the existing project news endpoint and rejects out-of-scope rows', async () => {
    const signal = new AbortController().signal
    platformApiGet.mockResolvedValueOnce({
      data: {
        data: [
          {
            id: 3,
            workspace_id: 7,
            project_set_id: 9,
            plan_unit_id: 0,
            user_id: 12,
            title: '发布候选版本完成',
            content: '测试已通过。',
            created_at: '2026-08-10T08:00:00Z',
            updated_at: '2026-08-10T08:00:00Z',
          },
          { id: 4, project_set_id: 10, title: '其他项目' },
          { id: 0, project_set_id: 9, title: '无效记录' },
        ],
      },
    })

    await expect(listScrumProjectNewsNew(9, signal)).resolves.toEqual([
      expect.objectContaining({ id: 3, title: '发布候选版本完成' }),
    ])
    expect(platformApiGet).toHaveBeenCalledWith('/project-sets/9/news', {
      params: { limit: '1' },
      signal,
    })
  })

  it('sanitizes transport errors before they reach React Query', async () => {
    platformApiGet.mockRejectedValueOnce({
      config: { headers: { Authorization: 'Bearer private-token' } },
      response: { status: 503, data: { error: 'private upstream details' } },
    })

    const error = await listScrumProjectNewsNew(9).catch((reason) => reason)

    expect(error).toMatchObject({
      name: 'PMSafeApiError',
      code: 'PROJECT_NEWS_FAILED',
      status: 503,
    })
    expect(JSON.stringify(error)).not.toContain('private-token')
  })
})
