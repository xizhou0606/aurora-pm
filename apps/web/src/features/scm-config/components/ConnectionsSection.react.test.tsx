import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { applyEmbeddedSession } from '@/shared/runtime'

import { ConnectionsSection } from './ConnectionsSection'

const realScmConfigClient = scmConfigClient

mock.module('../api', () => ({
  scmConfigClient: {
    ...realScmConfigClient,
      listConnections: vi.fn().mockResolvedValue([
        {
          id: 1,
          name: 'gitlab-main',
          provider: 'gitlab',
          base_url: 'https://gitlab.example.com',
          enabled: true,
          token_masked: '••••1234',
          version: 1,
          updated_by: 0,
          created_at: '2026-09-07T00:00:00Z',
          updated_at: '2026-09-07T00:00:00Z',
        },
      ]),
    testConnection: vi.fn().mockResolvedValue({ login: 'pm-bot', display_name: 'PM Bot' }),
  },
}))

vi.mock('../../hooks/useScmConfigAccess', () => ({
  useScmConfigAccess: () => ({
    status: 'ready',
    retry: () => undefined,
    canView: () => true,
    canManage: () => true,
  }),
}))

import { scmConfigClient } from '../api'

function renderSection() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <ConnectionsSection />
    </QueryClientProvider>,
  )
}

describe('ConnectionsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    applyEmbeddedSession({ token: 'test-token', workspaceId: 8, timezone: 'Asia/Shanghai' })
  })

  it('renders connections without ever exposing the raw token', async () => {
    renderSection()
    await waitFor(() => {
      expect(screen.getByText('gitlab-main')).toBeTruthy()
    })
    expect(screen.getByText('••••1234')).toBeTruthy()
    // 后端 DTO 根本不携带 token 字段；这里守住"界面只见掩码"的契约。
    expect(screen.queryByText('glpat-secret')).not.toBeTruthy()
  })

  it('tests a connection and surfaces the provider account', async () => {
    renderSection()
    await waitFor(() => {
      expect(screen.getByText('gitlab-main')).toBeTruthy()
    })
    fireEvent.click(screen.getByText('测试连接'))
    await waitFor(() => {
      expect(scmConfigClient.testConnection).toHaveBeenCalledWith(1)
      expect(screen.getByText('连接有效：PM Bot')).toBeTruthy()
    })
  })
})
