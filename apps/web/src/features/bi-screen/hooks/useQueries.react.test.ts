import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import type { Environment } from '../components/environment'
import { dashboardFixture } from '../utils/testFixtures'
import type { RouteState } from '../types'
import { dashboardQueryKey, useDashboardQuery } from './useQueries'

describe('BI screen query ownership', () => {
  it('uses the canonical workspace-scoped biScreen namespace', () => {
    expect(dashboardQueryKey({ workspaceId: 7, sessionRevision: 3 }, {
      scope: { projectSetIds: [1], projectIds: [11], requirementIds: [] },
      period: 'week',
      tab: 'overview',
      teamDimension: 'version',
		teamView: 'data',
      invalidScope: false,
    })).toEqual([
      'pm', 'workspace', 7, 'biScreen', 'dashboard', 3, [1], [11], [], 'week', 'version', undefined,
    ])
  })

  it('does not present the previous version while the selected version is loading', async () => {
    let resolveSecond: ((value: ReturnType<typeof dashboardFixture>) => void) | undefined
    const dashboard = vi.fn()
      .mockResolvedValueOnce(dashboardFixture({
        team_data: { ...dashboardFixture().team_data, dimension_key: '11' },
      }))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveSecond = resolve }))
    const environment = {
      workspaceId: 7,
      sessionRevision: 3,
      pathname: '/workspace/bi-screen',
      search: {},
      allowed: true,
      navigate: vi.fn(),
      client: { dashboard },
    } as unknown as Environment
    const baseRoute: RouteState = {
      scope: { projectSetIds: [], projectIds: [], requirementIds: [] },
      period: 'week', tab: 'team', teamDimension: 'version', teamDimensionKey: '11',
      teamView: 'data', invalidScope: false,
    }
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = ({ children }: { children: ReactNode }) => (
      createElement(QueryClientProvider, { client: queryClient }, children)
    )
    const { result, rerender } = renderHook(
      ({ route }) => useDashboardQuery(environment, route, true),
      { wrapper, initialProps: { route: baseRoute } },
    )
    await waitFor(() => expect(result.current.data?.team_data.dimension_key).toBe('11'))

    rerender({ route: { ...baseRoute, teamDimensionKey: '12' } })
    await waitFor(() => expect(dashboard).toHaveBeenCalledTimes(2))
    expect(result.current.data).toBeUndefined()
    expect(result.current.isPending).toBe(true)

    resolveSecond?.(dashboardFixture({
      team_data: { ...dashboardFixture().team_data, dimension_key: '12' },
    }))
    await waitFor(() => expect(result.current.data?.team_data.dimension_key).toBe('12'))
  })
})
