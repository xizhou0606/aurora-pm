import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const {
  coreApiGet,
  getRuntimeToken,
  getRuntimeWorkspaceId,
  platformApiDelete,
  platformApiGet,
  platformApiPatch,
  platformApiPost,
} = (() => ({
  coreApiGet: vi.fn(),
  getRuntimeToken: vi.fn(),
  getRuntimeWorkspaceId: vi.fn(),
  platformApiDelete: vi.fn(),
  platformApiGet: vi.fn(),
  platformApiPatch: vi.fn(),
  platformApiPost: vi.fn(),
}))()

mock.module('@/shared/api', () => ({
  coreApi: { get: coreApiGet },
  platformApi: {
    delete: platformApiDelete,
    get: platformApiGet,
    patch: platformApiPatch,
    post: platformApiPost,
  },
}))

vi.mock('@/shared/runtime', () => ({
  getRuntimeToken,
  getRuntimeWorkspaceId,
}))

import {
  clearProjectRequirementScopeOptionsCache,
  createScrumProjectNew,
  createScrumVersionTaskNew,
  deleteScrumProjectNew,
  deleteScrumVersionTaskNew,
  getScrumBoardNew,
  listScrumProjectAuditLogsNew,
  listScrumProjectsNew,
  listPlatformUsers,
  listProjectRequirementScopeOptions,
  listScrumProductRequirementsNew,
  listWorkspaceTeamMembers,
  listWorkspaceTeams,
  patchScrumProjectNew,
  patchScrumVersionTaskNew,
} from './index'

describe('project requirement scope options cache', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    clearProjectRequirementScopeOptionsCache()
    getRuntimeToken.mockReturnValue('session-a')
    getRuntimeWorkspaceId.mockReturnValue(2)
    platformApiGet.mockResolvedValue({
      data: {
        data: {
          project_sets: [{ id: 1, team_id: 7, name: '项目集 A', code: 'A' }],
          projects: [{ id: 11, team_id: 7, project_new_id: 1, title: '项目 A1' }],
          requirements: [{ id: 101, team_id: 7, project_new_id: 1, version_task_new_id: 11, title: '需求 A1' }],
        },
      },
    })
  })

  it('deduplicates concurrent requests and reuses the workspace session cache', async () => {
    const [first, second] = await Promise.all([
      listProjectRequirementScopeOptions(),
      listProjectRequirementScopeOptions(),
    ])
    const third = await listProjectRequirementScopeOptions()

    expect(platformApiGet).toHaveBeenCalledTimes(1)
    expect(platformApiGet).toHaveBeenCalledWith('/project-requirement-scope-options')
    expect(second).toBe(first)
    expect(third).toBe(first)
    expect(first.requirements[0].id).toBe(101)
  })

  it('invalidates cached options after scoped data changes', async () => {
    await listProjectRequirementScopeOptions()
    clearProjectRequirementScopeOptionsCache()
    await listProjectRequirementScopeOptions()

    expect(platformApiGet).toHaveBeenCalledTimes(2)
  })

  it('separates cache entries by workspace and login session', async () => {
    await listProjectRequirementScopeOptions()
    getRuntimeWorkspaceId.mockReturnValue(3)
    await listProjectRequirementScopeOptions()
    getRuntimeToken.mockReturnValue('session-b')
    await listProjectRequirementScopeOptions()

    expect(platformApiGet).toHaveBeenCalledTimes(3)
  })
})

describe('project management deferred data APIs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    platformApiGet.mockResolvedValue({ data: { data: [], total: 0 } })
  })

  it('requests only summary and version rows for the initial board', async () => {
    await getScrumBoardNew(9, ['summary', 'version_tasks'])

    expect(platformApiGet).toHaveBeenCalledWith('/project-management-board', {
      params: {
        project_new_id: '9',
        include: 'summary,version_tasks',
      },
    })
  })

  it('forwards normalized list, audit pagination and cancellation signals', async () => {
    const controller = new AbortController()
    await listScrumProjectsNew(
      { teamId: 2, q: '  release ', limit: 20, offset: 0 },
      controller.signal,
    )
    await listScrumProjectAuditLogsNew(
      9,
      { limit: 30, offset: 10 },
      controller.signal,
    )

    expect(platformApiGet).toHaveBeenNthCalledWith(1, '/project-sets', {
      params: {
        limit: '20',
        offset: '0',
        sort: 'updated_at_desc',
        team_id: '2',
        q: 'release',
      },
      signal: controller.signal,
    })
    expect(platformApiGet).toHaveBeenNthCalledWith(2, '/project-sets/9/audit-logs', {
      params: { limit: '30', offset: '10' },
      signal: controller.signal,
    })
  })

  it('removes raw Axios request details before a read error reaches Query', async () => {
    platformApiGet.mockRejectedValueOnce({
      config: { headers: { Authorization: 'Bearer secret' } },
      response: { status: 503, data: { error: 'private upstream details' } },
    })

    const error = await listScrumProjectsNew({ limit: 20, offset: 0 })
      .catch((reason) => reason)

    expect(error).toMatchObject({
      name: 'PMSafeApiError',
      code: 'PROJECT_SETS_FAILED',
      status: 503,
    })
    expect(error).not.toHaveProperty('config')
    expect(JSON.stringify(error)).not.toContain('secret')
  })

  it('loads a paginated requirement page by project', async () => {
    await listScrumProductRequirementsNew({ projectNewId: 9, limit: 50, offset: 0 })

    expect(platformApiGet).toHaveBeenCalledWith('/product-requirements-new', {
      params: {
        project_new_id: '9',
        limit: '50',
        offset: '0',
      },
    })
  })

  it('loads platform users with a cancellable bounded request', async () => {
    const controller = new AbortController()
    coreApiGet.mockResolvedValueOnce({
      data: {
        data: [
          { id: 11, username: 'teo', display_name: 'Teo' },
          { id: 0, username: 'invalid' },
        ],
      },
    })

    await expect(listPlatformUsers(controller.signal)).resolves.toEqual([
      { id: 11, username: 'teo', display_name: 'Teo' },
    ])
    expect(coreApiGet).toHaveBeenCalledWith('/directory/users', {
      params: { limit: 500, offset: 0 },
      signal: controller.signal,
    })
  })

  it('keeps fetching directory pages until the reported total is covered', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const firstPage = Array.from({ length: 500 }, (_, index) => ({
      id: index + 1,
      username: `user-${index + 1}`,
    }))
    coreApiGet
      .mockResolvedValueOnce({ data: { data: { items: firstPage, total: 502 } } })
      .mockResolvedValueOnce({
        data: {
          data: {
            items: [
              { id: 501, username: 'user-501' },
              { id: 502, username: 'user-502' },
            ],
            total: 502,
          },
        },
      })

    try {
      await expect(listPlatformUsers()).resolves.toHaveLength(502)
      expect(coreApiGet).toHaveBeenNthCalledWith(1, '/directory/users', {
        params: { limit: 500, offset: 0 },
        signal: undefined,
      })
      expect(coreApiGet).toHaveBeenNthCalledWith(2, '/directory/users', {
        params: { limit: 500, offset: 500 },
        signal: undefined,
      })
      expect(warn).not.toHaveBeenCalled()
    } finally {
      warn.mockRestore()
    }
  })

  it('warns and stops at the directory page cap instead of failing silently', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    coreApiGet.mockImplementation(
      (_url: string, config?: { params?: { offset?: number } }) => {
        const offset = config?.params?.offset ?? 0
        return Promise.resolve({
          data: {
            data: {
              items: Array.from({ length: 500 }, (_, index) => ({
                id: offset + index + 1,
                username: `user-${offset + index + 1}`,
              })),
              total: 12_000,
            },
          },
        })
      },
    )

    try {
      await expect(listPlatformUsers()).resolves.toHaveLength(10_000)
      expect(coreApiGet).toHaveBeenCalledTimes(20)
      expect(coreApiGet).toHaveBeenLastCalledWith('/directory/users', {
        params: { limit: 500, offset: 9_500 },
        signal: undefined,
      })
      expect(warn).toHaveBeenCalledTimes(1)
      expect(String(warn.mock.calls[0]?.[1])).toContain('truncated')
      expect(String(warn.mock.calls[0]?.[1])).toContain('10000 of 12000')
    } finally {
      warn.mockRestore()
    }
  })

  it('always queries the workspace directory without a project or team scope', async () => {
    const controller = new AbortController()
    coreApiGet.mockResolvedValue({ data: { data: [] } })

    await listPlatformUsers(controller.signal)
    await listPlatformUsers()

    expect(coreApiGet).toHaveBeenNthCalledWith(1, '/directory/users', {
      params: { limit: 500, offset: 0 },
      signal: controller.signal,
    })
    expect(coreApiGet).toHaveBeenNthCalledWith(2, '/directory/users', {
      params: { limit: 500, offset: 0 },
      signal: undefined,
    })
  })

  it('removes raw Axios details from platform user errors', async () => {
    coreApiGet.mockRejectedValueOnce({
      config: { headers: { Authorization: 'Bearer user-secret' } },
      response: { status: 502, data: { error: 'private user details' } },
    })

    const error = await listPlatformUsers().catch((reason) => reason)

    expect(error).toMatchObject({
      name: 'PMSafeApiError',
      code: 'PLATFORM_USERS_FAILED',
      status: 502,
    })
    expect(error).not.toHaveProperty('config')
    expect(JSON.stringify(error)).not.toContain('user-secret')
  })

  it('loads workspace teams with cancellation and safe errors', async () => {
    const controller = new AbortController()
    coreApiGet.mockResolvedValueOnce({
      data: { data: [{ id: 3, name: 'Platform' }, { id: 0, name: 'Invalid' }] },
    })

    await expect(listWorkspaceTeams(controller.signal)).resolves.toEqual([
      { id: 3, name: 'Platform' },
    ])
    expect(coreApiGet).toHaveBeenCalledWith('/directory/teams', { signal: controller.signal })

    coreApiGet.mockRejectedValueOnce({
      config: { headers: { Authorization: 'Bearer team-secret' } },
      response: { status: 503, data: { error: 'private team details' } },
    })
    const error = await listWorkspaceTeams().catch((reason) => reason)
    expect(error).toMatchObject({
      name: 'PMSafeApiError',
      code: 'WORKSPACE_TEAMS_FAILED',
      status: 503,
    })
    expect(JSON.stringify(error)).not.toContain('team-secret')
  })

  it('loads authoritative team roles for project-create preflight', async () => {
    const controller = new AbortController()
    coreApiGet.mockResolvedValueOnce({
      data: {
        data: [
          { id: 7, username: 'teo', display_name: 'Teo', role: 'admin' },
          { id: 0, username: 'invalid', role: 'owner' },
        ],
      },
    })

    await expect(listWorkspaceTeamMembers(3, controller.signal)).resolves.toEqual([
      { id: 7, username: 'teo', display_name: 'Teo', role: 'admin' },
    ])
    expect(coreApiGet).toHaveBeenCalledWith('/directory/teams/3/members', {
      signal: controller.signal,
    })
  })
})

describe('P6 project and plan-unit mutation adapters', () => {
  const projectCreate = {
    team_id: 7,
    name: 'Project A',
    description: 'Description',
    owner_user_id: 11,
    planned_start_at: null,
    planned_end_at: null,
    status: 'active' as const,
  }
  const projectPatch = {
    name: 'Project B',
    description: '',
    planned_start_at: '2026-08-01',
    planned_end_at: '2026-08-20',
    status: 'archived' as const,
  }
  const planUnitCreate = {
    project_new_id: 21,
    title: 'Version A',
    type: 'version' as const,
    owner_user_id: null,
    planned_start_at: null,
    planned_end_at: null,
    actual_start_at: null,
    actual_completed_at: null,
    risk_level: 'low' as const,
    goal: '',
  }
  const planUnitPatch = {
    title: 'Version B',
    type: 'version' as const,
    owner_user_id: 11,
    planned_start_at: '2026-08-02',
    planned_end_at: '2026-08-22',
    actual_start_at: null,
    actual_completed_at: null,
    risk_level: 'high' as const,
    goal: 'Ship it',
  }

  beforeEach(() => {
    vi.clearAllMocks()
    clearProjectRequirementScopeOptionsCache()
  })

  it('uses the existing six endpoints and named request bodies', async () => {
    const createdProject = {
      id: 21,
      team_id: 7,
      name: 'Project A',
      status: 'active',
    }
    const updatedProject = { ...createdProject, name: 'Project B' }
    const createdPlanUnit = {
      id: 31,
      team_id: 7,
      project_new_id: 21,
      title: 'Version A',
      status: 'planning',
    }
    const updatedPlanUnit = { ...createdPlanUnit, title: 'Version B' }
    platformApiPost
      .mockResolvedValueOnce({ data: { data: createdProject } })
      .mockResolvedValueOnce({ data: { data: createdPlanUnit } })
    platformApiPatch
      .mockResolvedValueOnce({ data: { data: updatedProject } })
      .mockResolvedValueOnce({ data: { data: updatedPlanUnit } })
    platformApiDelete
      .mockResolvedValueOnce({ data: { data: { deleted: true } } })
      .mockResolvedValueOnce({ data: { data: { deleted: true } } })

    await expect(createScrumProjectNew(projectCreate)).resolves.toEqual(createdProject)
    await expect(patchScrumProjectNew(21, {
      ...projectPatch,
      team_id: 99,
      owner_user_id: 77,
    } as typeof projectPatch & { team_id: number; owner_user_id: number }))
      .resolves.toEqual(updatedProject)
    await expect(deleteScrumProjectNew(21)).resolves.toEqual({ deleted: true })
    await expect(createScrumVersionTaskNew(planUnitCreate)).resolves.toEqual(createdPlanUnit)
    await expect(patchScrumVersionTaskNew(31, planUnitPatch)).resolves.toEqual(updatedPlanUnit)
    await expect(deleteScrumVersionTaskNew(31)).resolves.toEqual({ deleted: true })

    expect(platformApiPost).toHaveBeenNthCalledWith(1, '/project-sets', projectCreate)
    expect(platformApiPatch).toHaveBeenNthCalledWith(1, '/project-sets/21', projectPatch)
    expect(platformApiDelete).toHaveBeenNthCalledWith(1, '/project-sets/21')
    expect(platformApiPost).toHaveBeenNthCalledWith(2, '/project-new', planUnitCreate)
    expect(platformApiPatch).toHaveBeenNthCalledWith(2, '/project-new/31', planUnitPatch)
    expect(platformApiPost.mock.calls[1]?.[1]).not.toHaveProperty('status')
    expect(platformApiPatch.mock.calls[1]?.[1]).not.toHaveProperty('status')
    expect(platformApiDelete).toHaveBeenNthCalledWith(2, '/project-new/31')
  })

  it('rejects contradictory success DTOs while preserving empty-body legacy acknowledgements', async () => {
    platformApiPost
      .mockResolvedValueOnce({ data: { data: { id: 0 } } })
      .mockResolvedValueOnce({ data: { data: { id: -1 } } })
    platformApiPatch
      .mockResolvedValueOnce({ data: { data: { id: 0 } } })
      .mockResolvedValueOnce({ data: { data: { id: 1.5 } } })
    platformApiDelete
      .mockResolvedValueOnce({ data: { data: { deleted: false } } })
      .mockResolvedValueOnce({ data: { data: { deleted: 'yes' } } })

    await expect(createScrumProjectNew(projectCreate)).rejects.toMatchObject({
      code: 'PROJECT_CREATE_FAILED',
    })
    await expect(patchScrumProjectNew(21, projectPatch)).rejects.toMatchObject({
      code: 'PROJECT_UPDATE_FAILED',
    })
    await expect(deleteScrumProjectNew(21)).rejects.toMatchObject({
      code: 'PROJECT_DELETE_FAILED',
    })
    await expect(createScrumVersionTaskNew(planUnitCreate)).rejects.toMatchObject({
      code: 'PLAN_UNIT_CREATE_FAILED',
    })
    await expect(patchScrumVersionTaskNew(31, planUnitPatch)).rejects.toMatchObject({
      code: 'PLAN_UNIT_UPDATE_FAILED',
    })
    await expect(deleteScrumVersionTaskNew(31)).rejects.toMatchObject({
      code: 'PLAN_UNIT_DELETE_FAILED',
    })

    platformApiPatch
      .mockResolvedValueOnce({ data: { data: {} } })
      .mockResolvedValueOnce({ data: { data: undefined } })
      .mockResolvedValueOnce({ data: { data: '' } })
      .mockResolvedValueOnce({ data: { data: '   ' } })
    platformApiDelete
      .mockResolvedValueOnce({ data: { data: {} } })
      .mockResolvedValueOnce({ data: { data: undefined } })
      .mockResolvedValueOnce({ data: { data: '' } })
      .mockResolvedValueOnce({ data: { data: '   ' } })

    await expect(patchScrumProjectNew(21, projectPatch)).resolves.toMatchObject({
      id: 21,
      name: projectPatch.name,
    })
    await expect(patchScrumVersionTaskNew(31, planUnitPatch)).resolves.toMatchObject({
      id: 31,
      title: planUnitPatch.title,
    })
    await expect(deleteScrumProjectNew(21)).resolves.toEqual({ deleted: true })
    await expect(deleteScrumVersionTaskNew(31)).resolves.toEqual({ deleted: true })
    await expect(patchScrumProjectNew(21, projectPatch)).resolves.toMatchObject({ id: 21 })
    await expect(patchScrumVersionTaskNew(31, planUnitPatch)).resolves.toMatchObject({ id: 31 })
    await expect(deleteScrumProjectNew(21)).resolves.toEqual({ deleted: true })
    await expect(deleteScrumVersionTaskNew(31)).resolves.toEqual({ deleted: true })
  })

  it('converts all six transport failures without exposing raw request details', async () => {
    const rawError = () => ({
      config: { headers: { Authorization: 'Bearer mutation-secret' } },
      response: { status: 502, data: { error: 'private mutation details' } },
    })
    const cases = [
      [platformApiPost, () => createScrumProjectNew(projectCreate), 'PROJECT_CREATE_FAILED'],
      [platformApiPatch, () => patchScrumProjectNew(21, projectPatch), 'PROJECT_UPDATE_FAILED'],
      [platformApiDelete, () => deleteScrumProjectNew(21), 'PROJECT_DELETE_FAILED'],
      [platformApiPost, () => createScrumVersionTaskNew(planUnitCreate), 'PLAN_UNIT_CREATE_FAILED'],
      [platformApiPatch, () => patchScrumVersionTaskNew(31, planUnitPatch), 'PLAN_UNIT_UPDATE_FAILED'],
      [platformApiDelete, () => deleteScrumVersionTaskNew(31), 'PLAN_UNIT_DELETE_FAILED'],
    ] as const

    for (const [request, action, code] of cases) {
      request.mockRejectedValueOnce(rawError())
      const error = await action().catch((reason) => reason)
      expect(error).toMatchObject({ name: 'PMSafeApiError', code, status: 502 })
      expect(error).not.toHaveProperty('config')
      expect(JSON.stringify(error)).not.toContain('mutation-secret')
    }
  })
})
