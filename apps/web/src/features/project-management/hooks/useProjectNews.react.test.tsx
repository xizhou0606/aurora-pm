import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { runtimeWorkspaceStore } from '@/shared/runtime'

const listProjectNews = (() => vi.fn())()

mock.module('../api', () => ({
  listScrumProjectNewsNew: listProjectNews,
}))

import {
  latestProjectNewsQueryOptions,
  useLatestProjectNewsQuery,
} from './useProjectNews'

describe('latest project news query', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    runtimeWorkspaceStore.getState().selectWorkspace(7)
    listProjectNews.mockResolvedValue([
      { id: 2, title: '最新更新' },
      { id: 1, title: '较早更新' },
    ])
  })

  it('isolates project news by workspace and selects the server-sorted first row', async () => {
    const options = latestProjectNewsQueryOptions(7, 9)
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    expect(options.queryKey).toEqual([
      'pm',
      'workspace',
      7,
      'projectSets',
      'project',
      9,
      'news',
      'latest',
    ] as never)
    expect(options.refetchOnMount).toBe(true)
    await expect(client.fetchQuery(options)).resolves.toEqual({
      id: 2,
      title: '最新更新',
    } as never)
    expect(listProjectNews).toHaveBeenCalledWith(9, expect.any(AbortSignal))
  })

  it('returns null for an empty project update list and rejects invalid IDs', async () => {
    listProjectNews.mockResolvedValueOnce([])
    const client = new QueryClient()

    await expect(client.fetchQuery(latestProjectNewsQueryOptions(7, 9)))
      .resolves.toBeNull()
    expect(() => latestProjectNewsQueryOptions(7, 0)).toThrow(/positive project ID/)
  })

  it('refreshes stale cached news when the project view remounts', async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const options = latestProjectNewsQueryOptions(7, 9)
    const cached = {
      id: 2,
      workspace_id: 7,
      project_set_id: 9,
      plan_unit_id: 0,
      user_id: 12,
      title: '缓存更新',
      content: '',
      created_at: '2026-08-10T08:00:00Z',
      updated_at: '2026-08-10T08:00:00Z',
    }
    client.setQueryData(options.queryKey, cached, { updatedAt: Date.now() })
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )

    const firstMount = renderHook(() => useLatestProjectNewsQuery(9), { wrapper })
    expect(firstMount.result.current.data).toEqual(cached)
    expect(listProjectNews).not.toHaveBeenCalled()
    firstMount.unmount()

    client.setQueryData(options.queryKey, cached, {
      updatedAt: Date.now() - 31_000,
    })
    const refreshed = {
      ...cached,
      id: 3,
      title: '刷新后的更新',
      created_at: '2026-08-10T09:00:00Z',
      updated_at: '2026-08-10T09:00:00Z',
    }
    listProjectNews.mockResolvedValueOnce([refreshed])
    const remount = renderHook(() => useLatestProjectNewsQuery(9), { wrapper })

    await waitFor(() => expect(listProjectNews).toHaveBeenCalledWith(
      9,
      expect.any(AbortSignal),
    ))
    await waitFor(() => expect(remount.result.current.data).toEqual(refreshed))
  })
})
