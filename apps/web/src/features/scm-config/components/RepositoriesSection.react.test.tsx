import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { applyEmbeddedSession } from '@/shared/runtime'

import { RepositoriesSection } from './RepositoriesSection'
import { scmConfigClient } from '../api'

const realScmConfigClient = scmConfigClient

mock.module('../api', () => ({
  scmConfigClient: {
    ...realScmConfigClient,
      listRepositories: vi.fn().mockResolvedValue([
        {
          id: 3,
          workspace_id: 8,
          connection_id: 1,
          external_id: '106',
          path: 'base/base-frontend',
          default_branch: 'main',
          filter_rules: {},
          project_set_id: null,
          enabled: true,
          version: 1,
          created_at: '2026-09-07T00:00:00Z',
          updated_at: '2026-09-07T00:00:00Z',
        },
      ]),
      listConnections: vi.fn().mockResolvedValue([
        {
          id: 1,
          name: 'fronted',
          provider: 'gitlab',
          base_url: 'https://gitlab.example.com',
          enabled: true,
          token_masked: '••••1gyb',
          version: 1,
          updated_by: 0,
          created_at: '2026-09-07T00:00:00Z',
          updated_at: '2026-09-07T00:00:00Z',
        },
      ]),
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

describe('RepositoriesSection 仓库绑定', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    applyEmbeddedSession({ token: 'test-token', workspaceId: 8, timezone: 'Asia/Shanghai' })
  })

  it('展示已绑定仓库与采集节奏说明', async () => {
    renderSection()
    await waitFor(() => {
      expect(screen.getByText('base/base-frontend')).toBeTruthy()
    })
    // 采集节奏 = 接入全量 + 按「统计口径」里可配置的同步时机定时同步自然窗。
    expect(screen.getByText(/按「统计口径」的同步时机定时同步对应自然窗/)).toBeTruthy()
    expect(screen.queryByText('立即采集')).toBeNull()
    expect(screen.queryByText('回填采集')).toBeNull()
  })
})

function renderSection() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <RepositoriesSection />
    </QueryClientProvider>,
  )
}
