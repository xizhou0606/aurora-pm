import { selectCalendarDate } from '@/shared/testing/select-calendar-date'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, waitFor } from '@testing-library/dom'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi, mock } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { PMTransportRequest, PMTransportUploadRequest } from '../api/client'
import type { WorkItemsClient } from '../api/client'
import { WorkItemsEnvironmentProvider } from '../hooks/useEnvironment'
import {
  shouldRetryWorkItemDetailRead,
  workItemDetailRetryDelay,
} from '../hooks/detail/useQueries'
import type { WorkItemsEnvironment, WorkItemsOperationAccess } from '../model/environment'
import { WorkItemsPage } from './Page'

mock.module('@/shared/ui', () => ({
  LazyRichTextField: ({
    label, value, serialization, visuallyHiddenLabel, readOnly, onChange, onPasteFiles,
  }: {
    label: string
    value: string
    serialization?: string
    visuallyHiddenLabel?: boolean
    readOnly?: boolean
    onChange?: (value: string) => void
    onPasteFiles?: (files: readonly File[]) => Promise<unknown>
  }) => (
    <div>
      <span className={visuallyHiddenLabel ? 'sr-only' : undefined}>{label}</span>
      <textarea
        aria-label={label}
        data-serialization={serialization}
        data-upload-enabled={onPasteFiles ? 'true' : 'false'}
        readOnly={readOnly}
        value={value}
        onChange={(event) => onChange?.(event.currentTarget.value)}
      />
    </div>
  ),
}))

let root: Root | undefined
let container: HTMLDivElement | undefined
;(globalThis as typeof globalThis & { React?: typeof React; IS_REACT_ACT_ENVIRONMENT?: boolean }).React = React
;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

afterEach(async () => {
  if (root) await act(async () => root?.unmount())
  container?.remove()
  root = undefined
  container = undefined
})

function task() {
  return {
    id: 31,
    workspace_id: 7,
    team_id: 4,
    project_set_id: 8,
    project_id: 13,
    requirement_id: 21,
    task_key: 'PM-31',
    title: 'PM native task',
    kind: 'task',
    status: 'draft',
    priority: 'normal',
    can_update: true,
    can_delete: true,
    can_comment: true,
    allowed_status_transitions: ['draft', 'in_dev'],
    watched: false,
  }
}

function makeEnvironment(input: {
  allowed?: boolean
  createAllowed?: boolean
  pathname?: string
  search?: Readonly<Record<string, unknown>>
  request?: (entry: PMTransportRequest) => Promise<unknown>
  requestPage?: (entry: PMTransportRequest) => Promise<{ items: unknown[]; total: number }>
  upload?: (entry: PMTransportUploadRequest) => Promise<unknown>
  createResponse?: unknown
  batchResponse?: unknown
  createError?: boolean
  scopePayload?: unknown
  usersPayload?: unknown
  taskPayload?: Record<string, unknown>
  relatedPayload?: unknown
  operationAccess?: Partial<WorkItemsOperationAccess>
} = {}) {
  const currentTask = { ...task(), ...(input.taskPayload || {}) }
  const request = vi.fn(input.request || (async (entry: PMTransportRequest) => {
    if (entry.path === '/tasks/batch-create' && entry.method === 'POST') {
      return input.batchResponse ?? { created: [], total: 0 }
    }
    if (entry.path === '/tasks' && entry.method === 'POST') {
      if (input.createError) throw new Error('create denied')
      return input.createResponse
    }
    if (entry.path === '/tasks/31' && entry.method === 'PATCH') {
      return { ...currentTask, ...(entry.body || {}) }
    }
    if (entry.path === '/tasks/31') return currentTask
    if (entry.path === '/tasks/31/related') return input.relatedPayload ?? { items: [] }
    if (entry.path === '/tasks/31/git-links') return []
    if (entry.path === '/issue-statuses') {
      return [{ status_key: 'draft', label: '待处理', sort_order: 10 }, { status_key: 'in_dev', label: '进行中', sort_order: 20 }]
    }
    if (entry.path === '/issue-workflow-transitions') {
      return { enforce: false, transitions: [{ from_status: 'draft', to_status: 'in_dev' }] }
    }
    return undefined
  }))
  const requestPage = vi.fn(input.requestPage || (async (entry: PMTransportRequest) => {
    if (entry.path === '/tasks') return { items: [currentTask], total: 1 }
    if (entry.path.endsWith('/activities')) return { items: [], total: 0 }
    if (entry.path.endsWith('/comments')) return { items: [], total: 0 }
    if (entry.path.endsWith('/attachments')) return { items: [], total: 0 }
    return { items: [], total: 0 }
  }))
  const upload = vi.fn(input.upload || (async () => ({ id: 91 })))
  const client: WorkItemsClient = {
    request: async <T,>(entry: PMTransportRequest) => request(entry) as Promise<T>,
    requestPage: async <T,>(entry: PMTransportRequest) => requestPage(entry) as Promise<{ items: T[]; total: number }>,
    requestPMScopeOptions: async <T,>() => ({
      project_sets: [{ id: 8, name: 'Project set' }],
      projects: [{ id: 13, project_new_id: 8, title: 'Project' }],
      requirements: [{ id: 21, version_task_new_id: 13, title: 'Requirement' }],
    } as unknown as T),
    requestPMUsers: async <T,>() => (input.usersPayload ?? []) as T,
    upload: async <T,>(entry: PMTransportUploadRequest) => upload(entry) as Promise<T>,
  }
  if (input.scopePayload !== undefined) {
    client.requestPMScopeOptions = async <T,>() => input.scopePayload as T
  }
  return {
    environment: {
      client,
      workspaceId: 7,
      userId: 101,
      sessionRevision: 1,
      pathname: input.pathname || '/workspace/work-items',
      search: input.search || { view: 'list' },
      allowed: input.allowed !== false,
      createAllowed: input.allowed !== false && input.createAllowed !== false,
      operationAccess: {
        status: 'ready', retry: vi.fn(), viewReferenceData: true,
        viewTaskTable: true, viewTaskDetail: true, createTask: true,
        updateTask: true, transitionTask: true, deleteTask: true, viewAttachments: true,
        uploadAttachment: true, downloadAttachment: true, deleteAttachment: true,
        viewComments: true, createComment: true, deleteComment: true,
        updateWatching: true,
        ...input.operationAccess,
      },
      navigate: vi.fn().mockResolvedValue(undefined),
    } satisfies WorkItemsEnvironment,
    request,
    requestPage,
    upload,
  }
}

async function render(environment: WorkItemsEnvironment) {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  await act(async () => root?.render(
    <QueryClientProvider client={queryClient}>
      <WorkItemsEnvironmentProvider value={environment}>
        <WorkItemsPage />
      </WorkItemsEnvironmentProvider>
    </QueryClientProvider>,
  ))
  return container
}

describe('Work-items PM API page boundary', () => {
  it.each([403, 404])('shows project application guidance for unavailable task (%s)', async (status) => {
    const { environment } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      request: async () => { throw new PMSafeApiError(status === 403 ? 'FORBIDDEN' : 'NOT_FOUND', status) },
    })
    await render(environment)
    await waitFor(() => expect(document.body.textContent).toContain(RESOURCE_ACCESS_COPY.projectGuide.request))
    expect(document.querySelector('[role="dialog"]')?.textContent).not.toContain('PM native task')
  })

  it('limits detail read retries to transient errors and the initialization window', () => {
    for (const status of [401, 403, 408, 425, 429, 500, 503]) {
      expect(shouldRetryWorkItemDetailRead(
        0,
        new PMSafeApiError('TRANSIENT_DETAIL_READ', status),
      )).toBe(true)
    }
    expect(shouldRetryWorkItemDetailRead(
      0,
      new PMSafeApiError('NETWORK_UNAVAILABLE'),
    )).toBe(true)
    for (const status of [400, 404, 409, 422]) {
      expect(shouldRetryWorkItemDetailRead(
        0,
        new PMSafeApiError('DOMAIN_REJECTED', status),
      )).toBe(false)
    }
    expect(shouldRetryWorkItemDetailRead(
      0,
      new PMSafeApiError('DETAIL_CONTRACT_INVALID', 502),
    )).toBe(false)
    expect(shouldRetryWorkItemDetailRead(
      0,
      new PMSafeApiError('REQUEST_ABORTED', undefined, true),
    )).toBe(false)
    expect(shouldRetryWorkItemDetailRead(
      2,
      new PMSafeApiError('WORKSPACE_INITIALIZING', 403),
    )).toBe(false)
    expect([
      workItemDetailRetryDelay(0),
      workItemDetailRetryDelay(1),
    ]).toEqual([350, 700])
  })

  it('fails permission closed before issuing business requests', async () => {
    const { environment, request, requestPage } = makeEnvironment({ allowed: false })
    const view = await render(environment)
    expect(view.querySelector('[role="alert"]')).not.toBeNull()
    expect(request).not.toHaveBeenCalled()
    expect(requestPage).not.toHaveBeenCalled()
  })

  it('fails closed when the task-table action is not granted', async () => {
    const { environment, request, requestPage } = makeEnvironment({
      operationAccess: { viewTaskTable: false },
    })
    const view = await render(environment)
    expect(view.textContent).toContain(WORK_ITEMS_COPY.page.unavailableDescription)
    expect(request).not.toHaveBeenCalled()
    expect(requestPage).not.toHaveBeenCalled()
  })

  it('renders the native list from the sole PM /tasks endpoint', async () => {
    const { environment, requestPage } = makeEnvironment()
    const view = await render(environment)
    await waitFor(() => expect(view.textContent).toContain('PM native task'))
    expect([...view.querySelectorAll('[role="tab"]')].slice(0, 2).map((tab) => tab.textContent))
      .toEqual([WORK_ITEMS_COPY.views.list, WORK_ITEMS_COPY.views.board])
    expect(requestPage).toHaveBeenCalledWith(expect.objectContaining({
      path: '/tasks',
      query: expect.objectContaining({ limit: 50, offset: 0, sort: 'created_at_desc' }),
    }))
    expect(requestPage.mock.calls.flatMap(([entry]) => Object.keys(entry.query || {})))
      .not.toContain('authority')
  })

  it('hides detail and mutation intents when their global actions are not granted', async () => {
    const { environment } = makeEnvironment({
      operationAccess: {
        viewTaskDetail: false,
        updateTask: false,
        deleteTask: false,
      },
    })
    const view = await render(environment)
    await waitFor(() => expect(view.textContent).toContain('PM native task'))

    expect(view.querySelector('[aria-label="查看PM native task详情"]')).toBeNull()
    expect(view.querySelector('[aria-label="编辑"]')).toBeNull()
    expect(view.querySelector('[aria-label="删除"]')).toBeNull()
  })

  it('keeps the collection total only in the bottom pagination', async () => {
    const { environment } = makeEnvironment({
      requestPage: async (entry) => entry.path === '/tasks'
        ? { items: [task()], total: 61 }
        : { items: [], total: 0 },
    })
    const view = await render(environment)
    await waitFor(() => expect(view.textContent).toContain('PM native task'))

    expect(view.textContent).not.toContain(WORK_ITEMS_COPY.filter.resultCount(61))
    expect(view.querySelector('[data-slot="pagination-total"]')?.textContent).toContain('61')
  })

  it('loads Gantt task pages incrementally instead of blocking the first render on all rows', async () => {
    const offsets: number[] = []
    const { environment } = makeEnvironment({
      search: { view: 'list', layout: 'gantt' },
      requestPage: async (entry) => {
        if (entry.path !== '/tasks') return { items: [], total: 0 }
        const offset = Number(entry.query?.offset || 0)
        offsets.push(offset)
        return {
          items: [{ ...task(), id: 31 + offset, task_key: `PM-${31 + offset}` }],
          total: 2,
        }
      },
    })
    const view = await render(environment)
    await waitFor(() => expect(view.textContent).toContain(WORK_ITEMS_COPY.gantt.loadMore))

    expect(offsets).toEqual([0])
    const loadMore = [...view.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.gantt.loadMore)
    await act(async () => loadMore?.click())
    await waitFor(() => expect(offsets).toEqual([0, 1]))
  })

  it('does not request the all-workspace board before a project/version scope is selected', async () => {
    const { environment, request, requestPage } = makeEnvironment({ search: { view: 'board' } })
    const view = await render(environment)
    await waitFor(() => expect(view.textContent).toContain(
      WORK_ITEMS_COPY.scope.boardRequiredDescription,
    ))

    expect(requestPage.mock.calls.some(([entry]) => entry.path === '/tasks')).toBe(false)
    expect(request.mock.calls.some(([entry]) => entry.path === '/issue-statuses')).toBe(false)
  })

  it('loads only main-backed detail resources and hides unavailable domains', async () => {
    const { environment, request, requestPage } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      search: { view: 'list' },
    })
    await render(environment)
    await act(async () => waitFor(() => expect(
      request.mock.calls.some(([entry]) => entry.path === '/tasks/31/related'),
    ).toBe(true)))
    const paths = [
      ...request.mock.calls.map(([entry]) => entry.path),
      ...requestPage.mock.calls.map(([entry]) => entry.path),
    ]
    expect(paths).toEqual(expect.arrayContaining([
      '/tasks/31', '/tasks/31/activities', '/tasks/31/comments',
      '/tasks/31/attachments', '/tasks/31/related', '/tasks/31/git-links',
    ]))
    expect(paths.some((path) => path.includes('/links') || path.includes('time-logs')
      || path.includes('create-options') || path.includes('test-'))).toBe(false)
  })

  it('recovers one initialization failure for comments and children independently', async () => {
    let commentAttempts = 0
    let childAttempts = 0
    const { environment } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      request: async (entry) => {
        if (entry.path === '/tasks/31') return task()
        if (entry.path === '/tasks/31/related') {
          childAttempts += 1
          if (childAttempts === 1) {
            throw new PMSafeApiError('WORKSPACE_INITIALIZING', 403)
          }
          return {
            items: [{
              relation: 'child',
              task: {
                ...task(),
                id: 32,
                task_key: 'PM-32',
                title: 'Recovered child',
              },
            }],
          }
        }
        if (entry.path === '/tasks/31/git-links') return []
        if (entry.path === '/issue-statuses') return []
        if (entry.path === '/issue-workflow-transitions') {
          return { enforce: false, transitions: [] }
        }
        return undefined
      },
      requestPage: async (entry) => {
        if (entry.path === '/tasks') return { items: [task()], total: 1 }
        if (entry.path === '/tasks/31/comments') {
          commentAttempts += 1
          if (commentAttempts === 1) {
            throw new PMSafeApiError('SESSION_INITIALIZING', 401)
          }
          return {
            items: [{ id: 91, body: 'Recovered comment', author_name: 'Teo' }],
            total: 1,
          }
        }
        return { items: [], total: 0 }
      },
    })

    await render(environment)

    await act(async () => waitFor(() => {
      expect(commentAttempts).toBe(2)
      expect(childAttempts).toBe(2)
      expect(document.body.textContent).toContain('Recovered comment')
      expect(document.body.textContent).toContain('Recovered child')
    }))
    expect(document.querySelector('section[aria-labelledby="work-items-detail-comments"] [role="alert"]')).toBeNull()
    expect(document.querySelector('#work-items-detail-children [role="alert"]')).toBeNull()
  })

  it('surfaces a generic retry state after a persistent child read failure', async () => {
    let childAttempts = 0
    const { environment } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      request: async (entry) => {
        if (entry.path === '/tasks/31') return task()
        if (entry.path === '/tasks/31/related') {
          childAttempts += 1
          throw new PMSafeApiError('CHILD_READ_DENIED', 403)
        }
        if (entry.path === '/tasks/31/git-links') return []
        if (entry.path === '/issue-statuses') return []
        if (entry.path === '/issue-workflow-transitions') {
          return { enforce: false, transitions: [] }
        }
        return undefined
      },
      requestPage: async (entry) => entry.path === '/tasks'
        ? { items: [task()], total: 1 }
        : { items: [], total: 0 },
    })

    await render(environment)

    await act(async () => waitFor(() => expect(childAttempts).toBe(3), {
      timeout: 2_000,
    }))
    await waitFor(() => {
      expect(document.querySelector('#work-items-detail-children [role="alert"]')).not.toBeNull()
    })
  })

  it('renders a single description title and reads Tiptap JSON through the JSON adapter', async () => {
    const description = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Rich task goal' }] }],
    })
    const { environment } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      taskPayload: { description },
    })
    await render(environment)
    const editor = await waitFor(() => {
      const value = document.querySelector('[aria-label="描述"]') as HTMLTextAreaElement | null
      expect(value).not.toBeNull()
      return value!
    })

    expect(editor.value).toBe(description)
    expect(editor.dataset.serialization).toBe('json')
    const visibleDescriptionLabels = [...document.querySelectorAll('span, h1, h2, h3, h4')]
      .filter((entry) => entry.textContent === WORK_ITEMS_COPY.detail.descriptionLabel
        && !entry.classList.contains('sr-only'))
    expect(visibleDescriptionLabels).toHaveLength(1)
  })

  it('renders children as one standard table with status, user and icon-only actions', async () => {
    const { environment } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      usersPayload: [{ id: 9, display_name: 'Teo' }],
      relatedPayload: { items: [{ relation: 'child', task: {
        id: 32, task_key: 'PM-32', title: 'Child row', priority: 'high',
        assignee_id: 9, assignee_display_name: 'Teo', status: 'in_dev', can_delete: true,
      } }] },
    })
    await render(environment)
    await waitFor(() => expect(document.body.textContent).toContain('Child row'))
    const table = document.querySelector(`[aria-label="${WORK_ITEMS_COPY.detail.children}"]`)
    expect(table).not.toBeNull()
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.taskName)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.taskNumber)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.priority)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.assignee)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.status)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.estimatedTime)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.startsAt)
    expect(table?.textContent).toContain(WORK_ITEMS_COPY.list.due)
    expect(table?.textContent).toContain('Teo')
    expect(table?.querySelector('[data-slot="table-action-button"]')).not.toBeNull()
  })

  it('owns one explicit detail save and destructive actions in the detail panel footer', async () => {
    const { environment, request, requestPage } = makeEnvironment({
      pathname: '/workspace/work-items/31',
      taskPayload: { description: 'Original goal' },
    })
    await render(environment)
    const editor = await waitFor(() => {
      const value = document.querySelector('[aria-label="描述"]') as HTMLTextAreaElement | null
      expect(value).not.toBeNull()
      return value!
    })
    const footer = document.querySelector('[data-slot="responsive-panel-footer"]') as HTMLElement
    expect(footer).not.toBeNull()
    expect(footer.textContent).toContain(WORK_ITEMS_COPY.detail.delete)
    expect(footer.textContent).toContain(WORK_ITEMS_COPY.mutation.edit)
    expect(footer.textContent).not.toContain(WORK_ITEMS_COPY.detail.save)
    expect(footer.textContent).not.toContain(WORK_ITEMS_COPY.detail.saveDescription)
    expect(document.body.textContent).not.toContain(WORK_ITEMS_COPY.detail.watch)
    expect(document.body.textContent).not.toContain(WORK_ITEMS_COPY.detail.allowedTransitions)

    expect(editor.readOnly).toBe(true)
    const edit = [...footer.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.mutation.edit) as HTMLButtonElement
    fireEvent.click(edit)
    await waitFor(() => expect(editor.readOnly).toBe(false))
    fireEvent.change(editor, { target: { value: 'Updated goal' } })
    const save = [...footer.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.detail.save) as HTMLButtonElement
    expect(save.disabled).toBe(false)
    const collectionReadsBeforeSave = requestPage.mock.calls.filter(([entry]) => (
      entry.path === '/tasks'
    )).length
    await act(async () => save.click())

    await waitFor(() => expect(request).toHaveBeenCalledWith({
      method: 'PATCH',
      path: '/tasks/31',
      body: { description: 'Updated goal' },
    }))
    await waitFor(() => expect(requestPage.mock.calls.filter(([entry]) => (
      entry.path === '/tasks'
    )).length).toBeGreaterThan(collectionReadsBeforeSave))
  })

  it('creates multiple children through controlled PM task writes and refreshes related rows once', async () => {
    const child = { ...task(), id: 32, title: 'Inherited child', kind: 'subtask', parent_id: 31 }
    const { environment, request } = makeEnvironment({
      pathname: '/workspace/work-items/31', createResponse: child,
    })
    await render(environment)
    const trigger = await waitFor(() => {
      const value = [...document.querySelectorAll('button')]
        .find((entry) => entry.textContent?.includes(WORK_ITEMS_COPY.detail.createChildren))
      expect(value).toBeTruthy()
      return value as HTMLButtonElement
    })
    await act(async () => trigger.click())
    const first = await waitFor(() => document.querySelector(
      `[aria-label="${WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childName, 1)}"]`,
    ) as HTMLInputElement)
    fireEvent.change(first, { target: { value: 'Inherited child' } })
    const add = [...document.querySelectorAll('button')]
      .find((entry) => entry.textContent?.includes(WORK_ITEMS_COPY.detail.addChildRow))!
    await act(async () => add.click())
    const second = document.querySelector(
      `[aria-label="${WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childName, 2)}"]`,
    ) as HTMLInputElement
    fireEvent.change(second, { target: { value: 'Second child' } })
    const dialog = [...document.querySelectorAll('[role="dialog"]')].at(-1) as HTMLElement
    const submit = [...dialog.querySelectorAll('button')]
      .find((entry) => entry.textContent?.includes(WORK_ITEMS_COPY.detail.createChildren))!
    const relatedBefore = request.mock.calls.filter(([entry]) => entry.path === '/tasks/31/related').length
    await act(async () => submit.click())

    await waitFor(() => expect(request.mock.calls.filter(([entry]) => (
      entry.method === 'POST' && entry.path === '/tasks'
    ))).toHaveLength(2))
    expect(request).toHaveBeenCalledWith({
      method: 'POST', path: '/tasks',
      body: { title: 'Inherited child', parent_id: 31, kind: 'subtask' },
    })
    expect(request).toHaveBeenCalledWith({
      method: 'POST', path: '/tasks',
      body: { title: 'Second child', parent_id: 31, kind: 'subtask' },
    })
    await waitFor(() => expect(request.mock.calls.filter(([entry]) => (
      entry.path === '/tasks/31/related'
    )).length).toBe(relatedBefore + 1))
    expect(document.body.textContent).not.toContain(WORK_ITEMS_COPY.detail.childCreateDescription)
  })

  it('keeps a failed child draft and hides creation without writable capability', async () => {
    const failed = makeEnvironment({ pathname: '/workspace/work-items/31', createError: true })
    await render(failed.environment)
    const trigger = await waitFor(() => {
      const value = [...document.querySelectorAll('button')]
        .find((entry) => entry.textContent?.includes(WORK_ITEMS_COPY.detail.createChildren))
      expect(value).toBeTruthy()
      return value as HTMLButtonElement
    })
    await act(async () => trigger.click())
    const input = document.querySelector(
      `[aria-label="${WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childName, 1)}"]`,
    ) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'Retry child' } })
    const dialog = [...document.querySelectorAll('[role="dialog"]')].at(-1) as HTMLElement
    const submit = [...dialog.querySelectorAll('button')]
      .find((entry) => entry.textContent?.includes(WORK_ITEMS_COPY.detail.createChildren))!
    await act(async () => submit.click())
    await waitFor(() => expect(failed.request.mock.calls.some(([entry]) => (
      entry.method === 'POST' && entry.path === '/tasks'
    ))).toBe(true))
    expect(input.value).toBe('Retry child')
    expect(document.body.textContent).toContain(WORK_ITEMS_COPY.detail.childPartialResult(0, 1))

    await act(async () => root?.unmount())
    root = undefined
    container?.remove()
    container = undefined

    const denied = makeEnvironment({ pathname: '/workspace/work-items/31', createAllowed: false })
    const deniedView = await render(denied.environment)
    await waitFor(() => expect(deniedView.textContent).toContain('PM native task'))
    expect(document.body.textContent).not.toContain(WORK_ITEMS_COPY.detail.createChildren)
    expect(denied.request.mock.calls.some(([entry]) => entry.method === 'POST')).toBe(false)
  })

  it('keeps the native error state when the PM list request fails', async () => {
    const { environment } = makeEnvironment({
      requestPage: async (entry) => {
        if (entry.path === '/tasks') throw new Error('PM unavailable')
        return { items: [], total: 0 }
      },
    })
    const view = await render(environment)
    await waitFor(() => expect(view.querySelector('[role="alert"]')).not.toBeNull())
  })

  it('opens creation without a selected range and validates the requirement before writing', async () => {
    const { environment, request } = makeEnvironment({ scopePayload: { project_sets: [], projects: [], requirements: [] } })
    await render(environment)
    const trigger = [...document.querySelectorAll('button')].find((button) => button.textContent === '新建任务') as HTMLButtonElement
    await waitFor(() => expect(trigger.disabled).toBe(false))
    await act(async () => trigger.click())
    const dialog = await waitFor(() => {
      const value = document.querySelector('[role="dialog"]')
      expect(value).not.toBeNull()
      return value as HTMLElement
    })
    const submit = [...dialog.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.create.submit)!
    await act(async () => submit.click())
    expect(document.body.textContent).toContain(WORK_ITEMS_COPY.create.validation.requirement)
    expect(request.mock.calls.some(([entry]) => entry.method === 'POST')).toBe(false)
  })

  it('keeps create hidden until a valid workspace is available', async () => {
    const { environment, request } = makeEnvironment({ createAllowed: false })
    await render(environment)
    const trigger = [...document.querySelectorAll('button')].find((button) => button.textContent === '新建任务')
    await waitFor(() => expect(trigger?.hasAttribute('disabled')).toBe(true))
    expect(document.body.textContent).toContain('请先选择有效工作区，再创建任务')
    expect(request.mock.calls.some(([entry]) => entry.method === 'POST')).toBe(false)
  })

  it('creates once through PM POST /tasks, opens the canonical detail and restores focus', async () => {
    const created = {
      ...task(),
      id: 51,
      task_key: 'PM-51',
      title: 'Created from React',
    }
    const { environment, request } = makeEnvironment({
      search: {
        view: 'list',
        project_set_ids: '8',
        project_ids: '13',
        requirement_ids: '21',
      },
      createResponse: created,
      usersPayload: [{ id: 101, display_name: 'Teo' }],
    })
    await render(environment)
    const trigger = [...document.querySelectorAll('button')].find((button) => button.textContent === '新建任务') as HTMLButtonElement
    await waitFor(() => expect(trigger.disabled).toBe(false))

    await act(async () => trigger.click())
    const dialog = await waitFor(() => {
      const value = document.querySelector('[role="dialog"]')
      expect(value).not.toBeNull()
      return value as HTMLElement
    })
    expect(dialog.textContent).toContain('新建全局任务')
    expect(dialog.textContent).toContain('附件')
    expect(dialog.textContent).not.toContain('故事点')
    expect(document.getElementById('work-items-create-kind')?.hasAttribute('disabled')).toBe(true)
    expect(document.getElementById('work-items-create-priority')?.textContent).toContain('普通')
    expect(document.getElementById('work-items-create-status')?.textContent).toContain('待处理')
    expect((document.getElementById('work-items-create-assignee') as HTMLInputElement | null)?.value).toBe('Teo')
    expect(document.getElementById('work-items-create-requirement-trigger')?.textContent)
      .toContain('Project set / Project / Requirement')
    const startsAt = document.getElementById('work-items-create-starts-at') as HTMLInputElement
    const dueAt = document.getElementById('work-items-create-due-at') as HTMLInputElement
    expect(startsAt.textContent).toContain('选择日期')
    expect(dueAt.textContent).toContain('选择日期')
    await selectCalendarDate(startsAt, '2026-08-05')
    await selectCalendarDate(dueAt, '2026-08-06')
    expect(startsAt.textContent).toContain('2026年8月5日')
    expect(dueAt.textContent).toContain('2026年8月6日')
    expect(document.querySelector('[aria-label="任务描述"]')?.getAttribute('data-serialization'))
      .toBe('json')
    const title = document.getElementById('work-items-create-title') as HTMLInputElement
    fireEvent.change(title, { target: { value: 'Created from React' } })
    const richDescription = JSON.stringify({
      type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Rich goal' }] }],
    })
    fireEvent.change(document.querySelector('[aria-label="任务描述"]') as HTMLTextAreaElement, {
      target: { value: richDescription },
    })
    const submit = document.querySelector(`button[form="work-items-create-form"]`) as HTMLButtonElement
    await act(async () => submit.click())

    await waitFor(() => expect(request.mock.calls.filter(([entry]) => (
      entry.method === 'POST' && entry.path === '/tasks'
    ))).toHaveLength(1))
    expect(request).toHaveBeenCalledWith(expect.objectContaining({
      method: 'POST',
      path: '/tasks',
      body: expect.objectContaining({
        requirement_id: 21,
        kind: 'task',
        title: 'Created from React',
        description: richDescription,
        status: 'draft',
        priority: 'normal',
        reporter_user_id: 101,
        assignee_id: 101,
        starts_at: new Date(2026, 7, 5, 0, 0, 0).toISOString(),
        due_at: new Date(2026, 7, 6, 23, 59, 59).toISOString(),
      }),
    }))
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/work-items?project_set_ids=8&project_ids=13&requirement_ids=21&task_id=51',
    )
    await waitFor(() => expect(document.querySelector('[role="dialog"]')).toBeNull())
    await waitFor(() => expect(document.activeElement).toBe(trigger))
  })

  it('opens quick create beside new task and posts inherited parent-child drafts atomically', async () => {
    const { environment, request } = makeEnvironment({
      search: {
        view: 'list', project_set_ids: '8', project_ids: '13', requirement_ids: '21',
      },
      usersPayload: [{ id: 101, display_name: 'Teo' }],
      batchResponse: {
        created: [{ client_id: 'parent', id: 71, children: [{ client_id: 'child', id: 72 }] }],
        total: 2,
      },
    })
    await render(environment)
    const trigger = await waitFor(() => {
      const value = [...document.querySelectorAll('button')]
        .find((button) => button.textContent === WORK_ITEMS_COPY.quickCreate.trigger) as HTMLButtonElement
      expect(value.disabled).toBe(false)
      return value
    })
    await act(async () => trigger.click())
    const dialog = await waitFor(() => document.querySelector('[role="dialog"]') as HTMLElement)
    expect(dialog.textContent).toContain(WORK_ITEMS_COPY.quickCreate.title)
    expect(dialog.className).toContain('h-[80dvh]')
    const headers = [...dialog.querySelectorAll('thead th')].map((header) => header.textContent?.trim())
    expect(headers.slice(0, 2)).toEqual([
      WORK_ITEMS_COPY.quickCreate.sequence,
      WORK_ITEMS_COPY.quickCreate.nature,
    ])
    const parentCells = dialog.querySelectorAll('tbody tr')[0]?.querySelectorAll('td')
    expect(parentCells?.[0]?.textContent?.trim()).toBe('')
    expect(parentCells?.[1]?.textContent?.trim()).toBe(WORK_ITEMS_COPY.quickCreate.taskNature)
    const title = dialog.querySelector('input[placeholder="输入任务标题"]') as HTMLInputElement
    fireEvent.change(title, { target: { value: 'Parent' } })
    const addChild = [...dialog.querySelectorAll('button')]
      .find((button) => button.textContent?.includes(WORK_ITEMS_COPY.quickCreate.addChild))!
    expect(addChild.closest('td')).toBe(parentCells?.[parentCells.length - 1])
    await act(async () => addChild.click())
    const childCells = dialog.querySelectorAll('tbody tr')[1]?.querySelectorAll('td')
    expect(childCells?.[0]?.textContent?.trim()).toBe('1')
    expect(childCells?.[1]?.textContent?.trim()).toBe(WORK_ITEMS_COPY.quickCreate.childNature)
    const addTask = [...dialog.querySelectorAll('button')]
      .find((button) => button.textContent?.includes(WORK_ITEMS_COPY.quickCreate.addTask))!
    expect(addTask.closest('[data-slot="responsive-panel-footer"]')).not.toBeNull()
    const titles = dialog.querySelectorAll('input[placeholder="输入任务标题"]')
    fireEvent.change(titles[1] as HTMLInputElement, { target: { value: 'Child' } })
    const submit = dialog.querySelector(`button[form="work-items-quick-create-form"]`) as HTMLButtonElement
    await act(async () => submit.click())

    await waitFor(() => expect(request.mock.calls.some(([entry]) => (
      entry.method === 'POST' && entry.path === '/tasks/batch-create'
    ))).toBe(true))
    const batch = request.mock.calls.find(([entry]) => entry.path === '/tasks/batch-create')?.[0]
    expect(batch?.body).toMatchObject({
      tasks: [{
        title: 'Parent', requirement_id: 21, priority: 'normal', assignee_id: 101,
        children: [{ title: 'Child', priority: 'normal', assignee_id: 101 }],
      }],
    })
  })

  it('confirms before discarding a quick-create draft', async () => {
    const { environment } = makeEnvironment({
      usersPayload: [{ id: 101, display_name: 'Teo' }],
    })
    await render(environment)
    const trigger = await waitFor(() => [...document.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.quickCreate.trigger) as HTMLButtonElement)
    await act(async () => trigger.click())
    const dialog = await waitFor(() => {
      const value = document.querySelector('[role="dialog"]')
      expect(value).not.toBeNull()
      return value as HTMLElement
    })
    const close = dialog.querySelector('[data-slot="responsive-panel-close"]') as HTMLButtonElement
    expect(close).not.toBeNull()
    await act(async () => close.click())
    const confirm = await waitFor(() => document.querySelector('[role="alertdialog"]') as HTMLElement)
    expect(confirm.textContent).toContain(WORK_ITEMS_COPY.quickCreate.unsavedTitle)
    const discard = [...confirm.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.quickCreate.discard)!
    await act(async () => discard.click())
    await waitFor(() => expect(document.querySelector('[role="dialog"]')).toBeNull())
  })

  it('stages attachments, creates once, then uploads sequentially without rolling back the task', async () => {
    const created = { ...task(), id: 51 }
    const order: string[] = []
    const { environment, request, upload } = makeEnvironment({
      search: {
        view: 'list', project_set_ids: '8', project_ids: '13', requirement_ids: '21',
      },
      createResponse: created,
      usersPayload: [{ id: 101, display_name: 'Teo' }],
      request: async (entry) => {
        if (entry.method === 'POST' && entry.path === '/tasks') {
          order.push('create')
          return created
        }
        if (entry.path === '/issue-statuses') {
          return [{ status_key: 'draft', label: '待处理', sort_order: 10 }]
        }
        if (entry.path === '/issue-workflow-transitions') return { enforce: false, transitions: [] }
        return undefined
      },
      upload: async (entry) => {
        const file = entry.formData.get('file') as File
        order.push(`upload:${file.name}`)
        if (file.name === 'second.txt') throw new Error('mock upload failed')
        return { id: 91 }
      },
    })
    await render(environment)
    const trigger = [...document.querySelectorAll('button')]
      .find((button) => button.textContent === '新建任务') as HTMLButtonElement
    await waitFor(() => expect(trigger.disabled).toBe(false))
    await act(async () => trigger.click())
    const title = await waitFor(() => document.getElementById('work-items-create-title') as HTMLInputElement)
    fireEvent.change(title, { target: { value: 'Attachment flow' } })
    const input = document.querySelector('input[aria-label="选择任务附件"]') as HTMLInputElement
    await act(async () => fireEvent.change(input, { target: { files: [
      new File(['first'], 'first.txt', { lastModified: 1 }),
      new File(['second'], 'second.txt', { lastModified: 2 }),
    ] } }))
    await waitFor(() => expect(document.body.textContent).toContain('second.txt'))

    const submit = document.querySelector(`button[form="work-items-create-form"]`) as HTMLButtonElement
    await act(async () => submit.click())

    await waitFor(() => expect(upload).toHaveBeenCalledTimes(2))
    expect(order).toEqual(['create', 'upload:first.txt', 'upload:second.txt'])
    expect(request.mock.calls.filter(([entry]) => entry.method === 'POST' && entry.path === '/tasks'))
      .toHaveLength(1)
    expect(document.body.textContent).toContain('任务已创建，部分附件上传失败')
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(environment.navigate).toHaveBeenCalledWith(
      '/workspace/work-items?project_set_ids=8&project_ids=13&requirement_ids=21&task_id=51',
    )
  })

  it('disables create attachments and rich-text paste upload without the upload action', async () => {
    const { environment, upload } = makeEnvironment({
      operationAccess: { uploadAttachment: false },
      usersPayload: [{ id: 101, display_name: 'Teo' }],
    })
    await render(environment)
    const trigger = [...document.querySelectorAll('button')]
      .find((button) => button.textContent === WORK_ITEMS_COPY.create.trigger) as HTMLButtonElement
    await waitFor(() => expect(trigger.disabled).toBe(false))
    await act(async () => trigger.click())
    const dialog = await waitFor(() => document.querySelector('[role="dialog"]') as HTMLElement)
    const input = dialog.querySelector('input[aria-label="选择任务附件"]') as HTMLInputElement
    const description = dialog.querySelector('textarea') as HTMLTextAreaElement

    expect(input.disabled).toBe(true)
    expect(description.dataset.uploadEnabled).toBe('false')
    await act(async () => fireEvent.change(input, {
      target: { files: [new File(['denied'], 'denied.txt')] },
    }))
    expect(dialog.textContent).not.toContain('denied.txt')
    expect(upload).not.toHaveBeenCalled()
  })

  it('keeps the draft open on a safe create failure and does not duplicate POST', async () => {
    const { environment, request } = makeEnvironment({
      search: {
        view: 'list',
        project_set_ids: '8',
        project_ids: '13',
        requirement_ids: '21',
      },
      createError: true,
    })
    await render(environment)
    const trigger = [...document.querySelectorAll('button')].find((button) => button.textContent === '新建任务') as HTMLButtonElement
    await waitFor(() => expect(trigger.disabled).toBe(false))
    await act(async () => trigger.click())
    const title = await waitFor(() => document.getElementById('work-items-create-title') as HTMLInputElement)
    fireEvent.change(title, { target: { value: 'Retained draft' } })
    const submit = document.querySelector(`button[form="work-items-create-form"]`) as HTMLButtonElement
    await act(async () => submit.click())

    await waitFor(() => expect(document.body.textContent).toContain('草稿已保留'))
    expect((document.getElementById('work-items-create-title') as HTMLInputElement).value).toBe('Retained draft')
    expect(request.mock.calls.filter(([entry]) => entry.method === 'POST' && entry.path === '/tasks'))
      .toHaveLength(1)
  })
})
