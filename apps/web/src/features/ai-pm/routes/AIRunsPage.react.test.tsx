import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { AIRunsPage } from './AIRunsPage'

const originalAdapter = platformApi.defaults.adapter
const response = (config: InternalAxiosRequestConfig, data: unknown) => ({ data, status: 200, statusText: 'OK', headers: {}, config })

describe('AIRunsPage', () => {
  beforeEach(() => runtimeWorkspaceStore.getState().selectWorkspace(7))

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    runtimeWorkspaceStore.getState().clearWorkspace()
  })

  it('shows a safe message instead of the provider error details', async () => {
    const internalError = 'AI runtime returned status 502: provider connection reset by peer'
    const failedRun = {
      id: 109, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
      subject_version: 1, status: 'failed', model: 'qwen-test', error_message: internalError,
      created_at: '2026-08-12T01:35:50.000Z', updated_at: '2026-08-12T01:40:00.000Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url === '/ai/runs/109') return response(config, { data: failedRun })
      if (config.url === '/ai/runs') return response(config, { data: [failedRun], total: 1 })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><AIRunsPage /></QueryClientProvider>)

    fireEvent.click(await screen.findByRole('row', { name: copy.aiPM.runs.openRow(109) }))

    expect(await screen.findByText(copy.aiPM.runs.errorProvider)).toBeTruthy()
    expect(screen.queryByText(internalError)).toBeNull()
  })
})
