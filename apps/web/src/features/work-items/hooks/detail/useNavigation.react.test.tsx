import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { WorkItemsClient } from '../../api/client'
import type { WorkItemsEnvironment } from '../../model/environment'
import { useWorkItemDetailNavigation } from './useNavigation'

describe('useWorkItemDetailNavigation', () => {
  it('keeps child detail deep links and returns to the parent through the owned stack', async () => {
    const navigate = vi.fn().mockResolvedValue(undefined)
    const environment = {
      client: {} as WorkItemsClient,
      workspaceId: 7,
      userId: 9,
      sessionRevision: 1,
      pathname: '/workspace/work-items',
      search: { view: 'list', task_id: 31 },
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
      navigate,
    } satisfies WorkItemsEnvironment
    const { result, rerender } = renderHook(
      ({ id }) => useWorkItemDetailNavigation(environment, id, true),
      { initialProps: { id: 31 } },
    )

    act(() => result.current.openTask(32))
    expect(navigate).toHaveBeenLastCalledWith('/workspace/work-items?task_id=32')
    rerender({ id: 32 })
    await waitFor(() => expect(result.current.canGoBack).toBe(true))

    act(() => result.current.goBack())
    expect(navigate).toHaveBeenLastCalledWith('/workspace/work-items?task_id=31')

    act(() => result.current.openRequirement({
      projectSetId: 5,
      projectId: 15,
      requirementId: 97,
    }))
    expect(navigate).toHaveBeenLastCalledWith(
      '/workspace/projects/5/items/15?tab=requirements',
    )
  })
})
