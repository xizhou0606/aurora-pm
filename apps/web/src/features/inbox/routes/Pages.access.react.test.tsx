import { fireEvent, screen, within } from '@testing-library/react'
import { describe, expect, it, vi, type Mock } from 'bun:test'

import { INBOX_COPY } from '@/shared/copy'
import { defaultClient, inboxEnvironment, renderInbox } from '../utils/testKit'

describe('Inbox access and isolation', () => {
  it('fails closed before any business request when route permission is unresolved', () => {
    const client = defaultClient()
    renderInbox(inboxEnvironment({ client, allowed: false, canReadBugs: false }))

    expect(screen.getByRole('alert').textContent).toContain(
      INBOX_COPY.page.unavailableTitle,
    )
    expect(client.request).not.toHaveBeenCalled()
    expect(client.requestPage).not.toHaveBeenCalled()
  })

  it('fails closed before any business request when the inbox action is denied', () => {
    const client = defaultClient()
    renderInbox(inboxEnvironment({
      client,
      operationAccess: {
        status: 'ready',
        retry: vi.fn(),
        viewTodos: false,
      },
    }))

    expect(screen.getByRole('alert').textContent).toContain(
      INBOX_COPY.page.unavailableTitle,
    )
    expect(client.request).not.toHaveBeenCalled()
    expect(client.requestPage).not.toHaveBeenCalled()
  })

  it('gates bugs only with testing-board permission and keeps mentions out of summary total', async () => {
    const client = defaultClient()
    const { queryClient } = renderInbox(inboxEnvironment({
      client,
      canReadBugs: false,
    }))

    expect(await screen.findByText('修复登录流程')).not.toBeNull()
    expect(screen.queryByText('请关注发布进度')).toBeNull()
    expect(screen.queryByText(INBOX_COPY.feed.bug.title)).toBeNull()
    expect(screen.getByText(INBOX_COPY.filter.resultCount(7))).not.toBeNull()
    expect(document.querySelector('label[for="inbox-search"]')?.classList.contains('sr-only')).toBe(true)

    const requests = ((client.request) as unknown as Mock<typeof client.request>).mock.calls.map(([request]) => request)
    const pages = ((client.requestPage) as unknown as Mock<typeof client.requestPage>).mock.calls.map(([request]) => request)
    expect(requests.find((request) => request.path === '/workbench/todos/summary')?.query)
      .toMatchObject({ domains: 'task,workflow', include_mentions: 0 })
    expect(pages.some((request) => request.query?.type === 'bug')).toBe(false)
    expect(pages.some((request) => request.query?.type === 'mention')).toBe(false)

    for (const query of queryClient.getQueryCache().getAll().filter(query => query.queryKey[3] === 'inbox')) {
      expect(query.queryKey.slice(0, 4)).toEqual(['pm', 'workspace', 7, 'inbox'])
      const options = query.options as unknown as Record<string, unknown>
      expect(options.staleTime).toBe(30_000)
      expect(options.refetchInterval).toBe(false)
      expect(options.refetchOnMount).toBe(true)
      expect(options.refetchOnWindowFocus).toBe(true)
      expect(options.retry).toEqual(expect.any(Function))
      expect(options.retryDelay).toEqual(expect.any(Function))
    }
  })

  it('validates selected scope membership before issuing summary or feed reads', async () => {
    const client = defaultClient()
    const environment = inboxEnvironment({
      client,
      search: { project_set_ids: '99' },
    })
    renderInbox(environment)

    expect((await screen.findByRole('alert')).textContent).toContain(
      INBOX_COPY.page.invalidScopeTitle,
    )
    expect(((client.request) as unknown as Mock<typeof client.request>).mock.calls.map(([request]) => request.path))
      .toEqual(['/project-requirement-scope-options'])
    expect(client.requestPage).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', {
      name: INBOX_COPY.page.resetInvalidScope,
    }))
    expect(environment.navigate).toHaveBeenCalledWith('/workspace/inbox', true)
  })

  it('keeps legacy-only scope fail-closed and recovers through canonical navigation', () => {
    const client = defaultClient()
    const environment = inboxEnvironment({ client, search: { team_id: '7' } })
    renderInbox(environment)

    expect(screen.getByRole('alert').textContent).toContain(
      INBOX_COPY.page.invalidScopeTitle,
    )
    expect(client.request).not.toHaveBeenCalled()
    expect(client.requestPage).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', {
      name: INBOX_COPY.page.resetInvalidScope,
    }))
    expect(environment.navigate).toHaveBeenCalledWith('/workspace/inbox', true)
  })

  it('renders native content without creating a nested main or iframe', async () => {
    renderInbox(inboxEnvironment())
    expect(await screen.findByTestId('inbox-native-module')).not.toBeNull()
    expect(document.querySelectorAll('main')).toHaveLength(0)
    expect(document.querySelectorAll('iframe')).toHaveLength(0)
    expect(document.querySelectorAll('[data-slot="operational-page"]')).toHaveLength(1)
    const calendar = await screen.findByTestId('inbox-calendar')
    expect(within(calendar).getByRole('grid')).not.toBeNull()
  })
})
