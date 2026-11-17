import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'

import type { Environment } from '../components/environment'
import type { TeamMetricSettings } from '../types'
import {
  useTeamMetricSettingsMutation,
  useTeamMetricSettingsQuery,
} from './useTeamMetricSettings'

const settings: TeamMetricSettings = {
  version: 2,
  can_manage: true,
  configured: true,
  valid: true,
  requirement_workflow_template_id: 12,
  approval_node_id: 'review',
  options: [{
    template_id: 12,
    template_name: '需求评审',
    published_version: 3,
    approval_nodes: [{ node_id: 'review', node_name: '产品评审' }],
  }],
}

describe('BI team metric settings query ownership', () => {
  it('refreshes the active versioned settings after a save conflict', async () => {
    const teamMetricSettings = vi.fn().mockResolvedValue(settings)
    const putTeamMetricSettings = vi.fn().mockRejectedValue(
      new PMSafeApiError('BI_TEAM_METRIC_SETTINGS_VERSION_CONFLICT', 409),
    )
    const environment = {
      workspaceId: 7,
      sessionRevision: 3,
      allowed: true,
      client: { teamMetricSettings, putTeamMetricSettings },
    } as unknown as Environment
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const wrapper = ({ children }: { children: ReactNode }) => (
      createElement(QueryClientProvider, { client: queryClient }, children)
    )
    const { result } = renderHook(() => ({
      query: useTeamMetricSettingsQuery(environment, true),
      mutation: useTeamMetricSettingsMutation(environment),
    }), { wrapper })
    await waitFor(() => expect(result.current.query.data).toEqual(settings))

    act(() => result.current.mutation.mutate({
      expected_version: 2,
      requirement_workflow_template_id: 12,
      approval_node_id: 'review',
    }))

    await waitFor(() => expect(result.current.mutation.isError).toBe(true))
    await waitFor(() => expect(teamMetricSettings).toHaveBeenCalledTimes(2))
  })
})
