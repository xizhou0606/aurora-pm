import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'

import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'
import { applyEmbeddedSession } from '@/shared/runtime'
import type { ScmAccessibleRepoDTO, ScmConnectionDTO } from '../api/contracts'
import { scmConfigKeys } from '../hooks'
import { RepositoryEditor } from './RepositoryEditor'

const connection: ScmConnectionDTO = {
  id: 1, name: 'GitLab', provider: 'gitlab', enabled: true,
  token_masked: '', version: 1, updated_by: 1, created_at: '', updated_at: '',
}
const clients: QueryClient[] = []
afterEach(() => { cleanup(); clients.splice(0).forEach((client) => client.clear()) })

function renderEditor(repos: ScmAccessibleRepoDTO[]) {
  applyEmbeddedSession({ token: 'test-token', workspaceId: 8, timezone: 'Asia/Shanghai' })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  clients.push(client)
  client.setQueryData([...scmConfigKeys(8).connections(), 'accessible-repos', 1], { repos, has_more: false })
  const onOpenChange = vi.fn()
  render(<QueryClientProvider client={client}>
    <RepositoryEditor open editing={null} connections={[connection]} onOpenChange={onOpenChange} onError={vi.fn()} />
  </QueryClientProvider>)
  return { onOpenChange }
}

it('applies a repository suggestion to the draft without closing or submitting the editor', async () => {
  const { onOpenChange } = renderEditor([{ external_id: '42', path: 'team/project', default_branch: 'develop' }])
  fireEvent.click(await screen.findByRole('button', { name: COPY.connections.pickRepos }))
  fireEvent.click(await screen.findByRole('button', { name: 'team/project' }))
  expect((screen.getByLabelText(COPY.repositories.fields.path) as HTMLInputElement).value).toBe('team/project')
  expect((screen.getByLabelText(COPY.repositories.fields.branch) as HTMLInputElement).value).toBe('develop')
  expect(screen.queryByRole('button', { name: 'team/project' })).toBeNull()
  expect(onOpenChange).not.toHaveBeenCalled()
})

it('shows the shared empty state when the connection has no repository suggestions', async () => {
  renderEditor([])
  fireEvent.click(await screen.findByRole('button', { name: COPY.connections.pickRepos }))
  expect(await screen.findByText(COPY.common.empty)).toBeTruthy()
  expect((screen.getByLabelText(COPY.repositories.fields.path) as HTMLInputElement).value).toBe('')
})
