import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import type {
  ScrumProductRequirementNew,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { useProjectContentWorkflow } from './useProjectContentWorkflow'

const controlled = (() => ({
  requirements: [] as Array<Record<string, unknown>>,
  targetUnits: [] as Array<Record<string, unknown>>,
  reloadRequirements: vi.fn(),
  move: vi.fn(),
  unused: vi.fn(),
}))()

mock.module('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock('./useContentListModels', () => ({
  useRequirementListModel: () => ({
    state: { status: 'empty', summary: 'empty', reason: 'initial' },
    rows: controlled.requirements,
    reload: controlled.reloadRequirements,
  }),
  useTaskListModel: () => ({
    state: { status: 'empty', summary: 'empty', reason: 'initial' },
    rows: [],
  }),
}))

mock.module('../model', () => ({
  useProjectListQuery: () => ({
    status: 'success',
    data: { rows: [{ id: 9, name: 'PM' }], total: 1 },
    refetch: vi.fn(),
  }),
  useProjectBoardQuery: (projectId: number) => ({
    status: 'success',
    data: {
      version_tasks: controlled.targetUnits.filter((unit) => unit.project_new_id === projectId),
    },
    refetch: vi.fn(),
  }),
  usePublishedRequirementWorkflowTemplatesQuery: () => ({
    status: 'success', data: [], refetch: vi.fn(),
  }),
  useRequirementListQuery: () => ({
    status: 'success', data: { rows: [], total: 0 }, refetch: vi.fn(),
  }),
  useTaskDeepLinkQuery: () => ({
    status: 'success', data: undefined, error: null, refetch: vi.fn(),
  }),
}))

vi.mock('./useContentMutations', () => ({
  useCreateRequirementMutation: () => ({ execute: controlled.unused }),
  useCreateTaskMutation: () => ({ execute: controlled.unused }),
  useDeleteRequirementMutation: () => ({ execute: controlled.unused }),
  useDeleteTaskMutation: () => ({ execute: controlled.unused }),
  useMoveRequirementMutation: () => ({ execute: controlled.move }),
  useUpdateRequirementMutation: () => ({ execute: controlled.unused }),
  useUpdateTaskMutation: () => ({ execute: controlled.unused }),
}))

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'PM',
  status: 'active',
}

const sourceUnit: ScrumVersionTaskNew = {
  id: 31,
  team_id: 2,
  project_new_id: 9,
  title: 'Source',
  status: 'development',
}

function requirement(overrides: Partial<ScrumProductRequirementNew> = {}): ScrumProductRequirementNew {
  return {
    id: 41,
    version: 7,
    team_id: 2,
    project_new_id: 9,
    version_task_new_id: 31,
    title: 'Requirement',
    priority: 'normal',
    status: 'development',
    ...overrides,
  }
}

function readyRuntime() {
  runtimeSessionStore.getState().clearSession()
  runtimeSessionStore.getState().setStandaloneToken('content-workflow-token')
  runtimeWorkspaceStore.getState().selectWorkspace(7)
}

function ticket() {
  return {
    workspaceId: runtimeWorkspaceStore.getState().currentWorkspaceId,
    sessionRevision: runtimeSessionStore.getState().revision,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((complete) => { resolve = complete })
  return { promise, resolve }
}

function setup() {
  return renderHook(() => useProjectContentWorkflow({
    route: { kind: 'plan-unit', projectId: 9, planUnitId: 31, tab: 'requirements' },
    project,
    unit: sourceUnit,
    users: [],
  }))
}

function selectMoveTarget(hook: ReturnType<typeof setup>, source: ScrumProductRequirementNew) {
  act(() => hook.result.current.requirements.openMove(source))
  act(() => {
    const state = hook.result.current.move
    if (state.status !== 'selecting') throw new Error('expected open move')
    hook.result.current.changeMoveDraft({ ...state.draft, targetPlanUnitId: 32 })
  })
}

describe('P7 requirement move stale recovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    readyRuntime()
    controlled.requirements = [requirement()]
    controlled.targetUnits = [
      sourceUnit,
      { id: 32, team_id: 2, project_new_id: 9, title: 'Target', status: 'planning' },
    ]
    controlled.reloadRequirements.mockImplementation(async () => controlled.requirements)
    controlled.move.mockResolvedValue({
      status: 'applied',
      data: requirement({ version: 10, version_task_new_id: 32 }),
      ticket: ticket(),
    })
  })

  it('reloads a same-scope latest row and retries with its exact version while preserving a valid target', async () => {
    controlled.move.mockRejectedValueOnce(new PMSafeApiError('REQUIREMENT_STALE', 409))
    const hook = setup()
    selectMoveTarget(hook, requirement())

    await act(async () => { await hook.result.current.submitMove(false) })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: { expectedVersion: 7, targetPlanUnitId: 32 },
      submit: { status: 'error', errorCode: 'REQUIREMENT_STALE' },
    })

    controlled.requirements = [requirement({ version: 9 })]
    hook.rerender()
    await act(async () => { await hook.result.current.reloadLatestMove() })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: {
        expectedVersion: 9,
        sourceProjectId: 9,
        sourcePlanUnitId: 31,
        targetProjectId: 9,
        targetPlanUnitId: 32,
      },
      submit: { status: 'idle' },
    })

    await act(async () => { await hook.result.current.submitMove(false) })
    expect(controlled.move).toHaveBeenLastCalledWith({
      sourceProjectId: 9,
      targetProjectId: 9,
      requirementId: 41,
      targetPlanUnitId: 32,
      expectedVersion: 9,
    })
  })

  it('keeps the stale state when the refreshed row has not advanced', async () => {
    controlled.move.mockRejectedValueOnce(new PMSafeApiError('REQUIREMENT_STALE', 409))
    const hook = setup()
    selectMoveTarget(hook, requirement())
    await act(async () => { await hook.result.current.submitMove(false) })

    hook.rerender()
    await act(async () => { await hook.result.current.reloadLatestMove() })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: { expectedVersion: 7, targetPlanUnitId: 32 },
      submit: { status: 'error', errorCode: 'REQUIREMENT_STALE' },
    })
    expect(controlled.move).toHaveBeenCalledTimes(1)
    expect(controlled.reloadRequirements).toHaveBeenCalledTimes(1)
  })

  it('keeps the frozen move draft when loading the latest row fails', async () => {
    controlled.move.mockRejectedValueOnce(new PMSafeApiError('REQUIREMENT_STALE', 409))
    controlled.reloadRequirements.mockRejectedValueOnce(new Error('refresh failed'))
    const hook = setup()
    selectMoveTarget(hook, requirement())
    await act(async () => { await hook.result.current.submitMove(false) })

    await act(async () => { await hook.result.current.reloadLatestMove() })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: { expectedVersion: 7, targetPlanUnitId: 32 },
      submit: { status: 'error', errorCode: 'REQUIREMENT_STALE' },
    })
    expect(controlled.move).toHaveBeenCalledTimes(1)
    expect(controlled.reloadRequirements).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['returns the latest A row', [requirement({ version: 9 })]],
    ['cannot find A', []],
  ])('ignores a late A reload that %s after a stale B panel replaces it', async (_label, latestRows) => {
    const reload = deferred<ScrumProductRequirementNew[]>()
    const requirementB = requirement({ id: 42, version: 5, title: 'Requirement B' })
    controlled.requirements = [requirement(), requirementB]
    controlled.move
      .mockRejectedValueOnce(new PMSafeApiError('REQUIREMENT_STALE', 409))
      .mockRejectedValueOnce(new PMSafeApiError('REQUIREMENT_STALE', 409))
    controlled.reloadRequirements.mockReturnValueOnce(reload.promise)
    const hook = setup()
    selectMoveTarget(hook, requirement())
    await act(async () => { await hook.result.current.submitMove(false) })

    let pendingReload!: Promise<void>
    act(() => { pendingReload = hook.result.current.reloadLatestMove() })
    act(() => hook.result.current.closeMove())
    selectMoveTarget(hook, requirementB)
    await act(async () => { await hook.result.current.submitMove(false) })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: { requirementId: 42, expectedVersion: 5, targetPlanUnitId: 32 },
      submit: { status: 'error', errorCode: 'REQUIREMENT_STALE' },
    })

    await act(async () => {
      reload.resolve(latestRows)
      await pendingReload
    })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: { requirementId: 42, expectedVersion: 5, targetPlanUnitId: 32 },
      submit: { status: 'error', errorCode: 'REQUIREMENT_STALE' },
    })
  })

  it.each([
    ['missing', []],
    ['moved out of scope', [requirement({ version: 9, project_new_id: 10, version_task_new_id: 40 })]],
  ])('fails closed when the latest row is %s', async (_label, latestRows) => {
    controlled.move.mockRejectedValueOnce(new PMSafeApiError('REQUIREMENT_STALE', 409))
    const hook = setup()
    selectMoveTarget(hook, requirement())
    await act(async () => { await hook.result.current.submitMove(false) })

    controlled.requirements = latestRows
    hook.rerender()
    await act(async () => { await hook.result.current.reloadLatestMove() })
    expect(hook.result.current.move).toMatchObject({
      status: 'selecting',
      draft: { expectedVersion: 7, targetPlanUnitId: null },
      submit: { status: 'error', errorCode: 'RESOURCE_CHANGED' },
    })
    await act(async () => { await hook.result.current.submitMove(false) })
    expect(controlled.move).toHaveBeenCalledTimes(1)
  })
})
