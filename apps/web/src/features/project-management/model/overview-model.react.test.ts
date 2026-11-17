import { QueryClient } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const apiMocks = (() => ({
  listRequirements: vi.fn(),
  listTasks: vi.fn(),
  listBugs: vi.fn(),
}))()

mock.module('../api', () => ({
  listScrumProductRequirementsNew: apiMocks.listRequirements,
  listScrumTasksNew: apiMocks.listTasks,
  listProjectOverviewBugs: apiMocks.listBugs,
}))

import { PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import {
  projectOverviewAggregateModel,
  projectOverviewQueryOptions,
  resolveProjectOverviewFactsState,
} from './index'

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'PM React',
  status: 'active',
  health_status: 'at_risk',
  health_reason: '交付存在依赖',
  risk_level: 'high',
  starts_at: '2026-07-01',
  due_at: '2026-08-01',
  progress: 50,
  completed_task_count: 1,
  total_task_count: 2,
}

const unit: ScrumVersionTaskNew = {
  id: 31,
  team_id: 2,
  project_new_id: 9,
  title: 'V1.0',
  type: 'version',
  status: 'development',
  health_status: 'on_track',
  risk_level: 'low',
  starts_at: '2026-07-01',
  due_at: '2026-08-10',
  progress: 40,
  delivery_nodes: [{
    label_client_key: 'design_freeze',
    label_name: '设计冻结',
    due_at: '2026-07-20',
    actual_completed_at: null,
  }],
}

function requirement(id = 41): ScrumProductRequirementNew {
  return {
    id,
    version: 1,
    team_id: 2,
    project_new_id: 9,
    version_task_new_id: 31,
    title: `需求 ${id}`,
    priority: 'urgent',
    status: 'development',
    workflow_instance_id: 91,
    workflow_overview: {
      instance_id: 91,
      status: 'running',
      current_node_names: ['产品评审'],
      current_nodes: [{ name: '产品评审', overdue: false }],
      completed_steps: 1,
      total_steps: 4,
    },
  }
}

function task(id = 51): ProjectWorkTask {
  return {
    id,
    team_id: 2,
    project_set_id: 9,
    project_id: 31,
    requirement_id: 41,
    kind: 'task',
    title: `任务 ${id}`,
    status: 'in_progress',
    due_at: '2026-07-20',
    assignee_id: null,
  }
}

describe('project overview aggregate Query', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.listRequirements.mockResolvedValue({ rows: [requirement()], total: 1 })
    apiMocks.listTasks.mockResolvedValue({ rows: [task()], total: 1 })
    apiMocks.listBugs.mockResolvedValue([])
  })

  it('owns one workspace/project/item key and loads both complete aggregates without N+1 reads', async () => {
    const options = projectOverviewQueryOptions(7, { projectId: 9, teamId: 2, itemId: 31 })
    expect(options.queryKey).toEqual([
      'pm', 'workspace', 7, 'projectSets', 'overview-aggregate',
      'project', 9, 'team', 2, 'item', 31,
    ] as never)

    const client = new QueryClient()
    const facts = await client.fetchQuery(options)

    expect(facts).toEqual({ requirements: [requirement()], tasks: [task()], bugs: [] })
    expect(apiMocks.listRequirements).toHaveBeenCalledTimes(1)
    expect(apiMocks.listTasks).toHaveBeenCalledTimes(1)
    expect(apiMocks.listBugs).toHaveBeenCalledWith({
      projectSetId: 9,
      planUnitId: 31,
    }, expect.any(AbortSignal))
    expect(apiMocks.listRequirements).toHaveBeenCalledWith({
      projectNewId: 9,
      versionTaskNewId: 31,
      limit: 500,
      offset: 0,
    }, expect.any(AbortSignal))
    expect(apiMocks.listTasks).toHaveBeenCalledWith({
      projectNewId: 9,
      versionTaskNewId: 31,
      limit: 500,
      offset: 0,
    }, expect.any(AbortSignal))
  })

  it('accepts the legacy team zero sentinel only with an exact workspace relation', async () => {
    const legacyRequirement = {
      ...requirement(),
      workspace_id: 7,
      team_id: 0,
    }
    const legacyTask = {
      ...task(),
      workspace_id: 7,
      team_id: 0,
    }
    apiMocks.listRequirements.mockResolvedValue({ rows: [legacyRequirement], total: 1 })
    apiMocks.listTasks.mockResolvedValue({ rows: [legacyTask], total: 1 })

    const client = new QueryClient()
    await expect(client.fetchQuery(projectOverviewQueryOptions(7, {
      projectId: 9,
      teamId: 0,
      projectWorkspaceId: 7,
      itemId: 31,
    }))).resolves.toEqual({
      requirements: [legacyRequirement],
      tasks: [legacyTask],
      bugs: [],
    })

    expect(() => projectOverviewQueryOptions(7, {
      projectId: 9,
      teamId: 0,
      projectWorkspaceId: 8,
      itemId: 31,
    })).toThrow(PMSafeApiError)
  })

  it.each([
    ['requirement team', { ...requirement(), team_id: 3 }, task()],
    ['requirement project', { ...requirement(), project_new_id: 10 }, task()],
    ['requirement item', { ...requirement(), version_task_new_id: 32 }, task()],
    ['task team', requirement(), { ...task(), team_id: 3 }],
    ['task project', requirement(), { ...task(), project_set_id: 10 }],
    ['task item', requirement(), { ...task(), project_id: 32 }],
  ])('fails closed when a %s row escapes the requested scope', async (_label, req, workTask) => {
    apiMocks.listRequirements.mockResolvedValue({ rows: [req], total: 1 })
    apiMocks.listTasks.mockResolvedValue({ rows: [workTask], total: 1 })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    await expect(client.fetchQuery(
      projectOverviewQueryOptions(7, { projectId: 9, teamId: 2, itemId: 31 }),
    )).rejects.toMatchObject({
      code: 'PROJECT_OVERVIEW_INCOMPLETE',
      status: 409,
    })
  })

  it('fails closed when a legacy team row escapes the current workspace', async () => {
    apiMocks.listRequirements.mockResolvedValue({
      rows: [{ ...requirement(), workspace_id: 8, team_id: 0 }],
      total: 1,
    })
    apiMocks.listTasks.mockResolvedValue({
      rows: [{ ...task(), workspace_id: 7, team_id: 0 }],
      total: 1,
    })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    await expect(client.fetchQuery(projectOverviewQueryOptions(7, {
      projectId: 9,
      teamId: 0,
      projectWorkspaceId: 7,
      itemId: 31,
    }))).rejects.toMatchObject({
      code: 'PROJECT_OVERVIEW_INCOMPLETE',
      status: 409,
    })
  })

  it('rejects incomplete or duplicated pagination instead of rendering partial facts', async () => {
    apiMocks.listRequirements.mockResolvedValue({ rows: [requirement()], total: 2 })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    await expect(client.fetchQuery(
      projectOverviewQueryOptions(7, { projectId: 9, teamId: 2, itemId: 31 }),
    )).rejects.toMatchObject({
      code: 'PROJECT_OVERVIEW_INCOMPLETE',
      status: 409,
    })
  })
})

describe('project overview aggregate state and model', () => {
  const retry = vi.fn()
  const facts = { requirements: [requirement()], tasks: [task()], bugs: [] }

  it('keeps stale facts only for transport/5xx failures and fails 403/404 closed', () => {
    expect(resolveProjectOverviewFactsState({
      status: 'error',
      facts,
      error: new PMSafeApiError('REQUIREMENTS_FAILED', 503),
      fetching: false,
      retry,
    })).toMatchObject({ status: 'ready', facts, staleError: { retry } })

    for (const status of [403, 404]) {
      expect(resolveProjectOverviewFactsState({
        status: 'error',
        facts,
        error: new PMSafeApiError('REQUIREMENTS_FAILED', status),
        fetching: false,
        retry,
      })).toEqual({ status: 'error', retry })
    }
  })

  it('renders a complete zero-value model when aggregate facts are empty', () => {
    const state = resolveProjectOverviewFactsState({
      status: 'success',
      facts: { requirements: [], tasks: [], bugs: [] },
      fetching: false,
      retry,
    })
    expect(state.status).toBe('empty')

    const model = projectOverviewAggregateModel({
      project: { ...project, health_status: 'on_track', risk_level: 'low' },
      units: [{ ...unit, delivery_nodes: [] }],
      facts: { requirements: [], tasks: [], bugs: [] },
      now: new Date('2026-07-01T00:00:00Z'),
    })
    expect(model.metrics).toHaveLength(5)
    expect(model.work).toEqual({
      requirements: { total: 0, done: 0, active: 0, backlog: 0, highPriority: 0 },
      tasks: { total: 0, done: 0, active: 0, todo: 0, cancelled: 0, unassigned: 0 },
      bugs: { open: 0, critical: 0, normal: 0, minor: 0 },
    })
    expect(model.delivery.totalNodes).toBe(0)
    expect(model.riskRequirements).toEqual([])
  })

  it('derives attention, delivery summary and work breakdown from the same facts', () => {
    const model = projectOverviewAggregateModel({
      project,
      unit,
      units: [unit],
      facts,
      now: new Date('2026-08-03T00:00:00Z'),
    })

    expect(model.metrics.map((metric) => metric.id)).toEqual([
      'status', 'progress', 'health', 'risk', 'dates',
    ])
    expect(model.attention.map((item) => item.target)).toEqual(expect.arrayContaining([
      'requirements', 'tasks', 'delivery',
    ]))
    expect(model.riskRequirements).toEqual([
      expect.objectContaining({
        id: 41,
        planUnitId: 31,
        severity: 'critical',
        overdueTasks: 1,
        progress: 0,
        dimensions: ['task_overdue', 'task_unassigned'],
      }),
    ])
    expect(model.riskRequirements[0]?.reasons).toEqual(expect.arrayContaining([
      '1 个未完成任务已超过截止日',
      '1 个未完成任务没有负责人',
    ]))
    expect(model.evidence).toEqual({ requirements: 1, tasks: 1, bugs: 0 })
    expect(model.delivery).toMatchObject({ totalNodes: 1, delayedNodes: 1 })
    expect(model.work.requirements).toMatchObject({ total: 1, active: 1, highPriority: 1 })
    expect(model.work.tasks).toMatchObject({ total: 1, active: 1, unassigned: 1 })
    expect(model.work.bugs.open).toBe(0)
  })

  it('keeps task execution statuses mutually exclusive and only flags unfinished unassigned work', () => {
    const tasks: ProjectWorkTask[] = [
      { ...task(51), status: 'draft', assignee_id: null },
      { ...task(52), status: 'in_dev', assignee_id: 7 },
      { ...task(53), status: 'pending_test', assignee_id: 7 },
      { ...task(54), status: 'completed', assignee_id: null },
      { ...task(55), status: 'cancelled', assignee_id: null },
      { ...task(56), status: 'duplicate', assignee_id: null },
      { ...task(57), status: 'doing', assignee_id: 7 },
      { ...task(58), status: 'blocked', assignee_id: 7 },
      { ...task(59), status: 'closed', assignee_id: null, due_at: '2026-06-01' },
    ]
    const model = projectOverviewAggregateModel({
      project,
      units: [unit],
      facts: { requirements: [requirement()], tasks, bugs: [] },
      now: new Date('2026-07-01T00:00:00Z'),
    })

    expect(model.work.tasks).toEqual({
      total: 9,
      todo: 1,
      active: 4,
      done: 2,
      cancelled: 2,
      unassigned: 1,
    })
    expect(
      model.work.tasks.todo + model.work.tasks.active + model.work.tasks.done + model.work.tasks.cancelled,
    ).toBe(model.work.tasks.total)
    expect(model.riskRequirements[0]).toEqual(expect.objectContaining({
      overdueTasks: 0,
      unassignedTasks: 1,
    }))
  })

  it('keeps workflow-only requirements out of the action list and surfaces open bugs', () => {
    const bound = { ...requirement(41) }
    delete bound.workflow_overview
    const unbound = { ...requirement(42), workflow_instance_id: undefined }
    delete unbound.workflow_overview

    const workflowOnly = projectOverviewAggregateModel({
      project,
      units: [unit],
      facts: { requirements: [bound, unbound], tasks: [], bugs: [] },
      now: new Date('2026-08-03T00:00:00Z'),
    })
    expect(workflowOnly.riskRequirements).toEqual([])

    const withBugs = projectOverviewAggregateModel({
      project,
      units: [unit],
      facts: {
        requirements: [bound],
        tasks: [],
        bugs: [{ requirementId: 41, severity: 'critical', count: 1 }],
      },
      now: new Date('2026-08-03T00:00:00Z'),
    })
    expect(withBugs.riskRequirements).toEqual([
      expect.objectContaining({
        id: 41,
        severity: 'critical',
        openBugs: 1,
        dimensions: ['bug_open'],
        reasons: [copy.projectManagement.overview.riskRequirements.openBugs(1)],
      }),
    ])
    expect(withBugs.evidence.bugs).toBe(1)
    expect(withBugs.work.bugs).toEqual({ open: 1, critical: 1, normal: 0, minor: 0 })
  })
})
