import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { pmKeys } from '@/shared/query'
import { PMSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  useCreatePlanUnitMutation,
  useCreateProjectMutation,
  useDeletePlanUnitMutation,
  useDeleteProjectMutation,
  useUpdatePlanUnitMutation,
  useUpdateProjectMutation,
} from './useProjectMutations'

const api = (() => ({
  createProject: vi.fn(),
  updateProject: vi.fn(),
  deleteProject: vi.fn(),
  createPlanUnit: vi.fn(),
  updatePlanUnit: vi.fn(),
  deletePlanUnit: vi.fn(),
}))()

mock.module('../api', () => ({
    createScrumProjectNew: api.createProject,
    patchScrumProjectNew: api.updateProject,
    deleteScrumProjectNew: api.deleteProject,
    createScrumVersionTaskNew: api.createPlanUnit,
    patchScrumVersionTaskNew: api.updatePlanUnit,
    deleteScrumVersionTaskNew: api.deletePlanUnit,
  }))

const projectPayload = {
  team_id: 3,
  name: 'Project',
  owner_user_id: null,
  planned_start_at: null,
  planned_end_at: null,
  status: 'active' as const,
}

const projectPatch = {
  name: 'Updated',
  description: '',
  status: 'active' as const,
}

const unitPayload = {
  project_new_id: 9,
  workflow_template_id: 8,
  title: 'Unit',
  type: 'project' as const,
  owner_user_id: null,
  planned_start_at: null,
  planned_end_at: null,
  actual_start_at: null,
  actual_completed_at: null,
  risk_level: 'low' as const,
  goal: '',
}

const unitPatch = {
  title: 'Unit 2',
  type: 'version' as const,
  owner_user_id: null,
  risk_level: 'medium' as const,
  goal: '',
}

function readyRuntime(workspaceId = 7) {
  runtimeSessionStore.getState().setStandaloneToken('p6-token')
  runtimeWorkspaceStore.getState().selectWorkspace(workspaceId)
}

function setupHook<T>(hook: () => T) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  return { client, ...renderHook(hook, { wrapper }) }
}

function expectKeyCall(
  spy: { mock: { calls: readonly (readonly unknown[])[] } },
  key: readonly unknown[],
) {
  const matched = spy.mock.calls.some(([filters]) => (
    JSON.stringify((filters as { queryKey?: readonly unknown[] } | undefined)?.queryKey) ===
    JSON.stringify(key)
  ))
  expect(matched).toBe(true)
}

describe('P6 project mutation hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    readyRuntime()
    api.createProject.mockResolvedValue({ id: 10, ...projectPayload })
    api.updateProject.mockResolvedValue({ id: 9, ...projectPatch })
    api.deleteProject.mockResolvedValue({ deleted: true })
    api.createPlanUnit.mockResolvedValue({ id: 31, ...unitPayload })
    api.updatePlanUnit.mockResolvedValue({ id: 31, project_new_id: 9, ...unitPatch })
    api.deletePlanUnit.mockResolvedValue({ deleted: true })
  })

  it('creates one project request and refreshes only project list variants', async () => {
    const hook = setupHook(useCreateProjectMutation)
    const cancel = vi.spyOn(hook.client, 'cancelQueries')
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    let result: Awaited<ReturnType<typeof hook.result.current.execute>> | undefined
    await act(async () => { result = await hook.result.current.execute(projectPayload) })

    expect(api.createProject).toHaveBeenCalledTimes(1)
    expect(api.createProject).toHaveBeenCalledWith(projectPayload)
    expect(result?.status).toBe('applied')
    expectKeyCall(cancel, pmKeys.resource(7, 'projectSets', 'list'))
    expectKeyCall(invalidate, pmKeys.resource(7, 'projectSets', 'list'))
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: pmKeys.resource(7, 'projectSets', 'list'),
      refetchType: 'active',
    })
  })

  it('refreshes project caches after a rejected update', async () => {
    api.updateProject.mockRejectedValueOnce(new PMSafeApiError('PROJECT_UPDATE_FAILED', 403))
    const hook = setupHook(useUpdateProjectMutation)
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    let failure: unknown

    await act(async () => {
      try {
        await hook.result.current.execute(9, projectPatch)
      } catch (error) {
        failure = error
      }
    })
    expect(failure).toMatchObject({ status: 403 })
    expect(api.updateProject).toHaveBeenCalledTimes(1)
    expectKeyCall(invalidate, pmKeys.resource(7, 'projectSets', 'list'))
    expectKeyCall(invalidate, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))
    expectKeyCall(invalidate, pmKeys.resource(7, 'audit', 'project', 9))
  })

  it('removes a deleted project aggregate only during explicit finalize', async () => {
    const hook = setupHook(useDeleteProjectMutation)
    const remove = vi.spyOn(hook.client, 'removeQueries')
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    let result: Awaited<ReturnType<typeof hook.result.current.execute>> | undefined
    await act(async () => { result = await hook.result.current.execute(9) })
    expect(remove).not.toHaveBeenCalled()
    expectKeyCall(invalidate, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))
    expectKeyCall(invalidate, pmKeys.resource(7, 'audit', 'project', 9))
    await act(async () => { await hook.result.current.finalize(9, result!.ticket) })
    expectKeyCall(remove, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))
    expectKeyCall(remove, pmKeys.resource(7, 'audit', 'project', 9))
  })

  it('invalidates the parent aggregate for plan-unit create, update, and delete', async () => {
    const create = setupHook(useCreatePlanUnitMutation)
    const createInvalidate = vi.spyOn(create.client, 'invalidateQueries')
    await act(async () => { await create.result.current.execute(unitPayload) })
    expect(api.createPlanUnit).toHaveBeenCalledTimes(1)
    expect(api.createPlanUnit.mock.calls[0]?.[0]).not.toHaveProperty('status')
    expectKeyCall(createInvalidate, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))

    const update = setupHook(useUpdatePlanUnitMutation)
    const updateInvalidate = vi.spyOn(update.client, 'invalidateQueries')
    await act(async () => { await update.result.current.execute(9, 31, unitPatch) })
    expect(api.updatePlanUnit).toHaveBeenCalledTimes(1)
    expect(api.updatePlanUnit.mock.calls[0]?.[1]).not.toHaveProperty('status')
    expectKeyCall(updateInvalidate, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))

    const remove = setupHook(useDeletePlanUnitMutation)
    const removeInvalidate = vi.spyOn(remove.client, 'invalidateQueries')
    let result: Awaited<ReturnType<typeof remove.result.current.execute>> | undefined
    await act(async () => { result = await remove.result.current.execute(9, 31) })
    await act(async () => { await remove.result.current.finalize(9, result!.ticket) })
    expect(api.deletePlanUnit).toHaveBeenCalledTimes(1)
    expectKeyCall(removeInvalidate, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))
  })

  it('refreshes both the project list and board after a stale plan-unit schedule update', async () => {
    api.updatePlanUnit.mockRejectedValueOnce(new PMSafeApiError(
      'SCHEDULE_SUBJECT_STALE',
      409,
      false,
      undefined,
      { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
    ))
    const hook = setupHook(useUpdatePlanUnitMutation)
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    let failure: unknown

    await act(async () => {
      try {
        await hook.result.current.execute(9, 31, unitPatch)
      } catch (error) {
        failure = error
      }
    })

    expect(failure).toMatchObject({ code: 'SCHEDULE_SUBJECT_STALE', status: 409 })
    expectKeyCall(invalidate, pmKeys.resource(7, 'projectSets', 'list'))
    expectKeyCall(invalidate, pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'))
    expectKeyCall(invalidate, pmKeys.resource(7, 'audit', 'project', 9))
  })

  it('marks a settled result stale and skips success invalidation after workspace change', async () => {
    let resolveRequest: ((value: { id: number; team_id: number; name: string; status: 'active' }) => void) | undefined
    api.createProject.mockReturnValueOnce(new Promise((resolve) => { resolveRequest = resolve }))
    const hook = setupHook(useCreateProjectMutation)
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    const pending = hook.result.current.execute(projectPayload)

    runtimeWorkspaceStore.getState().selectWorkspace(8)
    resolveRequest?.({ id: 12, team_id: 3, name: 'Stale', status: 'active' })
    await expect(pending).resolves.toMatchObject({ status: 'stale' })
    expect(invalidate).not.toHaveBeenCalled()
  })
})
