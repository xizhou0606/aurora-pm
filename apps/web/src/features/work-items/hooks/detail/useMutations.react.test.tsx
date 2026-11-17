import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import type { PMTransportRequest } from '../../api/client'
import type { WorkItemsEnvironment } from '../../model/environment'
import type { WorkItemRelated } from '../../model/types'
import { useWorkItemChildDeleteMutation } from './useMutations'

function child(canDelete = true): WorkItemRelated {
  return {
    id: 32,
    rowId: 'task:32',
    relation: 'child',
    title: 'Child task',
    status: 'draft',
    watched: false,
    capabilities: {
      canUpdate: true,
      canDelete,
      canComment: true,
      canAttach: true,
      canWatch: true,
      canClone: false,
      allowedTransitions: [],
    },
  }
}

function setup(deleteTask = true) {
  const request = vi.fn().mockResolvedValue(undefined)
  const environment = {
    workspaceId: 7,
    userId: 101,
    sessionRevision: 1,
    pathname: '/workspace/work-items',
    search: {},
    allowed: true,
    createAllowed: true,
    operationAccess: {
      status: 'ready', retry: vi.fn(), viewReferenceData: true,
      viewTaskTable: true, viewTaskDetail: true, createTask: true,
      updateTask: true, transitionTask: true, deleteTask, viewAttachments: true,
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
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  return { environment, request, wrapper }
}

describe('useWorkItemChildDeleteMutation', () => {
  it('reuses the task delete adapter for an allowed child', async () => {
    const { environment, request, wrapper } = setup()
    const { result } = renderHook(
      () => useWorkItemChildDeleteMutation(environment, 31, ['tasks']),
      { wrapper },
    )

    await act(async () => result.current.mutateAsync(child()))
    expect(request).toHaveBeenCalledWith({
      method: 'DELETE',
      path: '/tasks/32',
      query: undefined,
    })
  })

  it('fails closed before deleting when tasks.delete or object capability is denied', async () => {
    const { environment, request, wrapper } = setup(false)
    const { result } = renderHook(
      () => useWorkItemChildDeleteMutation(environment, 31, ['tasks']),
      { wrapper },
    )

    await act(async () => {
      await expect(result.current.mutateAsync(child(false)))
        .rejects.toMatchObject({ code: 'WORK_ITEMS_ACTION_FORBIDDEN', status: 403 })
    })
    expect(request).not.toHaveBeenCalled()
  })
})
