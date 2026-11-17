import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { pmKeys } from '@/shared/query'
import { PMSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  useCreateRequirementMutation,
  useCreateTaskMutation,
  useDeleteRequirementMutation,
  useDeleteTaskMutation,
  useMoveRequirementMutation,
  useUpdateRequirementMutation,
  useUpdateTaskMutation,
} from './useContentMutations'

const api = (() => ({
  createRequirement: vi.fn(),
  updateRequirement: vi.fn(),
  deleteRequirement: vi.fn(),
  createTask: vi.fn(),
  updateTask: vi.fn(),
  deleteTask: vi.fn(),
}))()

mock.module('../api', () => ({
  createScrumProductRequirementNew: api.createRequirement,
  patchScrumProductRequirementNew: api.updateRequirement,
  deleteScrumProductRequirementNew: api.deleteRequirement,
  createProjectWorkTask: api.createTask,
  patchProjectWorkTask: api.updateTask,
  deleteProjectWorkTask: api.deleteTask,
}))

const requirementPayload = {
  version_task_new_id: 31,
  workflow_template_id: 8,
  title: 'Requirement',
  priority: 'high' as const,
}

const requirementPatch = {
  expected_version: 7,
  title: 'Requirement updated',
}

const taskPayload = {
  requirement_id: 41,
  kind: 'task' as const,
  title: 'Task',
  priority: 'high' as const,
}

const taskPatch = {
  title: 'Task updated',
  status: 'in_dev' as const,
}

function readyRuntime(workspaceId = 7) {
  runtimeSessionStore.getState().clearSession()
  runtimeSessionStore.getState().setStandaloneToken('p7-content-token')
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

function contentKeys(workspaceId: number, projectIds: number[]) {
  return [
    pmKeys.resource(workspaceId, 'projectSets', 'list'),
    ...projectIds.flatMap((projectId) => [
      pmKeys.resource(
        workspaceId,
        'planUnits',
        'board',
        projectId,
        'summary-version-tasks',
      ),
      pmKeys.resource(workspaceId, 'audit', 'project', projectId),
      pmKeys.resource(workspaceId, 'requirements', 'list', 'project', projectId),
      pmKeys.resource(workspaceId, 'tasks', 'list', 'project', projectId),
    ]),
  ]
}

function recordedKeys(spy: { mock: { calls: readonly (readonly unknown[])[] } }) {
  return spy.mock.calls.map(([filters]) => (
    (filters as { queryKey?: readonly unknown[] } | undefined)?.queryKey
  ))
}

function expectExactScope(
  cancel: { mock: { calls: readonly (readonly unknown[])[] } },
  invalidate: { mock: { calls: readonly (readonly unknown[])[] } },
  workspaceId: number,
  projectIds: number[],
) {
  const expected = contentKeys(workspaceId, projectIds)
  expect(recordedKeys(cancel)).toEqual(expected)
  expect(recordedKeys(invalidate)).toEqual(expected)
}

describe('P7 content mutation hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    readyRuntime()
    api.createRequirement.mockResolvedValue({ id: 41, ...requirementPayload })
    api.updateRequirement.mockResolvedValue({
      id: 41,
      version: 8,
      team_id: 2,
      project_new_id: 9,
      version_task_new_id: 31,
      priority: 'high',
      ...requirementPatch,
    })
    api.deleteRequirement.mockResolvedValue({ deleted: true })
    api.createTask.mockResolvedValue({ id: 51, ...taskPayload })
    api.updateTask.mockResolvedValue({
      id: 51,
      team_id: 2,
      project_set_id: 9,
      project_id: 31,
      kind: 'task',
      requirement_id: 41,
      ...taskPatch,
    })
    api.deleteTask.mockResolvedValue({ deleted: true })
  })

  it('cancels and invalidates the exact requirement scope for create, update and delete', async () => {
    const create = setupHook(useCreateRequirementMutation)
    const createCancel = vi.spyOn(create.client, 'cancelQueries')
    const createInvalidate = vi.spyOn(create.client, 'invalidateQueries')
    await act(async () => {
      await create.result.current.execute(9, requirementPayload)
    })
    expect(api.createRequirement).toHaveBeenCalledWith(requirementPayload)
    expect(api.createRequirement.mock.calls[0]?.[0]).not.toHaveProperty('status')
    expectExactScope(createCancel, createInvalidate, 7, [9])

    const update = setupHook(useUpdateRequirementMutation)
    const updateCancel = vi.spyOn(update.client, 'cancelQueries')
    const updateInvalidate = vi.spyOn(update.client, 'invalidateQueries')
    await act(async () => {
      await update.result.current.execute(9, 41, requirementPatch)
    })
    expect(api.updateRequirement).toHaveBeenCalledWith(41, requirementPatch)
    expect(api.updateRequirement.mock.calls[0]?.[1]).not.toHaveProperty('status')
    expectExactScope(updateCancel, updateInvalidate, 7, [9])

    const remove = setupHook(useDeleteRequirementMutation)
    const removeCancel = vi.spyOn(remove.client, 'cancelQueries')
    const removeInvalidate = vi.spyOn(remove.client, 'invalidateQueries')
    await act(async () => {
      await remove.result.current.execute(9, 41)
    })
    expect(api.deleteRequirement).toHaveBeenCalledWith(41)
    expectExactScope(removeCancel, removeInvalidate, 7, [9])
  })

  it('cancels and invalidates the exact task scope for create, update and delete', async () => {
    const create = setupHook(useCreateTaskMutation)
    const createCancel = vi.spyOn(create.client, 'cancelQueries')
    const createInvalidate = vi.spyOn(create.client, 'invalidateQueries')
    await act(async () => {
      await create.result.current.execute(9, taskPayload)
    })
    expect(api.createTask).toHaveBeenCalledWith(taskPayload)
    expectExactScope(createCancel, createInvalidate, 7, [9])

    const update = setupHook(useUpdateTaskMutation)
    const updateCancel = vi.spyOn(update.client, 'cancelQueries')
    const updateInvalidate = vi.spyOn(update.client, 'invalidateQueries')
    await act(async () => {
      await update.result.current.execute(9, 51, taskPatch)
    })
    expect(api.updateTask).toHaveBeenCalledWith(51, taskPatch)
    expectExactScope(updateCancel, updateInvalidate, 7, [9])

    const remove = setupHook(useDeleteTaskMutation)
    const removeCancel = vi.spyOn(remove.client, 'cancelQueries')
    const removeInvalidate = vi.spyOn(remove.client, 'invalidateQueries')
    await act(async () => {
      await remove.result.current.execute(9, 51)
    })
    expect(api.deleteTask).toHaveBeenCalledWith(51)
    expectExactScope(removeCancel, removeInvalidate, 7, [9])
  })

  it('cancels and invalidates both source and target project scopes for move', async () => {
    const hook = setupHook(useMoveRequirementMutation)
    const cancel = vi.spyOn(hook.client, 'cancelQueries')
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')

    await act(async () => {
      await hook.result.current.execute({
        sourceProjectId: 9,
        targetProjectId: 10,
        requirementId: 41,
        targetPlanUnitId: 32,
        expectedVersion: 7,
      })
    })

    expect(api.updateRequirement).toHaveBeenCalledWith(41, {
      version_task_new_id: 32,
      expected_version: 7,
    })
    expectExactScope(cancel, invalidate, 7, [9, 10])
  })

  it('reuses the frozen move version and target when confirming a suggested schedule', async () => {
    const hook = setupHook(useMoveRequirementMutation)

    await act(async () => {
      await hook.result.current.execute({
        sourceProjectId: 9,
        targetProjectId: 10,
        requirementId: 41,
        targetPlanUnitId: 32,
        expectedVersion: 7,
        confirmSubjectSchedule: true,
      })
    })

    expect(api.updateRequirement).toHaveBeenCalledWith(41, {
      version_task_new_id: 32,
      expected_version: 7,
      confirm_subject_schedule: true,
    })
  })

  it('refreshes the exact resource scope after 403 and 404 responses', async () => {
    api.updateRequirement.mockRejectedValueOnce(
      new PMSafeApiError('REQUIREMENT_UPDATE_FAILED', 403),
    )
    const update = setupHook(useUpdateRequirementMutation)
    const updateCancel = vi.spyOn(update.client, 'cancelQueries')
    const updateInvalidate = vi.spyOn(update.client, 'invalidateQueries')
    await act(async () => {
      await expect(update.result.current.execute(9, 41, requirementPatch))
        .rejects.toMatchObject({ status: 403 })
    })
    expectExactScope(updateCancel, updateInvalidate, 7, [9])

    api.deleteTask.mockRejectedValueOnce(
      new PMSafeApiError('TASK_DELETE_FAILED', 404),
    )
    const remove = setupHook(useDeleteTaskMutation)
    const removeCancel = vi.spyOn(remove.client, 'cancelQueries')
    const removeInvalidate = vi.spyOn(remove.client, 'invalidateQueries')
    await act(async () => {
      await expect(remove.result.current.execute(9, 51))
        .rejects.toMatchObject({ status: 404 })
    })
    expectExactScope(removeCancel, removeInvalidate, 7, [9])
  })

  it('refreshes requirement data after a 409 while preserving the caller-visible conflict', async () => {
    api.updateRequirement.mockRejectedValueOnce(
      new PMSafeApiError('REQUIREMENT_CONFLICT', 409),
    )
    const update = setupHook(useUpdateRequirementMutation)
    const cancel = vi.spyOn(update.client, 'cancelQueries')
    const invalidate = vi.spyOn(update.client, 'invalidateQueries')

    await act(async () => {
      await expect(update.result.current.execute(9, 41, requirementPatch))
        .rejects.toMatchObject({ status: 409 })
    })

    expectExactScope(cancel, invalidate, 7, [9])
  })

  it('returns the server-refreshed requirement version after a successful update', async () => {
    const update = setupHook(useUpdateRequirementMutation)
    let result: Awaited<ReturnType<typeof update.result.current.execute>> | undefined

    await act(async () => {
      result = await update.result.current.execute(9, 41, requirementPatch)
    })

    expect(result).toMatchObject({
      status: 'applied',
      data: { id: 41, version: 8 },
    })
  })

  it('marks workspace-switched results stale without invalidating the new scope', async () => {
    let resolveRequest: ((value: { id: number; title: string }) => void) | undefined
    api.createRequirement.mockReturnValueOnce(new Promise((resolve) => {
      resolveRequest = resolve
    }))
    const hook = setupHook(useCreateRequirementMutation)
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    const newScopeKey = pmKeys.resource(8, 'requirements', 'list', 'project', 9)
    hook.client.setQueryData(newScopeKey, { marker: 'new workspace' })

    const pending = hook.result.current.execute(9, requirementPayload)
    runtimeWorkspaceStore.getState().selectWorkspace(8)
    resolveRequest?.({ id: 41, title: requirementPayload.title })

    await expect(pending).resolves.toMatchObject({ status: 'stale' })
    expect(invalidate).not.toHaveBeenCalled()
    expect(hook.client.getQueryState(newScopeKey)?.isInvalidated).toBe(false)
    expect(hook.client.getQueryData(newScopeKey)).toEqual({ marker: 'new workspace' } as never)
  })

  it('marks session-switched results stale without invalidating the replacement session scope', async () => {
    let resolveRequest: ((value: { id: number; kind: 'task'; title: string }) => void) | undefined
    api.createTask.mockReturnValueOnce(new Promise((resolve) => {
      resolveRequest = resolve
    }))
    const hook = setupHook(useCreateTaskMutation)
    const invalidate = vi.spyOn(hook.client, 'invalidateQueries')
    const replacementScopeKey = pmKeys.resource(7, 'tasks', 'list', 'project', 9)
    hook.client.setQueryData(replacementScopeKey, { marker: 'replacement session' })

    const pending = hook.result.current.execute(9, taskPayload)
    runtimeSessionStore.getState().setStandaloneToken('replacement-session-token')
    resolveRequest?.({ id: 51, kind: 'task', title: taskPayload.title })

    await expect(pending).resolves.toMatchObject({ status: 'stale' })
    expect(invalidate).not.toHaveBeenCalled()
    expect(hook.client.getQueryState(replacementScopeKey)?.isInvalidated).toBe(false)
    expect(hook.client.getQueryData(replacementScopeKey))
      .toEqual({ marker: 'replacement session' } as never)
  })
})
