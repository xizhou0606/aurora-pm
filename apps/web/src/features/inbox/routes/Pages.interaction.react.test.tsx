import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi, type Mock } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import { copy, INBOX_COPY } from '@/shared/copy'
import {
  defaultClient,
  inboxEnvironment,
  inboxTestKey,
  renderInbox,
  type InboxClient,
} from '../utils/testKit'
import {
  inboxInitialReadRetryDelay,
  shouldRetryInboxInitialRead,
} from '../hooks/useQueries'

describe('Inbox interactions', () => {
  it('retries only transient initialization reads within the bounded backoff', () => {
    for (const status of [401, 403, 408, 425, 429, 500, 503]) {
      expect(shouldRetryInboxInitialRead(
        0,
        new PMSafeApiError('TRANSIENT_READ', status),
      )).toBe(true)
    }
    expect(shouldRetryInboxInitialRead(
      0,
      new PMSafeApiError('NETWORK_UNAVAILABLE'),
    )).toBe(true)
    for (const status of [400, 404, 409, 422]) {
      expect(shouldRetryInboxInitialRead(
        0,
        new PMSafeApiError('DOMAIN_REJECTED', status),
      )).toBe(false)
    }
    expect(shouldRetryInboxInitialRead(
      0,
      new PMSafeApiError('INBOX_CONTRACT_INVALID', 502),
    )).toBe(false)
    expect(shouldRetryInboxInitialRead(
      0,
      new PMSafeApiError('REQUEST_ABORTED', undefined, true),
    )).toBe(false)
    expect(shouldRetryInboxInitialRead(
      2,
      new PMSafeApiError('SESSION_INITIALIZING', 403),
    )).toBe(false)
    expect([
      inboxInitialReadRetryDelay(0),
      inboxInitialReadRetryDelay(1),
    ]).toEqual([350, 700])
  })

  it('renders normalized task status copy with the canonical tone', async () => {
    renderInbox(inboxEnvironment())

    const status = await screen.findByText('进行中')
    expect(status.getAttribute('data-slot')).toBe('status-pill')
    expect(status.getAttribute('data-tone')).toBe('info')
  })

  it('defaults to the calendar and switches between the three todo views', async () => {
    renderInbox(inboxEnvironment())

    expect(await screen.findByTestId('inbox-calendar')).not.toBeNull()
    expect(await screen.findByText('项目集 A / 项目 A / 需求 A')).not.toBeNull()
    expect(screen.getByRole('tab', { name: INBOX_COPY.view.calendar })
      .getAttribute('aria-selected')).toBe('true')

    fireEvent.click(screen.getByRole('tab', { name: INBOX_COPY.view.board }))
    expect(await screen.findByTestId('inbox-board')).not.toBeNull()
    expect(screen.getByRole('tab', { name: INBOX_COPY.view.board })
      .getAttribute('aria-selected')).toBe('true')

    fireEvent.click(screen.getByRole('tab', { name: INBOX_COPY.view.list }))
    expect(await screen.findByRole('table', { name: INBOX_COPY.feed.workflow.title }))
      .not.toBeNull()
  })

  it('uses the shared scope cascader and clears descendants when the parent changes', async () => {
    const emptyEnvironment = inboxEnvironment()
    const emptyView = renderInbox(emptyEnvironment)

    expect(await screen.findByText(INBOX_COPY.scope.summaryAll)).not.toBeNull()
    expect(screen.getAllByRole('button', {
      name: INBOX_COPY.scope.label,
    })).toHaveLength(1)
    expect(screen.queryByRole('combobox', {
      name: INBOX_COPY.scope.project,
    })).toBeNull()

    emptyView.unmount()
    const environment = inboxEnvironment({
      search: {
        project_set_ids: '8',
        project_ids: '13',
        requirement_ids: '21',
      },
    })
    renderInbox(environment)

    await waitFor(() => expect((screen.getByRole('button', { name: INBOX_COPY.scope.label }) as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(screen.getByRole('button', { name: INBOX_COPY.scope.label }))
    fireEvent.click(await screen.findByRole('checkbox', { name: '项目集 A' }))

    await waitFor(() => expect(environment.navigate).toHaveBeenCalled())
    const [target] = ((environment.navigate) as unknown as Mock<typeof environment.navigate>).mock.lastCall ?? []
    expect(target).not.toContain('project_ids=')
    expect(target).not.toContain('requirement_ids=')
  })

  it('keeps unscoped todo feeds available when only the scope contract is invalid', async () => {
    const client = defaultClient()
    const baseRequest = client.request
    client.request = vi.fn(async (request) => {
      if (request.path === '/project-requirement-scope-options') {
        return {
          project_sets: [{ id: 8, name: '项目集 A' }],
          projects: [{ id: 13, project_new_id: 8, title: '项目 A' }],
          requirements: [{ id: 21, title: '未排期需求' }],
        } as never
      }
      return baseRequest(request)
    }) as InboxClient['request']

    const { onRuntimeFailure } = renderInbox(inboxEnvironment({ client }))

    expect(await screen.findByText(copy.ui.scope.error)).not.toBeNull()
    expect(await screen.findByText('修复登录流程')).not.toBeNull()
    expect(screen.getByText('产品评审')).not.toBeNull()
    expect(screen.getByText('登录错误')).not.toBeNull()
    expect(screen.queryByText('请关注发布进度')).toBeNull()
    expect(onRuntimeFailure).not.toHaveBeenCalled()
  })

  it('blocks every todo read when a selected scope cannot be validated', async () => {
    const client = defaultClient()
    client.request = vi.fn(async (request) => {
      if (request.path === '/project-requirement-scope-options') {
        return {
          project_sets: [{ id: 8, name: '项目集 A' }],
          projects: [{ id: 13, project_new_id: 8, title: '项目 A' }],
          requirements: [{ id: 21, title: '未排期需求' }],
        } as never
      }
      throw new Error(`unexpected request ${request.path}`)
    }) as InboxClient['request']

    const { onRuntimeFailure } = renderInbox(inboxEnvironment({
      client,
      search: { project_set_ids: '8' },
    }))

    expect(await screen.findByText(copy.ui.scope.error)).not.toBeNull()
    expect(((client.request) as unknown as Mock<typeof client.request>).mock.calls.map(([request]) => request.path))
      .toEqual(['/project-requirement-scope-options'])
    expect(client.requestPage).not.toHaveBeenCalled()
    expect(onRuntimeFailure).not.toHaveBeenCalled()
  })

  it('opens workflow, task and bug details through typed local routes', async () => {
    const environment = inboxEnvironment()
    renderInbox(environment)

    fireEvent.click(await screen.findByRole('button', {
      name: INBOX_COPY.feed.openDetail('产品评审'),
    }))
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/requirements/21?workflow_node=review',
    )

    fireEvent.click(await screen.findByRole('button', {
      name: INBOX_COPY.feed.openDetail('修复登录流程'),
    }))
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/work-items?task_id=31&from=inbox',
    )

    fireEvent.click(await screen.findByRole('button', {
      name: INBOX_COPY.feed.openDetail('登录错误'),
    }))
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?bug_id=41&from=inbox',
    )
  })

  it('pages task feeds without requesting the retired task mention feed', async () => {
    const environment = inboxEnvironment({ search: { q: '登录', bug_page: 3 } })
    renderInbox(environment)
    const task = await screen.findByRole('region', { name: INBOX_COPY.feed.task.title })
    await screen.findByText('修复登录流程')

    fireEvent.click(within(task).getByRole('button', { name: copy.ui.pagination.next }))
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/inbox?q=%E7%99%BB%E5%BD%95&task_page=2&bug_page=3',
      false,
    )
    expect(((environment.client.requestPage) as unknown as Mock<typeof environment.client.requestPage>).mock.calls.some(([request]) => request.query?.type === 'mention')).toBe(false)
  })

  it('debounces search for 500ms, supports Enter, and restores Router history state', async () => {
    const environment = inboxEnvironment()
    const first = renderInbox(environment)
    const input = await screen.findByLabelText(INBOX_COPY.filter.searchLabel)
    fireEvent.change(input, { target: { value: '发布' } })

    await new Promise((resolve) => window.setTimeout(resolve, 250))
    expect(environment.navigate).not.toHaveBeenCalled()
    await waitFor(() => expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/inbox?q=%E5%8F%91%E5%B8%83',
      true,
    ))

    ;((environment.navigate) as unknown as Mock<typeof environment.navigate>).mockClear()
    fireEvent.change(input, { target: { value: '阻塞' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/inbox?q=%E9%98%BB%E5%A1%9E',
      true,
    )

    first.unmount()
    renderInbox(inboxEnvironment({ search: { q: '返回后的筛选', task_page: 2 } }))
    expect((await screen.findByLabelText(
      INBOX_COPY.filter.searchLabel,
    ) as HTMLInputElement).value).toBe('返回后的筛选')
  })

  it('retries only the failed feed without hiding successful siblings', async () => {
    const client = defaultClient()
    const basePage = client.requestPage
    let taskAttempts = 0
    client.requestPage = vi.fn(async (request) => {
      if (request.query?.type === 'task') {
        taskAttempts += 1
        if (taskAttempts === 1) throw new PMSafeApiError('TASK_READ_FAILED', 422)
      }
      return basePage(request)
    }) as InboxClient['requestPage']
    renderInbox(inboxEnvironment({ client }))

    const task = await screen.findByRole('region', {
      name: INBOX_COPY.feed.task.title,
    })
    expect(await within(task).findByText(INBOX_COPY.feed.task.error)).not.toBeNull()
    expect(screen.getByText('登录错误')).not.toBeNull()
    expect(screen.queryByText('请关注发布进度')).toBeNull()
    expect(taskAttempts).toBe(1)

    fireEvent.click(within(task).getByRole('button', { name: INBOX_COPY.actions.retry }))
    expect(await screen.findByText('修复登录流程')).not.toBeNull()
    expect(taskAttempts).toBe(2)
  })

  it('recovers one canonical-session initialization failure without showing an error', async () => {
    const client = defaultClient()
    const basePage = client.requestPage
    let taskAttempts = 0
    client.requestPage = vi.fn(async (request) => {
      if (request.query?.type === 'task') {
        taskAttempts += 1
        if (taskAttempts === 1) {
          throw new PMSafeApiError('SESSION_INITIALIZING', 401)
        }
      }
      return basePage(request)
    }) as InboxClient['requestPage']

    renderInbox(inboxEnvironment({ client }))

    const task = await screen.findByRole('region', {
      name: INBOX_COPY.feed.task.title,
    })
    expect(await screen.findByText('修复登录流程')).not.toBeNull()
    expect(taskAttempts).toBe(2)
    expect(within(task).queryByText(INBOX_COPY.feed.task.error)).toBeNull()
  })

  it('keeps successful rows visible when a background refresh enters stale_error', async () => {
    const client = defaultClient()
    const basePage = client.requestPage
    let failTaskRefresh = false
    client.requestPage = vi.fn(async (request) => {
      if (request.query?.type === 'task' && failTaskRefresh) {
        throw new PMSafeApiError('TASK_REFRESH_FAILED', 422)
      }
      return basePage(request)
    }) as InboxClient['requestPage']
    const { queryClient } = renderInbox(inboxEnvironment({ client }))
    expect(await screen.findByText('修复登录流程')).not.toBeNull()

    failTaskRefresh = true
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: inboxTestKey(7, 'feed', 'task'),
      })
    })

    await waitFor(() => expect(
      ((client.requestPage) as unknown as Mock<typeof client.requestPage>).mock.calls.filter(
        ([request]) => request.query?.type === 'task',
      ),
    ).toHaveLength(2))
    expect(screen.getByText('修复登录流程')).not.toBeNull()
    expect(await screen.findByText(INBOX_COPY.page.staleError)).not.toBeNull()
  })
})
