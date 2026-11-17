import * as actualRouter from '@tanstack/react-router'
import { useEffect } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import type { ScrumProductRequirementNew, ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import type { ProjectScreenState } from '../model'
import { useProjectWriteWorkflow } from './useProjectWriteWorkflow'

const mocks = (() => ({
  navigate: vi.fn(),
  createProject: vi.fn(),
  updateProject: vi.fn(),
  deleteProject: vi.fn(),
  createPlanUnit: vi.fn(),
  updatePlanUnit: vi.fn(),
  deletePlanUnit: vi.fn(),
  finalizeDeleteProject: vi.fn(),
  finalizeDeletePlanUnit: vi.fn(),
  teamRefetch: vi.fn(),
  teamMemberQueryHook: vi.fn(),
  boardQueryHook: vi.fn(),
  boardRefetch: vi.fn(),
  overviewQueryHook: vi.fn(),
  overviewFacts: undefined as { requirements: ScrumProductRequirementNew[]; tasks: [] } | undefined,
  teams: [{ id: 3, name: 'Delivery team' }],
  teamMemberQueries: [] as Array<{
    data?: Array<{ id: number; username: string; role?: string }>
    isPending: boolean
    isError: boolean
    refetch: ReturnType<typeof vi.fn>
  }>,
}))()

// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  useNavigate: () => mocks.navigate,
}))

mock.module('../model', () => ({
    useWorkspaceTeamsQuery: () => ({
      data: mocks.teams,
      isPending: false,
      isError: false,
      refetch: mocks.teamRefetch,
    }),
    usePublishedPlanUnitWorkflowTemplatesQuery: () => ({
      data: [{ id: 8, name: '通用交付流程', published_version: 2 }],
      isPending: false,
      isError: false,
      refetch: vi.fn(),
    }),
    useProjectBoardQuery: (projectId: number, enabled: boolean) => {
      mocks.boardQueryHook(projectId, enabled)
      return { refetch: mocks.boardRefetch }
    },
    useProjectOverviewQuery: (input: unknown, enabled: boolean) => {
      mocks.overviewQueryHook(input, enabled)
      return {
        data: enabled ? mocks.overviewFacts : undefined,
        isPending: enabled && !mocks.overviewFacts,
        isError: false,
        isFetching: false,
        refetch: vi.fn(),
      }
    },
    useWorkspaceTeamMembersQueries: (teamIds: number[], enabled: boolean) => {
      mocks.teamMemberQueryHook(teamIds, enabled)
      return mocks.teamMemberQueries
    },
  }))

vi.mock('./useOwnerDirectoryModel', () => ({
  useOwnerDirectoryModel: () => ({ status: 'empty' as const }),
}))

vi.mock('./usePlanUnitWriteDelivery', () => {
  return {
    usePlanUnitWriteDelivery: (input: {
      editor: {
        status: string
        draft?: { mode: string }
        deliveryCatalog?: { status: string }
      }
      dispatch: (event: { type: 'delivery-loaded'; labels: [] }) => void
    }) => {
      useEffect(() => {
        if (
          input.editor.status === 'editing' &&
          input.editor.draft?.mode === 'update' &&
          input.editor.deliveryCatalog?.status === 'loading'
        ) input.dispatch({ type: 'delivery-loaded', labels: [] })
      }, [input.dispatch, input.editor])
      return {
        busy: false,
        labelManager: { status: 'closed' as const },
        catalog: { status: 'loading' as const, writable: false },
        targetTitle: undefined,
        reset: vi.fn(),
        retry: vi.fn(),
        addNode: vi.fn(),
        removeNode: vi.fn(),
        changeNode: vi.fn(),
        openLabelManager: vi.fn(),
        closeLabelManager: vi.fn(),
        changeLabelName: vi.fn(),
        createLabel: vi.fn(),
        renameLabel: vi.fn(),
        deleteLabel: vi.fn(),
      }
    },
  }
})

vi.mock('./useProjectMutations', () => ({
  useCreateProjectMutation: () => ({ execute: mocks.createProject }),
  useUpdateProjectMutation: () => ({ execute: mocks.updateProject }),
  useDeleteProjectMutation: () => ({
    execute: mocks.deleteProject,
    finalize: mocks.finalizeDeleteProject,
  }),
  useCreatePlanUnitMutation: () => ({ execute: mocks.createPlanUnit }),
  useUpdatePlanUnitMutation: () => ({ execute: mocks.updatePlanUnit }),
  useDeletePlanUnitMutation: () => ({
    execute: mocks.deletePlanUnit,
    finalize: mocks.finalizeDeletePlanUnit,
  }),
  isPMMutationTicketCurrent: () => true,
}))

function readyRuntime() {
  runtimeSessionStore.getState().setStandaloneToken('local-mock-token')
  runtimeSessionStore.getState().setIdentity({
    id: 7,
    username: 'admin',
    role: 'super_admin',
  })
  runtimeWorkspaceStore.getState().selectWorkspace(7)
}

type ReadyProjectScreen = Omit<Extract<ProjectScreenState, { status: 'ready' }>, 'resource'> & {
  resource: Extract<Extract<ProjectScreenState, { status: 'ready' }>['resource'], { kind: 'project' }>
}

function scheduleEditorFixtures(): {
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  screenState: ReadyProjectScreen
} {
  const project: ScrumProjectNew = {
    id: 9,
    workspace_id: 7,
    team_id: 3,
    name: '交付平台',
    starts_at: '2026-08-01',
    due_at: '2026-08-31',
    status: 'active',
  }
  const unit: ScrumVersionTaskNew = {
    id: 31,
    workspace_id: 7,
    team_id: 3,
    project_new_id: 9,
    title: 'V1 发布',
    type: 'version',
    starts_at: '2026-08-10',
    due_at: '2026-08-20',
    risk_level: 'low',
    status: 'planning',
    requirement_count: 1,
  }
  const screenState: ReadyProjectScreen = {
    status: 'ready',
    refresh: { status: 'idle' },
    resource: {
      kind: 'project',
      projectId: project.id,
      project,
      planUnits: [unit],
      header: { title: project.name, description: '', meta: [] },
      content: { kind: 'items', rows: [unit], users: [] },
    },
  }
  return { project, unit, screenState }
}

describe('project-set create workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    readyRuntime()
    mocks.teams = [{ id: 3, name: 'Delivery team' }]
    mocks.overviewFacts = undefined
    mocks.boardRefetch.mockReset()
    mocks.boardRefetch.mockResolvedValue({ data: undefined })
    mocks.teamMemberQueries = [{
      data: [{ id: 7, username: 'admin', role: 'admin' }],
      isPending: false,
      isError: false,
      refetch: vi.fn(),
    }]
    mocks.createProject.mockResolvedValue({
      status: 'applied',
      data: { id: 42, team_id: 3, name: 'Created project set', status: 'active' },
      ticket: {
        workspaceId: 7,
        sessionRevision: runtimeSessionStore.getState().revision,
      },
    })
  })

  it('selects a created project set in the master list', async () => {
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'list' },
      screen: { status: 'select' },
      projects: [],
    }))

    act(() => result.current.openProjectCreate())
    await waitFor(() => expect(result.current.projectEditor.status).toBe('editing'))
    if (result.current.projectEditor.status !== 'editing') {
      throw new Error('project editor did not open')
    }
    const draft = result.current.projectEditor.draft
    act(() => result.current.changeProjectDraft({
      ...draft,
      name: 'Created project set',
    }))

    await act(async () => result.current.submitProject())

    expect(mocks.createProject).toHaveBeenCalledWith(expect.objectContaining({
      team_id: 3,
      name: 'Created project set',
    }))
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: '/workspace/projects',
      search: { project_id: 42 },
    })
    expect(result.current.feedback).toEqual({
      status: 'success',
      message: copy.projectManagement.write.feedback.projectCreated,
    })
    expect(result.current.projectEditor.status).toBe('closed' as never)
  })

  it('allows any workspace member to create in an available team', async () => {
    mocks.teamMemberQueries = [{
      data: [{ id: 7, username: 'admin', role: 'member' }],
      isPending: false,
      isError: false,
      refetch: vi.fn(),
    }]
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'list' },
      screen: { status: 'select' },
      projects: [],
    }))

    expect(result.current.projectActions().create).toBe(true)
    act(() => result.current.openProjectCreate())
    await waitFor(() => expect(result.current.projectEditor.status).toBe('editing'))
  })

  it('offers every workspace team without loading role assignments', () => {
    mocks.teams = [
      { id: 3, name: 'Member team' },
      { id: 4, name: 'Owner team' },
      { id: 5, name: 'Admin team' },
      { id: 6, name: 'Non-member team' },
    ]
    mocks.teamMemberQueries = [
      { data: [{ id: 7, username: 'admin', role: 'member' }], isPending: false, isError: false, refetch: vi.fn() },
      { data: [{ id: 7, username: 'admin', role: 'owner' }], isPending: false, isError: false, refetch: vi.fn() },
      { data: [{ id: 7, username: 'admin', role: 'admin' }], isPending: false, isError: false, refetch: vi.fn() },
      { data: [{ id: 9, username: 'other', role: 'admin' }], isPending: false, isError: false, refetch: vi.fn() },
    ]
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'list' },
      screen: { status: 'select' },
      projects: [],
    }))

    expect(result.current.teams).toEqual({
      status: 'ready',
      items: [
        { id: 3, label: 'Member team' },
        { id: 4, label: 'Owner team' },
        { id: 5, label: 'Admin team' },
        { id: 6, label: 'Non-member team' },
      ],
    })
    expect(result.current.projectActions().create).toBe(true)
    expect(mocks.teamMemberQueryHook).not.toHaveBeenCalled()
  })

  it('allows every workspace team for workspace managers without member requests', () => {
    readyRuntime()
    mocks.teams = [
      { id: 3, name: 'Delivery team' },
      { id: 4, name: 'Platform team' },
    ]
    mocks.teamMemberQueries = []
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'list' },
      screen: { status: 'select' },
      projects: [],
    }))

    expect(result.current.teams).toEqual({
      status: 'ready',
      items: [
        { id: 3, label: 'Delivery team' },
        { id: 4, label: 'Platform team' },
      ],
    })
    expect(result.current.projectActions().create).toBe(true)
    expect(mocks.teamMemberQueryHook).not.toHaveBeenCalled()
  })

  it('keeps the editor open and refreshes teams after a backend rejection', async () => {
    const memberRefetch = vi.fn()
    mocks.teamMemberQueries = [{
      data: [{ id: 7, username: 'admin', role: 'admin' }],
      isPending: false,
      isError: false,
      refetch: memberRefetch,
    }]
    mocks.createProject.mockRejectedValueOnce(
      new PMSafeApiError('PROJECT_CREATION_PERMISSION_REQUIRED', 403),
    )
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'list' },
      screen: { status: 'select' },
      projects: [],
    }))

    act(() => result.current.openProjectCreate())
    await waitFor(() => expect(result.current.projectEditor.status).toBe('editing'))
    if (result.current.projectEditor.status !== 'editing') {
      throw new Error('project editor did not open')
    }
    const draft = result.current.projectEditor.draft
    act(() => result.current.changeProjectDraft({
      ...draft,
      name: 'Forbidden project set',
    }))
    await act(async () => result.current.submitProject())

    expect(result.current.projectEditor.status).toBe('editing')
    expect(result.current.projectEditor.status === 'editing'
      ? result.current.projectEditor.submit
      : null).toEqual({
        status: 'error',
        errorCode: 'RESOURCE_CHANGED',
      })
    expect(result.current.feedback).toEqual({
      status: 'error',
      message: copy.projectManagement.write.validation.contextChanged,
    })
    expect(mocks.teamRefetch).toHaveBeenCalledTimes(1)
    expect(memberRefetch).not.toHaveBeenCalled()
    expect(mocks.navigate).not.toHaveBeenCalled()
  })

  it('rebases a project editor onto fresh null dates before retrying a stale schedule update', async () => {
    const { project, unit, screenState } = scheduleEditorFixtures()
    const latestProject = {
      ...project,
      description: 'External description',
      starts_at: null,
      due_at: '2026-09-01T12:00:00Z',
    }
    mocks.boardRefetch.mockResolvedValueOnce({
      data: { project: latestProject, version_tasks: [unit], requirements: [], tasks: [] },
    })
    mocks.updateProject
      .mockRejectedValueOnce(new PMSafeApiError(
        'SCHEDULE_SUBJECT_STALE',
        409,
        false,
        undefined,
        { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
      ))
      .mockResolvedValueOnce({
        status: 'applied',
        data: latestProject,
        ticket: { workspaceId: 7, sessionRevision: runtimeSessionStore.getState().revision },
      })
    const hook = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      screen: screenState,
      projects: [project],
    }))

    act(() => hook.result.current.openProjectEdit(project))
    if (hook.result.current.projectEditor.status !== 'editing') throw new Error('project editor did not open')
    const projectDraft = hook.result.current.projectEditor.draft
    act(() => hook.result.current.changeProjectDraft({
      ...projectDraft,
      name: 'Local project name',
      plannedEndAt: '2026-09-05',
    }))
    await act(async () => hook.result.current.submitProject())

    await act(async () => hook.result.current.reloadProjectEditorSchedule())
    expect(mocks.boardRefetch).toHaveBeenCalledWith({ throwOnError: true })
    expect(hook.result.current.projectEditor.status).toBe('editing')
    if (hook.result.current.projectEditor.status !== 'editing') throw new Error('project editor closed')
    expect(hook.result.current.projectEditor.draft).toMatchObject({
      name: 'Local project name',
      description: 'External description',
      plannedStartAt: '',
      plannedEndAt: '2026-09-05',
    })

    await act(async () => hook.result.current.submitProject())
    expect(mocks.updateProject.mock.calls[1]).toEqual([9, expect.objectContaining({
      name: 'Local project name',
      description: 'External description',
      planned_end_at: '2026-09-05',
      expected_planned_end_at: '2026-09-01T12:00:00Z',
    })])
    expect(mocks.updateProject.mock.calls[1]?.[1]).not.toHaveProperty('planned_start_at')
  })

  it('rebases a plan-unit editor onto fresh null and actual dates before a stale retry', async () => {
    const { project, unit, screenState } = scheduleEditorFixtures()
    const latestUnit = {
      ...unit,
      starts_at: null,
      due_at: '2026-09-01T12:00:00Z',
      actual_start_at: null,
      actual_completed_at: '2026-09-02T12:00:00Z',
    }
    mocks.boardRefetch.mockResolvedValueOnce({
      data: { project, version_tasks: [latestUnit], requirements: [], tasks: [] },
    })
    mocks.updatePlanUnit
      .mockRejectedValueOnce(new PMSafeApiError(
        'SCHEDULE_SUBJECT_STALE',
        409,
        false,
        undefined,
        { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
      ))
      .mockResolvedValueOnce({
        status: 'applied',
        data: latestUnit,
        ticket: { workspaceId: 7, sessionRevision: runtimeSessionStore.getState().revision },
      })
    const hook = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      screen: screenState,
      projects: [project],
    }))

    act(() => hook.result.current.openPlanUnitEdit(project, unit))
    await waitFor(() => expect(hook.result.current.planUnitEditor.status === 'editing' && hook.result.current.planUnitEditor.deliveryCatalog.status).toBe('ready'))
    if (hook.result.current.planUnitEditor.status !== 'editing') throw new Error('plan-unit editor did not open')
    const planUnitDraft = hook.result.current.planUnitEditor.draft
    act(() => hook.result.current.changePlanUnitDraft({
      ...planUnitDraft,
      title: 'Local unit title',
      plannedEndAt: '2026-09-05',
    }))
    await act(async () => hook.result.current.submitPlanUnit())

    await act(async () => hook.result.current.reloadPlanUnitEditorSchedule())
    expect(mocks.boardRefetch).toHaveBeenCalledWith({ throwOnError: true })
    expect(hook.result.current.planUnitEditor.status).toBe('editing')
    if (hook.result.current.planUnitEditor.status !== 'editing') throw new Error('plan-unit editor closed')
    expect(hook.result.current.planUnitEditor.draft).toMatchObject({
      title: 'Local unit title',
      plannedStartAt: '',
      plannedEndAt: '2026-09-05',
      actualStartAt: '',
      actualCompletedAt: '2026-09-02',
    })

    await act(async () => hook.result.current.submitPlanUnit())
    expect(mocks.updatePlanUnit.mock.calls[1]).toEqual([9, 31, expect.objectContaining({
      title: 'Local unit title',
      planned_end_at: '2026-09-05',
      expected_planned_end_at: '2026-09-01T12:00:00Z',
    })])
    expect(mocks.updatePlanUnit.mock.calls[1]?.[2]).not.toHaveProperty('planned_start_at')
    expect(mocks.updatePlanUnit.mock.calls[1]?.[2]).not.toHaveProperty('actual_start_at')
    expect(mocks.updatePlanUnit.mock.calls[1]?.[2]).not.toHaveProperty('actual_completed_at')
  })

  it('keeps a stale project draft after failed reads and unlocks on an authoritative ABA baseline', async () => {
    const { project, unit, screenState } = scheduleEditorFixtures()
    mocks.updateProject
      .mockRejectedValueOnce(new PMSafeApiError(
        'SCHEDULE_SUBJECT_STALE',
        409,
        false,
        undefined,
        { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
      ))
      .mockResolvedValueOnce({
        status: 'applied',
        data: project,
        ticket: { workspaceId: 7, sessionRevision: runtimeSessionStore.getState().revision },
      })
    mocks.boardRefetch
      .mockRejectedValueOnce(new Error('board unavailable'))
      .mockResolvedValueOnce({ data: undefined })
      .mockResolvedValueOnce({
        data: { project, version_tasks: [unit], requirements: [], tasks: [] },
      })
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      screen: screenState,
      projects: [project],
    }))

    act(() => result.current.openProjectEdit(project))
    if (result.current.projectEditor.status !== 'editing') throw new Error('project editor did not open')
    const draft = result.current.projectEditor.draft
    act(() => result.current.changeProjectDraft({
      ...draft,
      name: 'Local project name',
      plannedEndAt: '2026-09-05',
    }))
    await act(async () => result.current.submitProject())

    act(() => result.current.changeProjectDraft({
      ...draft,
      name: 'Bypass stale project',
      plannedEndAt: '2026-10-01',
    }))
    await act(async () => result.current.submitProject())
    expect(mocks.updateProject).toHaveBeenCalledTimes(1)
    expect(result.current.projectEditor).toMatchObject({
      status: 'editing',
      draft: { name: 'Local project name', plannedEndAt: '2026-09-05' },
      submit: { status: 'error', errorCode: 'SCHEDULE_SUBJECT_STALE' },
    })

    await act(async () => Promise.all([
      result.current.reloadProjectEditorSchedule(),
      result.current.reloadProjectEditorSchedule(),
    ]))
    expect(mocks.boardRefetch).toHaveBeenCalledTimes(1)
    expect(result.current.projectEditor).toMatchObject({
      status: 'editing',
      draft: { name: 'Local project name', plannedEndAt: '2026-09-05' },
      submit: { status: 'error', errorCode: 'SCHEDULE_SUBJECT_STALE' },
    })

    await act(async () => result.current.reloadProjectEditorSchedule())
    expect(mocks.boardRefetch).toHaveBeenCalledTimes(2)
    expect(result.current.projectEditor).toMatchObject({
      status: 'editing',
      draft: { name: 'Local project name', plannedEndAt: '2026-09-05' },
      submit: { status: 'error', errorCode: 'SCHEDULE_SUBJECT_STALE' },
    })

    await act(async () => result.current.reloadProjectEditorSchedule())
    expect(mocks.boardRefetch).toHaveBeenCalledTimes(3)
    expect(result.current.projectEditor).toMatchObject({
      status: 'editing',
      draft: { name: 'Local project name', plannedEndAt: '2026-09-05' },
      submit: { status: 'idle' },
    })
    await act(async () => result.current.submitProject())
    expect(mocks.updateProject.mock.calls[1]).toEqual([9, expect.objectContaining({
      planned_end_at: '2026-09-05',
      expected_planned_end_at: '2026-08-31',
    })])
  })

  it('keeps a stale plan-unit draft after refetch rejection and unlocks on an authoritative ABA baseline', async () => {
    const { project, unit, screenState } = scheduleEditorFixtures()
    mocks.updatePlanUnit
      .mockRejectedValueOnce(new PMSafeApiError(
        'SCHEDULE_SUBJECT_STALE',
        409,
        false,
        undefined,
        { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
      ))
      .mockResolvedValueOnce({
        status: 'applied',
        data: unit,
        ticket: { workspaceId: 7, sessionRevision: runtimeSessionStore.getState().revision },
      })
    mocks.boardRefetch
      .mockRejectedValueOnce(new Error('board unavailable'))
      .mockResolvedValueOnce({
        data: { project, version_tasks: [unit], requirements: [], tasks: [] },
      })
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      screen: screenState,
      projects: [project],
    }))

    act(() => result.current.openPlanUnitEdit(project, unit))
    await waitFor(() => expect(result.current.planUnitEditor.status === 'editing' && result.current.planUnitEditor.deliveryCatalog.status).toBe('ready'))
    if (result.current.planUnitEditor.status !== 'editing') throw new Error('plan-unit editor did not open')
    const draft = result.current.planUnitEditor.draft
    act(() => result.current.changePlanUnitDraft({
      ...draft,
      title: 'Local unit title',
      plannedEndAt: '2026-09-05',
    }))
    await act(async () => result.current.submitPlanUnit())

    act(() => result.current.changePlanUnitDraft({
      ...draft,
      title: 'Bypass stale unit',
      plannedEndAt: '2026-10-01',
    }))
    await act(async () => result.current.submitPlanUnit())
    expect(mocks.updatePlanUnit).toHaveBeenCalledTimes(1)
    expect(result.current.planUnitEditor).toMatchObject({
      status: 'editing',
      draft: { title: 'Local unit title', plannedEndAt: '2026-09-05' },
      submit: { status: 'error', errorCode: 'SCHEDULE_SUBJECT_STALE' },
    })

    await act(async () => result.current.reloadPlanUnitEditorSchedule())
    expect(result.current.planUnitEditor).toMatchObject({
      status: 'editing',
      draft: { title: 'Local unit title', plannedEndAt: '2026-09-05' },
      submit: { status: 'error', errorCode: 'SCHEDULE_SUBJECT_STALE' },
    })

    await act(async () => result.current.reloadPlanUnitEditorSchedule())
    expect(mocks.boardRefetch).toHaveBeenCalledTimes(2)
    expect(result.current.planUnitEditor).toMatchObject({
      status: 'editing',
      draft: { title: 'Local unit title', plannedEndAt: '2026-09-05' },
      submit: { status: 'idle' },
    })
    await act(async () => result.current.submitPlanUnit())
    expect(mocks.updatePlanUnit.mock.calls[1]).toEqual([9, 31, expect.objectContaining({
      planned_end_at: '2026-09-05',
      expected_planned_end_at: '2026-08-20',
    })])
  })

  it('resolves requirement titles through the real project editor controller after a schedule 409', async () => {
    const { project, screenState } = scheduleEditorFixtures()
    mocks.overviewFacts = {
      requirements: [{
        id: 42,
        version: 3,
        workspace_id: 7,
        team_id: 3,
        project_new_id: 9,
        version_task_new_id: 31,
        title: '支付需求',
        status: 'grooming',
      }],
      tasks: [],
    }
    mocks.updateProject.mockRejectedValueOnce(new PMSafeApiError(
      'WORKFLOW_SCHEDULE_CONFLICT',
      409,
      false,
      undefined,
      {
        state: 'conflict',
        conflicts: [{ code: 'NODE_AFTER_SUBJECT_END', subject_type: 'requirement', subject_id: 42, node_id: 'qa' }],
      },
    ))
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      screen: screenState,
      projects: [project],
    }))

    act(() => result.current.openProjectEdit(project))
    await act(async () => result.current.submitProject())

    await waitFor(() => expect(result.current.resolveScheduleSubjectName('requirement', 42)).toBe('支付需求'))
    expect(mocks.overviewQueryHook).toHaveBeenCalledWith({
      projectId: 9,
      teamId: 3,
      projectWorkspaceId: 7,
    }, true)
  })

  it('resolves requirement titles through the real plan-unit editor controller after a schedule 409', async () => {
    const { project, unit, screenState } = scheduleEditorFixtures()
    mocks.overviewFacts = {
      requirements: [{
        id: 43,
        version: 2,
        workspace_id: 7,
        team_id: 3,
        project_new_id: 9,
        version_task_new_id: 31,
        title: '会员需求',
        status: 'grooming',
      }],
      tasks: [],
    }
    mocks.updatePlanUnit.mockRejectedValueOnce(new PMSafeApiError(
      'WORKFLOW_SCHEDULE_CONFLICT',
      409,
      false,
      undefined,
      {
        state: 'conflict',
        conflicts: [{ code: 'NODE_BEFORE_SUBJECT_START', subject_type: 'requirement', subject_id: 43, node_id: 'review' }],
      },
    ))
    const { result } = renderHook(() => useProjectWriteWorkflow({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      screen: screenState,
      projects: [project],
    }))

    act(() => result.current.openPlanUnitEdit(project, unit))
    await waitFor(() => expect(result.current.planUnitEditor.status === 'editing' && result.current.planUnitEditor.deliveryCatalog.status).toBe('ready'))
    await act(async () => result.current.submitPlanUnit())

    await waitFor(() => expect(result.current.resolveScheduleSubjectName('requirement', 43)).toBe('会员需求'))
    expect(mocks.overviewQueryHook).toHaveBeenCalledWith({
      projectId: 9,
      teamId: 3,
      projectWorkspaceId: 7,
      itemId: 31,
    }, true)
  })
})
