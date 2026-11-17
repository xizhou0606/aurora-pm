import { QueryClient } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const apiMocks = (() => ({
  listRequirements: vi.fn(),
  listTasks: vi.fn(),
}))()

mock.module('../api', () => ({
  listScrumProductRequirementsNew: apiMocks.listRequirements,
  listScrumTasksNew: apiMocks.listTasks,
}))

import { PMSafeApiError } from '@/shared/api'
import type {
  PlatformUser,
  ProjectWorkTask,
  ScrumProductRequirementNew,
  ScrumVersionTaskNew,
} from '../api'
import {
  buildRequirementPatch,
  buildTaskPatch,
  hasRequirementPatchChanges,
  mapRequirementCreateDraft,
  mapTaskCreateDraft,
  reduceContentDelete,
  reduceRequirementEditor,
  reduceRequirementMove,
  REQUIREMENT_ACTIONS,
  requirementDraftFromCreate,
  requirementDraftFromRequirement,
  requirementListQueryOptions,
  requirementActionAvailability,
  requirementCreateAvailable,
  requirementEditableFields,
  resolveRequirementListState,
  requirementMoveAvailable,
  taskActionAvailability,
  resolveTaskListState,
  taskDraftFromCreate,
  taskDraftFromTask,
  taskListQueryOptions,
  type ContentDeleteState,
  type RequirementEditorState,
  type RequirementMoveState,
} from './index'

const planUnit: ScrumVersionTaskNew = {
  id: 31,
  team_id: 2,
  project_new_id: 9,
  title: 'Release Search',
  status: 'development',
}

const requirement: ScrumProductRequirementNew = {
  id: 41,
  version: 7,
  team_id: 2,
  project_new_id: 9,
  version_task_new_id: 31,
  title: 'React requirements',
  description: 'Current description',
  product_doc_url: 'https://example.test/product/41',
  product_owner_user_id: 11,
  design_url: 'https://example.test/design/41',
  design_owner_user_id: 12,
  developer_user_ids: [13],
  acceptance_criteria: 'Current acceptance',
  priority: 'p1',
  status: 'backlog',
  task_count: 2,
  completed_task_count: 1,
  deliverable_overview: {
    requirement_document: true,
    design: true,
    test_cases: true,
  },
}

const task: ProjectWorkTask = {
  id: 51,
  team_id: 2,
  project_set_id: 9,
  project_id: 31,
  requirement_id: 41,
  kind: 'task',
  task_key: 'PM-51',
  title: 'Implement requirements',
  description: 'Current task description',
  status: 'draft',
  priority: 'normal',
  assignee_id: 13,
  estimated_minutes: 60,
  starts_at: null,
  due_at: null,
}

const users: PlatformUser[] = [
  { id: 11, username: 'owner', display_name: 'Product Owner' },
  { id: 13, username: 'developer', display_name: 'Developer' },
]

describe('P7 content action model', () => {
  it('enables business actions for existing resources without authorization metadata', () => {
    const actions = requirementActionAvailability(requirement)

    expect(actions).toMatchObject({
      update: true,
      delete: true,
      createTask: true,
      move: true,
    })
    expect(requirementEditableFields()).toEqual({
      content: true,
      design: true,
      priority: true,
      acceptance: true,
      owner: true,
      governance: true,
    })
    expect(requirementCreateAvailable()).toBe(false)
    expect(requirementMoveAvailable(requirement)).toBe(false)
    expect(taskActionAvailability()).toEqual({
      update: false,
      delete: false,
    })
  })
})

describe('P7 content form model', () => {
  it('includes the one-time workflow selection only in requirement creation', () => {
    expect(mapRequirementCreateDraft({
      ...requirementDraftFromCreate({ projectId: 9, planUnitId: 31 }),
      title: 'Workflow requirement',
    })).toEqual({
      ok: false,
      errors: { workflowTemplateId: 'INVALID_ID' },
    })

    expect(mapRequirementCreateDraft({
      ...requirementDraftFromCreate({ projectId: 9, planUnitId: 31 }),
      workflowTemplateId: 8,
      title: '  Workflow requirement  ',
    })).toMatchObject({
      ok: true,
      value: {
        version_task_new_id: 31,
        workflow_template_id: 8,
        title: 'Workflow requirement',
      },
    })
  })

  it('builds a changed-only requirement patch filtered by current field actions', () => {
    const draft = {
      ...requirementDraftFromRequirement(requirement),
      title: '  React requirements v2  ',
      designUrl: 'https://example.test/design/changed',
      priority: 'p2',
      businessStatus: 'done',
    }
    const result = buildRequirementPatch(
      draft,
      requirement,
      new Set([
        REQUIREMENT_ACTIONS.updateContent,
        REQUIREMENT_ACTIONS.updateGovernance,
      ]),
    )

    expect(result).toEqual({
      ok: true,
      value: {
        expected_version: 7,
        title: 'React requirements v2',
      },
    })
    expect(result.ok && result.value).not.toHaveProperty('status')
  })

  it('keeps the expected version without turning an unchanged draft into an update', () => {
    const result = buildRequirementPatch(
      requirementDraftFromRequirement(requirement),
      requirement,
      new Set(Object.values(REQUIREMENT_ACTIONS)),
    )

    expect(result).toEqual({
      ok: true,
      value: { expected_version: 7 },
    })
    expect(result.ok && hasRequirementPatchChanges(result.value)).toBe(false)
  })

  it('keeps task create kind fixed and emits only changed task fields', () => {
    const create = mapTaskCreateDraft({
      ...taskDraftFromCreate({ projectId: 9, planUnitId: 31, requirementId: 41 }),
      title: '  New PM task  ',
      estimatedHours: '1.5',
    })
    expect(create).toMatchObject({
      ok: true,
      value: {
        requirement_id: 41,
        kind: 'task',
        title: 'New PM task',
        estimated_minutes: 90,
      },
    })

    expect(buildTaskPatch({
      ...taskDraftFromTask(task),
      title: '  Implement requirements safely  ',
    }, task)).toEqual({
      ok: true,
      value: { title: 'Implement requirements safely' },
    })
  })
})

describe('P7 content workflow state', () => {
  it('keeps the frozen version and unsaved requirement draft after a conflict', () => {
    const draft = {
      ...requirementDraftFromRequirement(requirement),
      title: 'Unsaved conflict draft',
    }
    let state: RequirementEditorState = reduceRequirementEditor(
      { status: 'closed' },
      { type: 'open', draft },
    )
    state = reduceRequirementEditor(state, { type: 'submit' })
    state = reduceRequirementEditor(state, {
      type: 'submit-failed',
      errorCode: 'REQUIREMENT_VERSION_CONFLICT',
    })

    expect(state).toMatchObject({
      status: 'editing',
      draft: { title: 'Unsaved conflict draft', expectedVersion: 7 },
      submit: { status: 'error', errorCode: 'REQUIREMENT_VERSION_CONFLICT' },
    })
  })

  it('keeps validation, pending, error and success transitions discriminated', () => {
    let state: RequirementEditorState = { status: 'closed' }
    state = reduceRequirementEditor(state, {
      type: 'open',
      draft: requirementDraftFromCreate({ projectId: 9, planUnitId: 31 }),
    })
    state = reduceRequirementEditor(state, { type: 'submit' })
    expect(state.status === 'editing' && state.validationVisible).toBe(true)
    expect(state.status === 'editing' && state.submit.status).toBe('idle')

    if (state.status !== 'editing' || state.draft.mode !== 'create') throw new Error('expected create editing state')
    state = reduceRequirementEditor(state, {
      type: 'change',
      draft: { ...state.draft, workflowTemplateId: 8, title: 'Valid requirement' },
    })
    state = reduceRequirementEditor(state, { type: 'submit' })
    expect(state.status === 'editing' && state.submit.status).toBe('pending')
    expect(reduceRequirementEditor(state, { type: 'close' })).toBe(state)

    state = reduceRequirementEditor(state, {
      type: 'submit-failed',
      errorCode: 'REQUIREMENT_CREATE_FAILED',
    })
    expect(state.status === 'editing' && state.submit).toEqual({
      status: 'error',
      errorCode: 'REQUIREMENT_CREATE_FAILED',
    })
    state = reduceRequirementEditor(state, { type: 'submit' })
    state = reduceRequirementEditor(state, { type: 'submit-succeeded' })
    expect(state).toEqual({ status: 'closed' })
  })

  it('keeps the frozen move target and safe schedule after confirmation-required 409', () => {
    const draft = {
      requirementId: 41,
      expectedVersion: 7,
      title: 'React requirements',
      sourceProjectId: 9,
      sourcePlanUnitId: 31,
      targetProjectId: 10,
      targetPlanUnitId: 32,
    }
    let state: RequirementMoveState = reduceRequirementMove(
      { status: 'closed' },
      { type: 'open', draft },
    )
    state = reduceRequirementMove(state, { type: 'submit' })
    state = reduceRequirementMove(state, {
      type: 'submit-failed',
      errorCode: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
      schedule: {
        state: 'partial',
        can_confirm: true,
        suggested: {
          planned_start_at: '2026-08-10',
          planned_end_at: '2026-08-25',
        },
        conflicts: [],
      },
    })

    expect(state).toMatchObject({
      status: 'selecting',
      draft,
      submit: {
        status: 'error',
        errorCode: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
        schedule: { can_confirm: true },
      },
    })
  })

  it('rebases only an explicitly stale move and fails closed when the latest source is unavailable', () => {
    const draft = {
      requirementId: 41,
      expectedVersion: 7,
      title: 'React requirements',
      sourceProjectId: 9,
      sourcePlanUnitId: 31,
      targetProjectId: 9,
      targetPlanUnitId: 32,
    }
    let state: RequirementMoveState = reduceRequirementMove(
      { status: 'closed' },
      { type: 'open', draft },
    )
    state = reduceRequirementMove(state, { type: 'submit' })
    state = reduceRequirementMove(state, {
      type: 'submit-failed',
      errorCode: 'REQUIREMENT_STALE',
    })
    state = reduceRequirementMove(state, {
      type: 'reload-latest',
      draft: { ...draft, expectedVersion: 9 },
    })
    expect(state).toMatchObject({
      status: 'selecting',
      draft: { expectedVersion: 9, targetPlanUnitId: 32 },
      submit: { status: 'idle' },
    })

    state = reduceRequirementMove(state, { type: 'submit' })
    state = reduceRequirementMove(state, {
      type: 'submit-failed',
      errorCode: 'REQUIREMENT_STALE',
    })
    state = reduceRequirementMove(state, { type: 'reload-unavailable' })
    expect(state).toMatchObject({
      status: 'selecting',
      draft: { expectedVersion: 9, targetPlanUnitId: null },
      submit: { status: 'error', errorCode: 'RESOURCE_CHANGED' },
    })
  })

  it('does not rebase a confirmation-required move through the stale reload event', () => {
    const draft = {
      requirementId: 41,
      expectedVersion: 7,
      title: 'React requirements',
      sourceProjectId: 9,
      sourcePlanUnitId: 31,
      targetProjectId: 10,
      targetPlanUnitId: 32,
    }
    let state: RequirementMoveState = reduceRequirementMove(
      { status: 'closed' },
      { type: 'open', draft },
    )
    state = reduceRequirementMove(state, { type: 'submit' })
    state = reduceRequirementMove(state, {
      type: 'submit-failed',
      errorCode: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
    })
    const frozen = state
    expect(reduceRequirementMove(state, {
      type: 'reload-latest',
      draft: { ...draft, expectedVersion: 9 },
    })).toBe(frozen)
  })

  it('prevents delete close and duplicate submit while pending', () => {
    const target = {
      kind: 'task' as const,
      projectId: 9,
      planUnitId: 31,
      requirementId: 41,
      taskId: 51,
      title: task.title,
    }
    let state: ContentDeleteState = reduceContentDelete(
      { status: 'closed' },
      { type: 'open', target },
    )
    state = reduceContentDelete(state, { type: 'submit' })
    expect(state.status === 'confirming' && state.submit.status).toBe('pending')
    expect(reduceContentDelete(state, { type: 'close' })).toBe(state)
    expect(reduceContentDelete(state, { type: 'submit' })).toBe(state)
  })
})

describe('P7 content Query ownership', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.listRequirements.mockResolvedValue({ rows: [requirement], total: 1 })
    apiMocks.listTasks.mockResolvedValue({ rows: [task], total: 1 })
  })

  it('normalizes filters into workspace-isolated keys and forwards cancellation', async () => {
    const requirementOptions = requirementListQueryOptions(7, {
      projectNewId: 9,
      versionTaskNewId: 31,
      q: '  release  ',
      status: 'development',
      priority: 'p1',
      limit: 20,
      offset: 40,
    })
    const taskOptions = taskListQueryOptions(8, {
      projectNewId: 9,
      versionTaskNewId: 31,
      requirementNewId: 41,
      limit: 50,
      offset: 100,
    })

    expect(requirementOptions.queryKey).toEqual([
      'pm', 'workspace', 7, 'requirements', 'list',
      'project', 9, 'plan-unit', 31,
      {
        q: 'release',
        status: 'development',
        priority: 'p1',
        limit: 20,
        offset: 40,
      },
    ] as never)
    expect(taskOptions.queryKey).toEqual([
      'pm', 'workspace', 8, 'tasks', 'list',
      'project', 9, 'plan-unit', 31, 'requirement', 41,
      { limit: 50, offset: 100 },
    ] as never)
    expect(() => requirementListQueryOptions(0, {
      projectNewId: 9,
      versionTaskNewId: 31,
    })).toThrow(/positive workspace ID/)
    expect(() => taskListQueryOptions(8, {
      projectNewId: 0,
      versionTaskNewId: 31,
    })).toThrow(/positive ID/)

    const client = new QueryClient()
    await client.fetchQuery(requirementOptions)
    await client.fetchQuery(taskOptions)
    expect(apiMocks.listRequirements).toHaveBeenCalledWith(
      expect.objectContaining({ q: 'release', limit: 20, offset: 40 }),
      expect.any(AbortSignal),
    )
    expect(apiMocks.listTasks).toHaveBeenCalledWith(
      expect.objectContaining({ requirementNewId: 41, limit: 50, offset: 100 }),
      expect.any(AbortSignal),
    )
  })

  it.each([
    ['requirement workspace', { ...requirement, workspace_id: 8 }, task],
    ['requirement project', { ...requirement, project_new_id: 10 }, task],
    ['requirement plan unit', { ...requirement, version_task_new_id: 32 }, task],
    ['task workspace', requirement, { ...task, workspace_id: 9 }],
    ['task project', requirement, { ...task, project_set_id: 10 }],
    ['task plan unit', requirement, { ...task, project_id: 32 }],
    ['task requirement', requirement, { ...task, requirement_id: 42 }],
  ])('fails closed when a %s row escapes the requested list scope', async (
    _label,
    requirementRow,
    taskRow,
  ) => {
    apiMocks.listRequirements.mockResolvedValue({ rows: [requirementRow], total: 1 })
    apiMocks.listTasks.mockResolvedValue({ rows: [taskRow], total: 1 })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    if (_label.startsWith('requirement')) {
      await expect(client.fetchQuery(requirementListQueryOptions(7, {
        projectNewId: 9,
        versionTaskNewId: 31,
      }))).rejects.toMatchObject({
        code: 'PROJECT_CONTENT_INCOMPLETE',
        status: 409,
      })
      await expect(client.fetchQuery(taskListQueryOptions(8, {
        projectNewId: 9,
        versionTaskNewId: 31,
        requirementNewId: 41,
      }))).resolves.toEqual({ rows: [task], total: 1 })
    } else {
      await expect(client.fetchQuery(requirementListQueryOptions(7, {
        projectNewId: 9,
        versionTaskNewId: 31,
      }))).resolves.toEqual({ rows: [requirement], total: 1 })
      await expect(client.fetchQuery(taskListQueryOptions(8, {
        projectNewId: 9,
        versionTaskNewId: 31,
        requirementNewId: 41,
      }))).rejects.toMatchObject({
        code: 'PROJECT_CONTENT_INCOMPLETE',
        status: 409,
      })
    }
  })
})

describe('P7 content view state', () => {
  const retry = vi.fn()

  it('resolves requirement loading, empty, error and ready as exclusive states', () => {
    const base = {
      data: undefined as { rows: ScrumProductRequirementNew[]; total: number } | undefined,
      page: 1,
      pageSize: 20,
      fetching: false,
      filtered: false,
      users,
      planUnitTitle: planUnit.title,
      retry,
    }
    expect(resolveRequirementListState({ ...base, status: 'pending' }))
      .toEqual({ status: 'loading' })
    expect(resolveRequirementListState({ ...base, status: 'error' }))
      .toEqual({ status: 'error', retry })
    expect(resolveRequirementListState({
      ...base,
      status: 'success',
      data: { rows: [], total: 0 },
    })).toEqual({ status: 'empty', reason: 'initial', refresh: { status: 'idle' } })
    expect(resolveRequirementListState({
      ...base,
      status: 'success',
      data: { rows: [], total: 0 },
      filtered: true,
    })).toEqual({ status: 'empty', reason: 'filtered', refresh: { status: 'idle' } })

    const ready = resolveRequirementListState({
      ...base,
      status: 'success',
      data: { rows: [requirement], total: 21 },
      page: 2,
      fetching: true,
    })
    expect(ready.status).toBe('ready')
    if (ready.status !== 'ready') throw new Error('expected ready requirements')
    expect(ready).toMatchObject({
      total: 21,
      page: 2,
      refresh: { status: 'refreshing' },
    })
    expect(ready.rows[0]).toMatchObject({
      id: 41,
      owner: { userId: 11, name: 'Product Owner', secondary: 'owner' },
      developers: [{ userId: 13, name: 'Developer', secondary: 'developer' }],
      planUnitTitle: 'Release Search',
      progress: { completed: 1, total: 2 },
      deliverables: { requirementDocument: true, design: true, testCases: true },
    })
  })

  it('resolves task loading, empty, error and ready without component-side derivation', () => {
    const base = {
      data: undefined as { rows: ProjectWorkTask[]; total: number } | undefined,
      page: 1,
      pageSize: 50,
      fetching: false,
      users,
      retry,
    }
    expect(resolveTaskListState({ ...base, status: 'pending' })).toEqual({ status: 'loading' })
    expect(resolveTaskListState({ ...base, status: 'error' })).toEqual({ status: 'error', retry })
    expect(resolveTaskListState({
      ...base,
      status: 'success',
      data: { rows: [], total: 0 },
    })).toEqual({ status: 'empty', refresh: { status: 'idle' } })

    const ready = resolveTaskListState({
      ...base,
      status: 'success',
      data: { rows: [task], total: 1 },
      fetching: true,
    })
    expect(ready.status).toBe('ready')
    if (ready.status !== 'ready') throw new Error('expected ready tasks')
    expect(ready).toMatchObject({
      total: 1,
      page: 1,
      refresh: { status: 'refreshing' },
    })
    expect(ready.rows[0]).toMatchObject({
      id: 51,
      key: 'PM-51',
      assignee: { userId: 13, name: 'Developer', secondary: 'developer' },
    })
  })

  it('preserves cached requirement and task data on transport or 5xx refresh failures', () => {
    const requirementState = resolveRequirementListState({
      status: 'error',
      data: { rows: [requirement], total: 1 },
      error: new PMSafeApiError('REQUIREMENT_REFRESH_FAILED', 503),
      page: 1,
      pageSize: 20,
      fetching: false,
      filtered: false,
      users,
      planUnitTitle: planUnit.title,
      retry,
    })
    expect(requirementState).toMatchObject({
      status: 'ready',
      rows: [{ id: requirement.id }],
      refresh: { status: 'stale_error', retry },
    })
    expect(resolveRequirementListState({
      status: 'error',
      data: { rows: [], total: 0 },
      error: new PMSafeApiError('REQUIREMENT_REFRESH_FAILED', 503),
      page: 1,
      pageSize: 20,
      fetching: false,
      filtered: true,
      users,
      planUnitTitle: planUnit.title,
      retry,
    })).toEqual({
      status: 'empty',
      reason: 'filtered',
      refresh: { status: 'stale_error', retry },
    })

    const taskState = resolveTaskListState({
      status: 'error',
      data: { rows: [task], total: 1 },
      error: new PMSafeApiError('NETWORK_UNAVAILABLE'),
      page: 1,
      pageSize: 20,
      fetching: false,
      users,
      retry,
    })
    expect(taskState).toMatchObject({
      status: 'ready',
      rows: [{ id: task.id }],
      refresh: { status: 'stale_error', retry },
    })
  })

  it.each([403, 404, 409])(
    'fails closed for cached content when refresh returns %s',
    (status) => {
      const error = new PMSafeApiError('CONTENT_SCOPE_REJECTED', status)
      expect(resolveRequirementListState({
        status: 'error',
        data: { rows: [requirement], total: 1 },
        error,
        page: 1,
        pageSize: 20,
        fetching: false,
        filtered: false,
        users,
        planUnitTitle: planUnit.title,
        retry,
      })).toEqual({ status: 'error', retry })
      expect(resolveTaskListState({
        status: 'error',
        data: { rows: [task], total: 1 },
        error,
        page: 1,
        pageSize: 20,
        fetching: false,
        users,
        retry,
      })).toEqual({ status: 'error', retry })
    },
  )

  it('fails closed for cached content on an unnormalized contract error', () => {
    const error = new Error('contract mismatch')
    expect(resolveRequirementListState({
      status: 'error',
      data: { rows: [requirement], total: 1 },
      error,
      page: 1,
      pageSize: 20,
      fetching: false,
      filtered: false,
      users,
      planUnitTitle: planUnit.title,
      retry,
    })).toEqual({ status: 'error', retry })
    expect(resolveTaskListState({
      status: 'error',
      data: { rows: [task], total: 1 },
      error,
      page: 1,
      pageSize: 20,
      fetching: false,
      users,
      retry,
    })).toEqual({ status: 'error', retry })
  })
})
