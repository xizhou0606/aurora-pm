import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import { pmKeys } from '@/shared/query'
import type { PMTransportRequest } from '../../api/client'
import type { WorkItemsEnvironment } from '../../model/environment'
import type { WorkItem, WorkItemsPage, WorkItemsRouteState } from '../../model/types'
import { parseWorkItemsRoute } from '../../model/utils'
import { pmTasksQueryKey } from './useQueries'
import {
  useWorkItemCreateAttachmentsMutation,
  useWorkItemDeleteMutation,
  useWorkItemUpdateMutation,
} from './useMutations'

function row(): WorkItem {
  return {
    id: 31,
    rowId: 'task:31',
    title: 'Board task',
    status: 'draft',
    watched: false,
    capabilities: {
      canUpdate: true,
      canDelete: false,
      canComment: false,
      canAttach: false,
      canWatch: false,
      canClone: false,
      allowedTransitions: ['in_dev'],
    },
  }
}

describe('useWorkItemUpdateMutation', () => {
  it('optimistically moves a board row and restores the prior page on PATCH failure', async () => {
    let rejectPatch: ((reason?: unknown) => void) | undefined
    const request = vi.fn((_entry: PMTransportRequest) => new Promise<unknown>((_resolve, reject) => {
      rejectPatch = reject
    }))
    const environment = {
      workspaceId: 7,
      userId: 101,
      sessionRevision: 1,
      pathname: '/workspace/work-items',
      search: { view: 'board', project_ids: '13' },
      allowed: true,
      createAllowed: true,
      operationAccess: {
        status: 'ready', retry: vi.fn(), viewReferenceData: true,
        viewTaskTable: true, viewTaskDetail: true, createTask: true,
        updateTask: true, transitionTask: true, deleteTask: true, viewAttachments: true,
        uploadAttachment: true, downloadAttachment: true, deleteAttachment: true,
        viewComments: true, createComment: true, deleteComment: true,
        updateWatching: true,
      },
      navigate: vi.fn().mockResolvedValue(undefined),
      client: {
        request: <T,>(entry: PMTransportRequest) => request(entry) as Promise<T>,
        requestPage: vi.fn(),
        requestPMScopeOptions: vi.fn(),
        requestPMUsers: vi.fn(),
      },
    } satisfies WorkItemsEnvironment
    const route: WorkItemsRouteState = parseWorkItemsRoute(
      environment.pathname,
      environment.search,
    )
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const key = pmTasksQueryKey(environment.workspaceId, route)
    queryClient.setQueryData<WorkItemsPage>(key, { items: [row()], total: 1 })
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(
      () => useWorkItemUpdateMutation(environment, route),
      { wrapper },
    )

    act(() => result.current.mutate({ row: row(), patch: { status: 'in_dev' } }))
    await waitFor(() => expect(
      queryClient.getQueryData<WorkItemsPage>(key)?.items[0].status,
    ).toBe('in_dev'))
    expect(request).toHaveBeenCalledTimes(1)
    expect(request).toHaveBeenCalledWith({
      method: 'PATCH',
      path: '/tasks/31',
      body: { status: 'in_dev' },
    })
    act(() => rejectPatch?.(new Error('mock patch failed')))
    await waitFor(() => expect(result.current.status).toBe('error'))
    expect(queryClient.getQueryData<WorkItemsPage>(key)?.items[0].status).toBe('draft')
  })
})

function guardedEnvironment(input: {
  uploadAttachment?: boolean
  deleteTask?: boolean
} = {}) {
  const request = vi.fn().mockResolvedValue(undefined)
  const upload = vi.fn().mockResolvedValue({ id: 91 })
  return {
    environment: {
      workspaceId: 7,
      userId: 101,
      sessionRevision: 1,
      pathname: '/workspace/work-items',
      search: { view: 'list' },
      allowed: true,
      createAllowed: true,
      operationAccess: {
        status: 'ready', retry: vi.fn(), viewReferenceData: true,
        viewTaskTable: true, viewTaskDetail: true, createTask: true,
        updateTask: true, transitionTask: true, deleteTask: input.deleteTask ?? true, viewAttachments: true,
        uploadAttachment: input.uploadAttachment ?? true,
        downloadAttachment: true, deleteAttachment: true,
        viewComments: true, createComment: true, deleteComment: true,
        updateWatching: true,
      },
      navigate: vi.fn().mockResolvedValue(undefined),
      client: {
        request: <T,>(entry: PMTransportRequest) => request(entry) as Promise<T>,
        requestPage: vi.fn(),
        requestPMScopeOptions: vi.fn(),
        requestPMUsers: vi.fn(),
        upload: <T,>(entry: Parameters<NonNullable<WorkItemsEnvironment['client']['upload']>>[0]) => (
          upload(entry) as Promise<T>
        ),
      },
    } satisfies WorkItemsEnvironment,
    request,
    upload,
  }
}

function mutationWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('work-item mutation permission boundaries', () => {
  it('fails closed before attachment upload when attachments.upload is denied', async () => {
    const { environment, upload } = guardedEnvironment({ uploadAttachment: false })
    const { result } = renderHook(
      () => useWorkItemCreateAttachmentsMutation(environment),
      { wrapper: mutationWrapper() },
    )

    await act(async () => {
      await expect(result.current.mutateAsync({
        taskId: 31,
        files: [new File(['denied'], 'denied.txt')],
      })).rejects.toMatchObject({ code: 'WORK_ITEMS_ACTION_FORBIDDEN', status: 403 })
    })
    expect(upload).not.toHaveBeenCalled()
  })

  it('fails closed before task deletion when object deletion is denied', async () => {
    const { environment, request } = guardedEnvironment()
    const route = parseWorkItemsRoute(environment.pathname, environment.search)
    const { result } = renderHook(
      () => useWorkItemDeleteMutation(environment, route),
      { wrapper: mutationWrapper() },
    )

    await act(async () => {
      await expect(result.current.mutateAsync({ row: row(), cascade: false }))
        .rejects.toMatchObject({ code: 'WORK_ITEMS_ACTION_FORBIDDEN', status: 403 })
    })
    expect(request).not.toHaveBeenCalled()
  })

  it('invalidates requirement progress caches after task deletion', async () => {
    const { environment, request } = guardedEnvironment()
    request.mockResolvedValue(undefined)
    const route = parseWorkItemsRoute(environment.pathname, environment.search)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const poolKey = ['ai-pm', environment.workspaceId, 'requirements', { q: '' }]
    const requirementKey = pmKeys.resource(environment.workspaceId, 'requirements')
    queryClient.setQueryData(poolKey, { rows: [], total: 0 })
    queryClient.setQueryData(requirementKey, [])
    const { result } = renderHook(
      () => useWorkItemDeleteMutation(environment, route),
      { wrapper },
    )
    const deletableRow = {
      ...row(),
      capabilities: { ...row().capabilities, canDelete: true },
    }

    await act(async () => {
      await result.current.mutateAsync({ row: deletableRow, cascade: false })
    })
    expect(queryClient.getQueryState(poolKey)?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(requirementKey)?.isInvalidated).toBe(true)
  })
})
