import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { PMSafeApiError } from '@/shared/api'
import { act, renderHook as renderTestingHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import type { ProjectMember, ProjectMembersPage } from '../api'
import { useProjectMembersModel } from './useProjectMembersModel'

const getProjectMembers = vi.fn()
const getProjectMemberRemovalImpact = vi.fn()
const putProjectMembers = vi.fn()

vi.mock('../api', () => ({
  getProjectMembers: (...args: unknown[]) => getProjectMembers(...args),
  getProjectMemberRemovalImpact: (...args: unknown[]) => getProjectMemberRemovalImpact(...args),
  putProjectMembers: (...args: unknown[]) => putProjectMembers(...args),
}))

function member(id: number): ProjectMember {
  return { id, username: `user-${id}`, display_name: `用户 ${id}` }
}

function page(items: ProjectMember[], hasMore: boolean): ProjectMembersPage {
  return { items, page: 1, page_size: items.length, total: items.length, has_more: hasMore }
}

function renderHook<T>(callback: () => T) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return renderTestingHook(callback, { wrapper: ({ children }: { children: React.ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider> })
}

const task = { id: 9, task_key: 'PM-9', title: '待交接', status: 'in_dev', assignee_id: 2, updated_at: '2026-10-12T00:00:00Z' }
const impact = { open_task_count: 1, open_bug_count: 0, active_workflow_count: 0, plan_unit_owner_count: 0, responsible_items: [], tasks: [task], can_resolve_tasks: true, members_updated_at: '2026-10-12T00:00:00Z' }

describe('useProjectMembersModel prefill', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(2)
    getProjectMembers.mockReset().mockResolvedValue(page([member(1), member(2), member(3)], false))
    putProjectMembers.mockReset().mockResolvedValue({})
    getProjectMemberRemovalImpact.mockReset().mockResolvedValue(impact)
  })

  it('pages through the full member roster before prefilled participants', async () => {
    getProjectMembers
      .mockImplementationOnce(() => Promise.resolve(page([member(1), member(2)], true)))
      .mockImplementationOnce(() => Promise.resolve(page([member(3)], true)))
      .mockImplementationOnce(() => Promise.resolve(page([member(4)], false)))

    const { result } = renderHook(() =>
      useProjectMembersModel({ planUnitId: 52, currentOwnerId: 1 }),
    )

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.members.map((m) => m.id)).toEqual([1, 2, 3, 4])
    expect(result.current.participants).toEqual([2, 3, 4])
    expect(getProjectMembers).toHaveBeenCalledTimes(3)
    expect(getProjectMembers).toHaveBeenNthCalledWith(1, 52, expect.objectContaining({ page: 1 }), expect.any(AbortSignal))
    expect(getProjectMembers).toHaveBeenNthCalledWith(3, 52, expect.objectContaining({ page: 3 }), expect.any(AbortSignal))
  })

  it('keeps a single-page roster when the server reports no more pages', async () => {
    getProjectMembers.mockImplementationOnce(() => Promise.resolve(page([member(1)], false)))

    const { result } = renderHook(() =>
      useProjectMembersModel({ planUnitId: 52, currentOwnerId: 1 }),
    )

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.members.map((m) => m.id)).toEqual([1])
    expect(result.current.participants).toEqual([])
    expect(getProjectMembers).toHaveBeenCalledTimes(1)
  })

  it('save sends the full participant list so no member is silently dropped', async () => {
    getProjectMembers
      .mockImplementationOnce(() => Promise.resolve(page([member(1), member(2)], true)))
      .mockImplementationOnce(() => Promise.resolve(page([member(3)], false)))
    putProjectMembers.mockImplementationOnce(() => Promise.resolve({}))

    const { result } = renderHook(() =>
      useProjectMembersModel({ planUnitId: 52, currentOwnerId: 1 }),
    )
    await waitFor(() => expect(result.current.loading).toBe(false))

    let saved = false
    await act(async () => {
      saved = await result.current.onSave()
    })

    expect(saved).toBe(true)
    expect(putProjectMembers).toHaveBeenCalledWith(52, expect.objectContaining({
      owner_user_id: 1,
      user_ids: [2, 3],
    }))
  })
})


describe('member removal confirmation', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(2)
    getProjectMembers.mockReset().mockResolvedValue(page([member(1), member(2), member(3)], false))
    putProjectMembers.mockReset().mockResolvedValue({})
    getProjectMemberRemovalImpact.mockReset().mockResolvedValue(impact)
  })
  async function prepare() {
    const hook = renderHook(() => useProjectMembersModel({ planUnitId: 59, currentOwnerId: 1 }))
    await waitFor(() => expect(hook.result.current.loading).toBe(false))
    act(() => hook.result.current.onParticipantsChange([3]))
    await act(async () => { expect(await hook.result.current.onSave()).toBe(false) })
    return hook
  }
  it('previews all removed members and does not write before confirmation; cancel is safe', async () => {
    const { result } = await prepare()
    expect(result.current.preview?.tasks).toEqual([task])
    expect(getProjectMemberRemovalImpact).toHaveBeenCalledWith(59, 2, expect.any(AbortSignal))
    expect(putProjectMembers).not.toHaveBeenCalled()
    act(() => result.current.cancelRemoval())
    expect(result.current.preview).toBeNull()
    expect(putProjectMembers).not.toHaveBeenCalled()
  })
  it('explicit clear sends a null assignee and the exact reviewed snapshot', async () => {
    const { result } = await prepare()
    await act(async () => { expect(await result.current.confirmRemoval(null)).toBe(true) })
    expect(putProjectMembers).toHaveBeenCalledWith(59, expect.objectContaining({
      user_ids: [3], expected_updated_at: impact.members_updated_at,
      task_resolution: { tasks: [task], assignee_user_id: null },
    }))
  })
  it('allows reassignment to a retained member but rejects the removed member', async () => {
    const { result } = await prepare()
    await act(async () => { expect(await result.current.confirmRemoval(2)).toBe(false) })
    expect(putProjectMembers).not.toHaveBeenCalled()
    await act(async () => { expect(await result.current.confirmRemoval(3)).toBe(true) })
    expect(putProjectMembers).toHaveBeenCalledWith(59, expect.objectContaining({ task_resolution: { tasks: [task], assignee_user_id: 3 } }))
  })
  it('blocks confirmation on incomplete previews or missing task permission', async () => {
    getProjectMemberRemovalImpact.mockResolvedValue({ ...impact, can_resolve_tasks: false })
    const { result } = await prepare()
    expect(result.current.preview?.blockedReason).toBeTruthy()
    await act(async () => { expect(await result.current.confirmRemoval(null)).toBe(false) })
    expect(putProjectMembers).not.toHaveBeenCalled()
  })
  it('keeps the dialog after concurrent change and requires a refreshed preview', async () => {
    const { result } = await prepare()
    putProjectMembers.mockRejectedValueOnce(new PMSafeApiError('PROJECT_MEMBER_TASKS_CHANGED', 409))
    await act(async () => { expect(await result.current.confirmRemoval(null)).toBe(false) })
    expect(result.current.preview?.blockedReason).toBeTruthy()
    expect(result.current.error).not.toContain('PM API request failed')
    await act(async () => { expect(await result.current.confirmRemoval(null)).toBe(false) })
    expect(putProjectMembers).toHaveBeenCalledTimes(1)
  })
  it('fails closed when preview loading fails', async () => {
    getProjectMemberRemovalImpact.mockRejectedValue(new Error('offline'))
    const { result } = await prepare()
    expect(result.current.error).toBeTruthy()
    expect(putProjectMembers).not.toHaveBeenCalled()
  })
})
