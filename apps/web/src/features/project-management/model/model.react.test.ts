import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { QueryClient } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const {
  boardRequest,
  auditRequest,
  listRequest,
  teamsRequest,
  usersRequest,
} = (() => ({
  boardRequest: vi.fn(),
  auditRequest: vi.fn(),
  listRequest: vi.fn(),
  teamsRequest: vi.fn(),
  usersRequest: vi.fn(),
}))()

mock.module('../api', () => ({
  getScrumBoardNew: boardRequest,
  listScrumProjectAuditLogsNew: auditRequest,
  listScrumProjectsNew: listRequest,
  listPlatformUsers: usersRequest,
  listWorkspaceTeams: teamsRequest,
}))

import {
  auditRowsForPlanUnit,
  auditDisplayRows,
  clampProgress,
  filterPlanUnits,
  healthLabel,
  normalizeProjectListInput,
  planUnitFromBoard,
  projectItemTypeLabel,
  projectAuditQueryOptions,
  projectBoardQueryOptions,
  projectFromSources,
  projectListQueryOptions,
  platformUsersQueryOptions,
  statusLabel,
  uniquePlanUnitOptions,
  workspaceTeamsQueryOptions,
} from './index'
import type { ScrumBoardNew, ScrumProjectNew } from '../api'

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'Project Nine',
  status: 'active',
}

const board: ScrumBoardNew = {
  project,
  version_tasks: [
    {
      id: 31,
      team_id: 2,
      project_new_id: 9,
      title: 'Release Search',
      goal: 'Search experience',
      type: 'project',
      owner_user_id: 11,
      status: 'development',
      health_status: 'at_risk',
      progress: 140,
    },
    {
      id: 32,
      team_id: 2,
      project_new_id: 9,
      title: 'Stable Release',
      type: 'version',
      owner_user_id: 12,
      status: 'released',
      health_status: 'on_track',
    },
  ],
  requirements: [],
  tasks: [],
}

describe('P5 project read query ownership', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listRequest.mockResolvedValue({ rows: [project], total: 1 })
    boardRequest.mockResolvedValue(board)
    auditRequest.mockResolvedValue({ rows: [], total: 0 })
    teamsRequest.mockResolvedValue([])
    usersRequest.mockResolvedValue([])
  })

  it('normalizes stable list scope and isolates every key by workspace', () => {
    expect(normalizeProjectListInput({ q: '  release  ', teamId: -1, limit: 0, offset: -2 }))
      .toEqual({ q: 'release', teamId: undefined, limit: 20, offset: 0 })
    expect(normalizeProjectListInput({ limit: Number.NaN, offset: 1.5 }))
      .toEqual({ q: undefined, teamId: undefined, limit: 20, offset: 0 })
    expect(projectListQueryOptions(7, { q: ' release ' }).queryKey).toEqual([
      'pm', 'workspace', 7, 'projectSets', 'list',
      { q: 'release', teamId: undefined, limit: 20, offset: 0 },
    ] as never)
    expect(projectBoardQueryOptions(8, 9).queryKey).toEqual([
      'pm', 'workspace', 8, 'planUnits', 'board', 9, 'summary-version-tasks',
    ] as never)
    expect(projectBoardQueryOptions(8, 9).retryOnMount).toBe(true)
    expect(projectAuditQueryOptions(8, 9).queryKey).toEqual([
      'pm', 'workspace', 8, 'audit', 'project', 9, { limit: 30, offset: 0 },
    ] as never)
    expect(platformUsersQueryOptions(8).queryKey).toEqual([
      'pm', 'workspace', 8, 'workspace', 'users',
    ] as never)
    expect(workspaceTeamsQueryOptions(8).queryKey).toEqual([
      'pm', 'workspace', 8, 'workspace', 'teams',
    ] as never)
    expect(() => projectBoardQueryOptions(8, 0)).toThrow(/positive ID/)
    expect(() => projectAuditQueryOptions(8, -1)).toThrow(/positive ID/)
  })

  it('forwards Query cancellation to the workspace directory adapter', async () => {
    const client = new QueryClient()

    await client.fetchQuery(platformUsersQueryOptions(7))
    await client.fetchQuery(workspaceTeamsQueryOptions(7))

    expect(usersRequest).toHaveBeenCalledWith(expect.any(AbortSignal))
    expect(teamsRequest).toHaveBeenCalledWith(expect.any(AbortSignal))
  })

  it('deduplicates concurrent loader consumers and reuses fresh component data', async () => {
    const client = new QueryClient()
    const options = projectBoardQueryOptions(7, 9)
    const [first, second] = await Promise.all([
      client.ensureQueryData(options),
      client.ensureQueryData(options),
    ])
    const mounted = await client.ensureQueryData(options)

    expect(boardRequest).toHaveBeenCalledTimes(1)
    expect(first).toBe(second)
    expect(mounted).toBe(first)
    expect(boardRequest).toHaveBeenCalledWith(
      9,
      ['summary', 'version_tasks'],
      expect.any(AbortSignal),
    )
  })
})

describe('P5 project read selectors', () => {
  it('fails closed on project and plan-unit parent mismatches', () => {
    expect(projectFromSources(9, board, [])).toBe(project)
    expect(projectFromSources(10, board, [{ ...project, id: 10 }])).toBeUndefined()
    expect(planUnitFromBoard(board, 31, 9)?.title).toBe('Release Search')
    expect(planUnitFromBoard(board, 31, 10)).toBeUndefined()
    expect(planUnitFromBoard(board, 99, 9)).toBeUndefined()
  })

  it('owns filters, option uniqueness and safe formatter fallbacks', () => {
    expect(filterPlanUnits(board.version_tasks, {
      search: 'search',
      status: 'development',
      health: 'at_risk',
    })).toEqual([board.version_tasks[0]])
    expect(uniquePlanUnitOptions(board.version_tasks, 'status'))
      .toEqual(['development', 'released'])
    expect(clampProgress(-5)).toBe(0)
    expect(clampProgress(140)).toBe(100)
    expect(statusLabel('not-real')).toBe(statusLabel('unknown'))
    expect(healthLabel('not-real')).toBe(healthLabel('unknown'))
    expect(projectItemTypeLabel('project')).toBe('项目')
    expect(projectItemTypeLabel('version')).toBe('版本')
    expect(filterPlanUnits(board.version_tasks, {
      search: '版本',
      status: 'all',
      health: 'all',
    })).toEqual([board.version_tasks[1]])
    expect(filterPlanUnits(board.version_tasks, {
      search: 'Teo',
      status: 'all',
      health: 'all',
    }, [
      { id: 11, username: 'teo', display_name: 'Teo' },
      { id: 12, username: 'release-owner', display_name: 'Release Owner' },
    ])).toEqual([board.version_tasks[0]])
  })

  it('scopes item activity through version, requirement and task ownership', () => {
    const rows = [
      { id: 1, resource: 'version', resource_id: 31 },
      { id: 2, resource: 'requirement', resource_id: 51 },
      { id: 3, resource: 'task', resource_id: 61 },
      { id: 4, resource: 'task', resource_id: 62 },
      { id: 5, resource: 'requirement', resource_id: 52 },
      { id: 6, resource: 'task', resource_id: 63 },
      { id: 7, resource: 'version', resource_id: 32 },
      { id: 8, resource: 'project', resource_id: 9 },
    ]
    expect(auditRowsForPlanUnit(rows, 31, {
      requirements: [
        { id: 51, version: 1, team_id: 2, project_new_id: 9, version_task_new_id: 31, title: 'Scoped', status: 'development' },
        { id: 52, version: 1, team_id: 2, project_new_id: 9, version_task_new_id: 32, title: 'Other', status: 'backlog' },
      ],
      tasks: [
        { id: 61, team_id: 2, project_set_id: 9, project_id: 31, requirement_id: 0, kind: 'task', title: 'Direct', status: 'in_dev' },
        { id: 62, team_id: 2, project_set_id: 9, project_id: 32, requirement_id: 51, kind: 'task', title: 'Via requirement', status: 'in_dev' },
        { id: 63, team_id: 2, project_set_id: 9, project_id: 32, requirement_id: 52, kind: 'task', title: 'Other', status: 'draft' },
      ],
    })).toEqual(rows.slice(0, 4))
    expect(auditDisplayRows([
      { id: 9, user_id: 11, action: 'update', resource: 'version' },
    ], [
      { id: 11, username: 'teo', display_name: 'Teo' },
    ])[0].actor.name).toBe('Teo')
  })

  it('keeps P6 mutations inside focused hooks and out of business components', () => {
    const root = join(process.cwd(), 'src/features/project-management')
    const files = ['components', 'routes']
      .flatMap((directory) => readdirSync(join(root, directory), { withFileTypes: true })
        .filter((entry) => entry.isFile() && /\.(?:ts|tsx)$/.test(entry.name))
        .filter((entry) => !/\.(?:react-)?test\./.test(entry.name))
        .map((entry) => join(root, directory, entry.name)))
    const source = files.map((path) => readFileSync(path, 'utf8')).join('\n')

    expect(source).not.toMatch(/\b(create|patch|delete|transfer)Scrum/)
    expect(source).not.toMatch(/method:\s*['"](?:post|patch|delete)['"]/i)
    expect(source).not.toMatch(/\buseMutation\b/)
    expect(source).not.toMatch(/\bmutationFn\b/)
    expect(source).not.toMatch(/\buseQueryClient\b|\binvalidateQueries\b|\bremoveQueries\b/)
    expect(source).not.toMatch(/platformApi\.(?:post|patch|delete)\b/)
  })
})
