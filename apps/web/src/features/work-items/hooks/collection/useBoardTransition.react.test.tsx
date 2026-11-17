import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { WorkItem, WorkItemWorkflowCatalog } from '../../model/types'
import { useWorkItemBoardTransition } from './useBoardTransition'

const workflow: WorkItemWorkflowCatalog = {
  statuses: [
    { key: 'draft', label: '待处理', sortOrder: 10 },
    { key: 'in_dev', label: '进行中', sortOrder: 20 },
    { key: 'completed', label: '已完成', sortOrder: 30 },
  ],
  transitions: [
    { from: 'draft', to: 'in_dev' },
    { from: 'in_dev', to: 'completed' },
  ],
  strict: true,
}

function row(overrides: Partial<WorkItem> = {}): WorkItem {
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
      allowedTransitions: ['in_dev', 'completed'],
    },
    ...overrides,
  }
}

describe('useWorkItemBoardTransition', () => {
  it('fails closed unless row capability and strict workflow both allow the target', () => {
    const update = vi.fn().mockResolvedValue(true)
    const { result } = renderHook(() => useWorkItemBoardTransition(workflow, update))

    expect(result.current.canDrag(row())).toBe(true)
    expect(result.current.canDrag(row({
      capabilities: { ...row().capabilities, canUpdate: false },
    }))).toBe(false)
    act(() => expect(result.current.start(row())).toBe(true))
    expect(result.current.canDrop('completed')).toBe(false)
    expect(result.current.canDrop('in_dev')).toBe(true)
  })

  it('owns drag state and submits exactly one allowed status mutation', async () => {
    const update = vi.fn().mockResolvedValue(true)
    const task = row()
    const { result } = renderHook(() => useWorkItemBoardTransition(workflow, update))

    act(() => {
      result.current.start(task)
      result.current.over('in_dev')
    })
    expect(result.current.state).toMatchObject({
      status: 'dragging', rowId: 'task:31', targetStatus: 'in_dev',
    })
    act(() => result.current.drop('in_dev'))
    await waitFor(() => expect(update).toHaveBeenCalledTimes(1))
    expect(update).toHaveBeenCalledWith(task, { status: 'in_dev' })
    await waitFor(() => expect(result.current.state).toEqual({ status: 'idle' }))
  })

  it('keeps an explicit error state and retries the failed command', async () => {
    const update = vi.fn()
      .mockRejectedValueOnce(new Error('mock patch failed'))
      .mockResolvedValueOnce(true)
    const task = row()
    const { result } = renderHook(() => useWorkItemBoardTransition(workflow, update))

    act(() => {
      result.current.start(task)
      result.current.drop('in_dev')
    })
    await waitFor(() => expect(result.current.state).toMatchObject({
      status: 'error', rowId: 'task:31', targetStatus: 'in_dev',
    }))
    act(() => result.current.retry())
    await waitFor(() => expect(update).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(result.current.state).toEqual({ status: 'idle' }))
  })

  it('provides the keyboard-equivalent adjacent transition', async () => {
    const update = vi.fn().mockResolvedValue(true)
    const task = row()
    const { result } = renderHook(() => useWorkItemBoardTransition(workflow, update))

    act(() => result.current.moveByKeyboard(task, 1))
    await waitFor(() => expect(update).toHaveBeenCalledWith(task, { status: 'in_dev' }))
  })
})
