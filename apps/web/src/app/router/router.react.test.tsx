import { RouterProvider, createMemoryHistory } from '@tanstack/react-router'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterAll, beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { createAppRouter } from './router'
import { installSessionRouteLifecycle } from './lifecycle'
import { AppProviders } from '@/app/providers'
import { pmKeys, queryClient } from '@/app/query'
import { seedActionPermissions } from '@/features/access-control'
import { projectBoardQueryOptions } from '@/features/project-management/routes/loader'
import {
  requirementPoolQueryOptions,
  requirementScopeQueryOptions,
} from '@/features/ai-pm/routes/loader'
import {
  emptyRequirementPoolFilterValue,
  requirementPoolQueryFilters,
} from '@/features/ai-pm/utils/requirement-pool-filter-fields'
import { PM_ACTION_CODES, type PMActionCode } from '@/generated/access-catalog'
import { coreApi, platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { createNativeInternalNavigator } from '@/shared/navigation'
import {
  runtimeSessionStore,
  runtimeWorkspaceStore,
} from '@/shared/runtime'
import { seedWorkspacesForCurrentSession, type WorkspaceRouteResolution } from '@/features/workspace'
import {
  hasLegacyProjectItemsSearch,
  parseLoginSearch,
  parseProfileSearch,
  parseInboxSearch,
  parsePlanUnitSearch,
  parsePositiveRouteId,
  parseProjectSearch,
  parseWorkItemsSearch,
  parseWorkspaceSearch,
} from '@/shared/router'
import type { RouteAccess, RouteAccessSnapshot } from './guards'

const projectApi = (() => ({
  list: vi.fn(),
  board: vi.fn(),
  audit: vi.fn(),
  users: vi.fn(),
  requirements: vi.fn(),
  tasks: vi.fn(),
  taskDetail: vi.fn(),
  updateProject: vi.fn(),
}))()

const hostNavigation = (() => ({
  navigateToWorkItems: vi.fn(() => true),
}))()
const biScreenLoader = (() => ({
  loadLocalBiScreen: vi.fn(),
}))()
const originalCoreAdapter = coreApi.defaults.adapter
const originalPlatformAdapter = platformApi.defaults.adapter

afterAll(() => {
  coreApi.defaults.adapter = originalCoreAdapter
  platformApi.defaults.adapter = originalPlatformAdapter
})

mock.module('@/shared/runtime', () => ({
    navigateToWorkItems: hostNavigation.navigateToWorkItems,
  }))

mock.module('@/features/project-management/api', async () => ({
    listScrumProjectsNew: projectApi.list,
    getScrumBoardNew: projectApi.board,
    listScrumProjectAuditLogsNew: projectApi.audit,
    listPlatformUsers: projectApi.users,
    searchProjectPlatformUsers: async () => ({
      items: await projectApi.users(),
      total: 1,
    }),
    listScrumProductRequirementsNew: projectApi.requirements,
    listScrumTasksNew: projectApi.tasks,
    getProjectWorkTask: projectApi.taskDetail,
    patchScrumProjectNew: projectApi.updateProject,
  }))

mock.module('@/app/native-modules/local-bi-screen-loader', () => biScreenLoader)

const projectFixture = {
  id: 9,
  team_id: 1,
  name: 'Project Nine',
  status: 'active',
  version_task_count: 1,
}

const boardFixture = {
  project: projectFixture,
  version_tasks: [{
    id: 31,
    team_id: 1,
    project_new_id: 9,
    title: 'Plan Unit Thirty One',
    status: 'development',
    total_task_count: 2,
    completed_task_count: 1,
    progress: 50,
  }],
  requirements: [],
  tasks: [],
}

const requirementFixture = {
  id: 41,
  team_id: 1,
  project_new_id: 9,
  version_task_new_id: 31,
  title: 'React Requirement Forty One',
  description: 'Native requirement content',
  product_owner_user_id: 11,
  developer_user_ids: [11],
  acceptance_criteria: 'Native behavior remains equivalent',
  priority: 'normal',
  status: 'backlog',
  task_count: 1,
  completed_task_count: 0,
}

const taskFixture = {
  id: 51,
  team_id: 1,
  project_set_id: 9,
  project_id: 31,
  requirement_id: 41,
  kind: 'task',
  task_key: 'PM-51',
  title: 'Native Task Fifty One',
  description: 'Native task content',
  status: 'draft',
  priority: 'normal',
  assignee_id: 11,
  starts_at: null,
  due_at: null,
}

function access(
  initial: RouteAccessSnapshot,
  hostSnapshot = initial,
  workspaceResolution?: WorkspaceRouteResolution,
): RouteAccess {
  let current = initial
  return {
    snapshot: vi.fn(() => current),
    waitForHostSession: vi.fn(async () => {
      current = hostSnapshot
      return current
    }),
    ensureIdentity: vi.fn(async () => current),
    resolveWorkspace: vi.fn(async (): Promise<WorkspaceRouteResolution> =>
      workspaceResolution ?? (
        current.workspaceId > 0
          ? { status: 'ok', workspaceId: current.workspaceId, reason: 'current' }
          : { status: 'pick' }
      ),
    ),
  }
}

async function renderRoute(
  href: string,
  routeAccess: RouteAccess,
  actionCodes: readonly PMActionCode[] = PM_ACTION_CODES,
) {
  const synchronizeRuntime = (snapshot: RouteAccessSnapshot) => {
    if (snapshot.embedded) {
      runtimeSessionStore.getState().applyEmbeddedSession({
        token: snapshot.token,
        workspaceId: snapshot.workspaceId,
      })
    } else {
      runtimeSessionStore.getState().setStandaloneToken(snapshot.token)
      runtimeWorkspaceStore.getState().selectWorkspace(snapshot.workspaceId)
      if (snapshot.token) {
        seedWorkspacesForCurrentSession([
          { id: snapshot.workspaceId || 7, name: 'Workspace' },
        ])
      }
    }
  }
  synchronizeRuntime(routeAccess.snapshot())
  const router = createAppRouter({
    history: createMemoryHistory({ initialEntries: [href] }),
    access: routeAccess,
  })
  await router.load()
  synchronizeRuntime(routeAccess.snapshot())
  seedActionPermissions(actionCodes)
  render(
    <AppProviders>
      <RouterProvider router={router} />
    </AppProviders>,
  )
  return router
}

describe('PM React route parsing', () => {
  it('normalizes IDs and established project search values', () => {
    expect(parsePositiveRouteId('12')).toBe(12)
    expect(parsePositiveRouteId('0')).toBeUndefined()
    expect(parsePositiveRouteId('invalid')).toBeUndefined()
    expect(parseProjectSearch({ tab: 'members', from_item: '31' })).toEqual({
      tab: 'overview',
      from_item: 31,
    })
    expect(parseProjectSearch({ tab: 'items', from_item: '31' })).toEqual({
      tab: 'items',
      from_item: 31,
    })
    expect(parseProjectSearch({ tab: 'plan_units', from_item: '31' })).toEqual({
      tab: 'items',
      from_item: 31,
    })
    expect(hasLegacyProjectItemsSearch('?tab=plan_units&from_item=31')).toBe(true)
    expect(hasLegacyProjectItemsSearch('?tab=items&from_item=31')).toBe(false)
    expect(parseProjectSearch({ tab: 'unknown', from_item: '-1' })).toEqual({
      tab: 'overview',
      from_item: undefined,
    })
    expect(parsePlanUnitSearch({})).toEqual({
      tab: 'requirements',
      from_item: undefined,
    })
    expect(parsePlanUnitSearch({ tab: 'delivery' })).toEqual({
      tab: 'delivery',
      from_item: undefined,
    })
    expect(parseProjectSearch({ tab: 'tasks', req_q: 'must-not-survive' })).toEqual({
      tab: 'overview',
      from_item: undefined,
    })
    expect(parsePlanUnitSearch({ tab: 'members', member_page: '3' })).toEqual({
      tab: 'requirements',
      from_item: undefined,
    })
  })

  it('whitelists P7 content search and collapses defaults or invalid values', () => {
    expect(parsePlanUnitSearch({
      tab: 'tasks',
      from_item: '31',
      req_q: '  native React  ',
      req_status: 'development',
      req_priority: 'urgent',
      req_page: '2',
      req_page_size: '100',
      task_requirement_id: '41',
      task_id: '51',
      task_page: '3',
      task_page_size: '20',
      workflow_node: '  design-review  ',
      token: 'must-not-survive',
      arbitrary: 'must-not-survive',
    })).toEqual({
      tab: 'tasks',
      from_item: undefined,
      workflow_node: 'design-review',
      req_q: 'native React',
      req_status: 'development',
      req_priority: 'urgent',
      req_page: 2,
      req_page_size: 100,
      task_requirement_id: 41,
      task_id: 51,
      task_page: 3,
      task_page_size: 20,
    })

    expect(parsePlanUnitSearch({
      tab: 'iframe',
      req_q: '   ',
      req_status: 'unknown',
      req_priority: 'critical',
      req_page: '1',
      req_page_size: '25',
      task_requirement_id: '-41',
      task_id: '51',
      task_page: '1',
      task_page_size: '25',
    })).toEqual({
      tab: 'requirements',
      from_item: undefined,
    })

    expect(parsePlanUnitSearch({
      tab: 'tasks',
      task_id: '51',
    })).toEqual({
      tab: 'tasks',
      from_item: undefined,
    })
    expect(parsePlanUnitSearch({
      tab: 'requirements',
      task_requirement_id: '41',
      task_id: '51',
    })).toEqual({
      tab: 'requirements',
      from_item: undefined,
      task_requirement_id: 41,
    })
  })

  it('retires legacy member searches and drops their pagination', () => {
    expect(parseProjectSearch({
      tab: 'members',
      member_page: '3',
      member_page_size: '100',
      page_size: '20',
      member_filter: 'must-not-survive',
    })).toEqual({
      tab: 'overview',
      from_item: undefined,
    })

    expect(parseProjectSearch({
      tab: 'members',
      member_page: '1',
      member_page_size: '25',
    })).toEqual({
      tab: 'overview',
      from_item: undefined,
    })
  })

  it('keeps only safe redirect and workspace status values', () => {
    expect(parseLoginSearch({ redirect: 'https://evil.example/token' })).toEqual({
      redirect: undefined,
      login_code: undefined,
      external_registration_code: undefined,
      login_provider: undefined,
    })
    expect(parseLoginSearch({ redirect: '/workspace/projects' })).toEqual({
      redirect: undefined,
      login_code: undefined,
      external_registration_code: undefined,
      login_provider: undefined,
    })
    expect(parseLoginSearch({ redirect: '%2Fworkspace%2Fprojects' })).toEqual({
      redirect: undefined,
      login_code: undefined,
      external_registration_code: undefined,
      login_provider: undefined,
    })
    expect(
      parseWorkspaceSearch({
        redirect: '/workspace/projects/9?tab=members',
        empty: '1',
        error: 'other',
      }),
    ).toEqual({
      redirect: '/workspace/projects/9?tab=members',
      empty: '1',
      error: undefined,
    })
    expect(
      parseLoginSearch({
        redirect: '/workspace/projects/9?tab=members&token=secret',
        login_code: 'a'.repeat(64),
        login_provider: 'google',
      }),
    ).toEqual({
      redirect: '/workspace/projects/9?tab=members',
      login_code: 'a'.repeat(64),
      external_registration_code: undefined,
      login_provider: 'google',
    })
    expect(parseLoginSearch({ login_provider: 'telegram' })).toEqual({
      redirect: undefined,
      login_code: undefined,
      external_registration_code: undefined,
      login_provider: 'telegram',
    })
    expect(
      parseLoginSearch({
        login_code: 'a'.repeat(257),
        login_provider: 'unknown',
      }),
    ).toEqual({
      redirect: undefined,
      login_code: undefined,
      external_registration_code: undefined,
      login_provider: undefined,
    })
    expect(parseLoginSearch({
      external_registration_code: 'registration-once',
      login_provider: 'google',
    })).toEqual({
      redirect: undefined,
      login_code: undefined,
      external_registration_code: 'registration-once',
      login_provider: 'google',
    })
    expect(parseProfileSearch({ identity_result: 'bound', login_provider: 'telegram' }))
      .toEqual({ identity_result: 'bound', login_provider: 'telegram' })
    expect(parseProfileSearch({ identity_result: 'failed', login_provider: 'feishu' }))
      .toEqual({ identity_result: undefined, login_provider: undefined })
  })
})

describe('PM React route guards', () => {
  beforeEach(() => {
    queryClient.clear()
    platformApi.defaults.adapter = async (config) => { throw new Error(`Unexpected platform API: ${config.method} ${config.url}`) }
    coreApi.defaults.adapter = async (config) => {
      if (config.method === 'get' && config.url === '/access/me/action-permissions') {
        return {
          data: { data: { action_codes: PM_ACTION_CODES } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      throw new Error(`Unexpected core API request in router test: ${config.method} ${config.url}`)
    }
    vi.clearAllMocks()
    projectApi.list.mockResolvedValue({ rows: [projectFixture], total: 1 })
    projectApi.board.mockResolvedValue(boardFixture)
    projectApi.audit.mockResolvedValue({ rows: [], total: 0 })
    projectApi.users.mockResolvedValue([
      { id: 11, username: 'teo', display_name: 'Teo' },
    ])
    projectApi.requirements.mockResolvedValue({
      rows: [requirementFixture],
      total: 1,
    })
    projectApi.tasks.mockResolvedValue({ rows: [taskFixture], total: 1 })
    projectApi.taskDetail.mockResolvedValue(taskFixture)
    projectApi.updateProject.mockResolvedValue({ ...projectFixture, name: 'Updated Project' })
    biScreenLoader.loadLocalBiScreen.mockResolvedValue(() => (
      <div data-testid="native-bi-route" />
    ))
  })

  const standaloneReady = access({
    embedded: false,
    token: 'standalone-token',
    workspaceId: 7,
  })

  it('routes a non-active PM identity to the isolated status guide', async () => {
    const router = await renderRoute(
      '/workspace/projects',
      access({
        embedded: false,
        token: 'pending-token',
        workspaceId: 7,
        accountStatus: 'pending_approval',
      }),
    )

    expect(router.state.location.pathname).toBe('/identity-status')
    expect(await screen.findByRole('heading', {
      name: copy.auth.identityStatus.pending_approval.title,
    })).toBeTruthy()
    expect(screen.queryByRole('navigation')).toBeNull()
  })

  it('routes an active forced-reset identity to password setup before workspace access', async () => {
    runtimeSessionStore.getState().setStandaloneToken('forced-reset-token')
    runtimeSessionStore.getState().setIdentity({
      id: 19,
      username: 'forced-reset-user',
      display_name: '强制改密用户',
      avatar_url: '/forced-reset-user.png',
      role: 'user',
      status: 'active',
      password_reset_required: true,
    })
    const router = await renderRoute(
      '/workspace/projects',
      access({
        embedded: false,
        token: 'forced-reset-token',
        workspaceId: 7,
        accountStatus: 'active',
        passwordResetRequired: true,
      }),
    )

    expect(router.state.location.pathname).toBe('/reset-password')
    expect(await screen.findByRole('heading', { name: copy.auth.passwordResetTitle })).toBeTruthy()
    expect(screen.getByText(copy.auth.passwordResetRequiredDescription)).toBeTruthy()
    expect(screen.getByText('强制改密用户')).toBeTruthy()
    expect(screen.getByText(copy.auth.passwordResetAccountUsername('forced-reset-user'))).toBeTruthy()
    expect(document.querySelector('[data-slot="password-reset-identity"] img')?.getAttribute('src'))
      .toBe('/forced-reset-user.png')
    expect(screen.queryByRole('navigation')).toBeNull()
  })

  it('keeps active identities out of the restricted status guide', async () => {
    const router = await renderRoute(
      '/identity-status',
      access({
        embedded: false,
        token: 'active-token',
        workspaceId: 7,
        accountStatus: 'active',
      }),
    )

    expect(router.state.location.pathname).toBe('/workspace/projects')
  })

  it('keeps the logged-out root public and earns the workspace action', async () => {
    const router = await renderRoute(
      '/',
      access({ embedded: false, token: '', workspaceId: 0 }),
    )

    expect(await screen.findByRole('heading', {
      name: /需求按节点走/,
    })).toBeTruthy()
    expect(screen.getAllByRole('link', { name: '进入工作台' }).length).toBeGreaterThan(0)
    expect(
      screen.getAllByRole('link', { name: '进入工作台' }).every(
        (link) => link.getAttribute('href') === '/login',
      ),
    ).toBe(true)
    expect(router.state.location.pathname).toBe('/')
  })

  it('matches all three stable PM route shapes', async () => {
    const listRouter = await renderRoute('/workspace/projects', standaloneReady)
    expect(await screen.findByTestId('route-projects')).toBeTruthy()
    expect(listRouter.state.location.pathname).toBe('/workspace/projects')

    const detailRouter = await renderRoute(
      '/workspace/projects/9?tab=members&from_item=31',
      standaloneReady,
    )
    expect(await screen.findByTestId('route-project-detail')).toBeTruthy()
    expect(detailRouter.state.location.search).toMatchObject({
      tab: 'overview',
      from_item: 31,
    })

    const itemRouter = await renderRoute(
      '/workspace/projects/9/items/31',
      standaloneReady,
    )
    expect(await screen.findByTestId('route-project-item-detail')).toBeTruthy()
    expect(itemRouter.state.location.pathname).toBe('/workspace/projects/9/items/31')
    expect((itemRouter.options.context as { queryClient: typeof queryClient }).queryClient).toBe(queryClient)
  }, 10_000)

  it('warms the requirement pool page queries from the route loader without double fetching', async () => {
    const scopeRequests: unknown[] = []
    const poolRequests: unknown[] = []
    const directoryRequests: unknown[] = []
    const previousPlatformAdapter = platformApi.defaults.adapter
    const previousCoreAdapter = coreApi.defaults.adapter
    platformApi.defaults.adapter = async (config) => {
      if (config.method === 'get' && config.url === '/project-requirement-scope-options') {
        scopeRequests.push(config.params)
        return {
          data: { data: { project_sets: [], projects: [], requirements: [] } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      if (config.method === 'get' && config.url === '/requirements') {
        poolRequests.push(config.params)
        return {
          data: { data: { rows: [], total: 0 } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      if (config.method === 'get' && config.url === '/pm/workflows/templates') {
        return {
          data: { data: { rows: [] } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      throw new Error(`Unexpected platform API request in router test: ${config.method} ${config.url}`)
    }
    coreApi.defaults.adapter = async (config) => {
      if (config.method === 'get' && config.url === '/access/me/action-permissions') {
        return {
          data: { data: { action_codes: PM_ACTION_CODES } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      if (config.method === 'get' && config.url === '/directory/users') {
        directoryRequests.push(config.params)
        return {
          data: { data: { rows: [] } },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      throw new Error(`Unexpected core API request in router test: ${config.method} ${config.url}`)
    }

    try {
      const router = await renderRoute('/workspace/projects', standaloneReady)
      expect(await screen.findByTestId('route-projects')).toBeTruthy()

      await router.navigate({ to: '/workspace/requirements' })
      const poolKey = requirementPoolQueryOptions(7, requirementPoolQueryFilters({
        filters: emptyRequirementPoolFilterValue,
        projects: [],
        view: 'list',
        page: 1,
        pageSize: 20,
      })).queryKey
      await waitFor(() => {
        const poolState = queryClient.getQueryState(poolKey)
        expect(poolState?.status).toBe('success')
        expect(poolState?.fetchStatus).toBe('idle')
      })
      expect(queryClient.getQueryState(requirementScopeQueryOptions(7).queryKey)?.status).toBe('success')
      // The loader prefetch and the mounted page must share one cache entry
      // per endpoint: the pool and scope entries are warmed by the loader, so
      // after the real page mounts (the pending skeleton is gone) neither
      // endpoint may have been requested again. The people search runs through
      // the workspace user directory cache and may be served without a
      // request when a previous page load warmed it.
      await waitFor(() => expect(screen.queryByTestId('route-pending')).toBeNull(), { timeout: 10_000 })
      expect(poolRequests).toHaveLength(1)
      expect(scopeRequests).toHaveLength(1)
    } finally {
      platformApi.defaults.adapter = previousPlatformAdapter
      coreApi.defaults.adapter = previousCoreAdapter
    }
  }, 15_000)

  it.each([
    { embedded: false, token: 'standalone-token', workspaceId: 7 },
    { embedded: true, token: 'host-token', workspaceId: 8 },
  ])('blocks direct mapped-page access without any action permission: $embedded', async (snapshot) => {
    coreApi.defaults.adapter = async (config) => ({
      data: { data: { action_codes: [] } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })

    await renderRoute('/workspace/projects', access(snapshot), [])

    expect(await screen.findByRole('heading', { name: copy.app.pageAccess.deniedTitle })).toBeTruthy()
    expect(screen.queryByTestId('route-projects')).toBeNull()
  })

  it('lets the protected route guard continue after workspace resolution', async () => {
    const routeAccess = access({
      embedded: false,
      token: 'standalone-token',
      workspaceId: 7,
    })

    await renderRoute('/workspace/projects', routeAccess)

    expect(routeAccess.resolveWorkspace).toHaveBeenCalledTimes(1)
  })

  it('sends unauthenticated workspace home traffic to /login without a default redirect', async () => {
    const router = await renderRoute(
      '/workspace/projects',
      access({ embedded: false, token: '', workspaceId: 0 }),
    )

    expect(
      await screen.findByRole('heading', {
        name: copy.auth.formTitle,
      }),
    ).toBeTruthy()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toEqual({})
    expect(router.state.location.href).toBe('/login')
  })

  it('redirects an unauthenticated standalone deep link to login safely', async () => {
    const router = await renderRoute(
      '/workspace/projects/9?tab=members',
      access({ embedded: false, token: '', workspaceId: 0 }),
    )

    expect(
      await screen.findByRole('heading', {
        name: copy.auth.formTitle,
      }),
    ).toBeTruthy()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toEqual({
      redirect: '/workspace/projects/9?tab=members',
    })
    expect(router.state.location.href).not.toContain('token=')

    const externalLogins = screen.getByRole('group', {
      name: copy.auth.otherLoginMethods,
    })
    const feishuLogin = within(externalLogins).getByRole('button', {
      name: copy.auth.feishuLogin,
    }) as HTMLButtonElement
    expect(feishuLogin.disabled).toBe(false)
    expect(feishuLogin.getAttribute('title')).toBe(copy.auth.feishuLogin)
    const googleLogin = within(externalLogins).getByRole('button', {
      name: copy.auth.googleLogin,
    }) as HTMLButtonElement
    expect(googleLogin.disabled).toBe(false)
    expect(within(externalLogins).queryByRole('button', {
      name: copy.auth.telegramLogin,
    })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: copy.auth.submit }))
    const loginError = await screen.findByRole('alert')
    expect(loginError.textContent).toBe(copy.auth.credentialsRequired)
    expect(screen.getByRole('textbox', { name: copy.auth.loginIdentifier }).getAttribute(
      'aria-describedby',
    )).toBe(loginError.id)
  })

  it('preserves a bounded device authorization link through login', async () => {
    const router = await renderRoute(
      '/workspace/mcp-cli/device?user_code=ABCD-EFGH',
      access({ embedded: false, token: '', workspaceId: 0 }),
    )

    expect(await screen.findByRole('heading', { name: copy.auth.formTitle })).toBeTruthy()
    expect(router.state.location.pathname).toBe('/login')
    expect(router.state.location.search).toEqual({
      redirect: '/workspace/mcp-cli/device?user_code=ABCD-EFGH',
    })
  })

  it('redirects a standalone session without a workspace to selection', async () => {
    const router = await renderRoute(
      '/workspace/projects/9',
      access({ embedded: false, token: 'session', workspaceId: 0 }),
    )

    expect(
      await screen.findByRole('heading', {
        name: copy.workspace.selectorTitle,
      }),
    ).toBeTruthy()
    expect(router.state.location.pathname).toBe('/select-workspace')
    expect(router.state.location.search.redirect).toBe('/workspace/projects/9')

    const workspaceTrigger = screen.getByRole('combobox', {
      name: copy.workspace.selectorLabel,
    })
    const workspaceLabel = screen.getByText(copy.workspace.selectorLabel)
    expect((workspaceLabel as HTMLLabelElement).control).toBe(workspaceTrigger)
  })

  it('keeps personal account management available without a selected workspace', async () => {
    const routeAccess = access(
      { embedded: false, token: 'session', workspaceId: 0 },
      undefined,
      { status: 'pick' },
    )
    const router = await renderRoute('/workspace/profile', routeAccess, [])

    expect(router.state.location.pathname).toBe('/workspace/profile')
    expect(await screen.findByRole('heading', { name: copy.account.title })).toBeTruthy()
    expect(routeAccess.resolveWorkspace).not.toHaveBeenCalled()
  })

  it('keeps the standalone account surface out of embedded mode', async () => {
    const router = await renderRoute(
      '/workspace/profile',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )

    expect(router.state.location.pathname).toBe('/workspace/projects')
    expect(screen.queryByRole('heading', { name: copy.account.title })).toBeNull()
  })

  it.each([
    [{ status: 'pick' } as const, {}],
    [{ status: 'empty' } as const, { empty: 1 }],
    [{ status: 'error' } as const, { error: 'load' }],
  ])('revalidates a persisted workspace and exposes %s selection state', async (
    resolution,
    expectedSearch,
  ) => {
    const router = await renderRoute(
      '/workspace/projects/9',
      access(
        { embedded: false, token: 'session', workspaceId: 7 },
        undefined,
        resolution,
      ),
    )

    expect(router.state.location.pathname).toBe('/select-workspace')
    expect(router.state.location.search).toMatchObject({
      redirect: '/workspace/projects/9',
      ...expectedSearch,
    })
  })

  it('waits for an embedded host session and renders Inbox natively without duplicate shell', async () => {
    const routeAccess = access(
      { embedded: true, token: '', workspaceId: 0 },
      { embedded: true, token: 'host-token', workspaceId: 8 },
    )
    const router = await renderRoute('/workspace/inbox', routeAccess)

    expect(routeAccess.waitForHostSession).toHaveBeenCalledTimes(1)
    expect(await screen.findByRole('heading', { name: copy.app.navigation.inbox })).toBeTruthy()
    expect(router.state.location.pathname).toBe('/workspace/inbox')
    expect(document.querySelector('[data-slot="standalone-shell"]')).toBeNull()
    expect(screen.queryByRole('navigation', {
      name: copy.app.shell.primaryNavigation,
    })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.app.shell.logout })).toBeNull()
    expect(document.querySelector('[data-slot="host-content"]')).toBeNull()
  })

  it('shows an accessible fail-closed boundary when an embedded session stays unavailable', async () => {
    const unavailable = access(
      { embedded: true, token: '', workspaceId: 0 },
      { embedded: true, token: '', workspaceId: 0 },
    )

    await renderRoute('/workspace/projects', unavailable)

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain(copy.auth.sessionUnavailableTitle)
    expect(alert.textContent).toContain(copy.auth.sessionUnavailableDescription)
    expect(document.activeElement).toBe(alert)
    expect(document.querySelector('[data-slot="standalone-shell"]')).toBeNull()
  })

  it('blocks the standalone workspace selector in embedded mode', async () => {
    const router = await renderRoute(
      '/select-workspace',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )

    expect(router.state.location.pathname).toBe('/workspace/projects')
    expect(screen.queryByRole('heading', { name: copy.workspace.selectorTitle }))
      .toBeNull()
  })

  it('removes embedded business content immediately when the session expires', async () => {
    const router = await renderRoute(
      '/workspace/projects',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )
    const lifecycle = installSessionRouteLifecycle(router)

    expect(await screen.findByTestId('route-projects')).toBeTruthy()
    await act(async () => {
      runtimeSessionStore.getState().clearSession()
      await lifecycle.settled()
    })

    expect(screen.queryByTestId('route-projects')).toBeNull()
    expect(
      screen.getByRole('heading', {
        name: copy.auth.sessionUnavailableTitle,
      }),
    ).toBeTruthy()
    expect(
      screen.queryByRole('heading', {
        name: copy.auth.formTitle,
      }),
    ).toBeNull()
    lifecycle.dispose()
  })

  it.each([
    '/workspace/quality-management',
    '/workspace/quality-management/bugs/new?source_test_id=8',
    '/workspace/quality-management/bugs/9?from=inbox',
  ])('keeps the Quality route native-only for %s', async (href) => {
    const router = await renderRoute(href, standaloneReady)

    expect(router.state.location.pathname).toBe(new URL(href, window.location.origin).pathname)
    expect(document.querySelector('[data-slot="host-content"]')).toBeNull()
    expect(document.querySelector('iframe')).toBeNull()
  })

  it.each([
    '/workspace/work-items',
    '/workspace/work-items/31?from=inbox',
  ])('keeps the Work-items route native-only for %s', async (href) => {
    const router = await renderRoute(href, standaloneReady)

    expect(router.state.location.pathname).toBe(new URL(href, window.location.origin).pathname)
    expect(document.querySelector('[data-slot="host-content"]')).toBeNull()
    expect(document.querySelector('iframe')).toBeNull()
  })

  it('renders BI through the native-only boundary in standalone and embedded modes', async () => {
    window.__RUNTIME_CONFIG__ = { PM_NATIVE_BI_SCREEN_MODE: 'iframe' }
    await renderRoute('/workspace/bi-screen?period=month', standaloneReady)

    expect(await screen.findByTestId('native-bi-route')).toBeTruthy()
    expect(document.querySelector('[data-slot="host-content"]')).toBeNull()
    expect(document.querySelector('iframe')).toBeNull()

    cleanup()
    queryClient.clear()
    await renderRoute(
      '/workspace/bi-screen?period=month',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )

    expect(await screen.findByTestId('native-bi-route')).toBeTruthy()
    expect(document.querySelector('[data-slot="standalone-shell"]')).toBeNull()
    expect(document.querySelector('[data-slot="host-content"]')).toBeNull()
    expect(document.querySelector('iframe')).toBeNull()
  })

  it('parses native internal href search once and keeps canonical URL values', async () => {
    const router = await renderRoute('/workspace/inbox', standaloneReady)
    const navigate = createNativeInternalNavigator(router.navigate, {
      deniedMessage: 'denied',
    })

    await act(async () => {
      await navigate('/workspace/inbox?project_set_ids=5&project_ids=8&requirement_ids=13')
    })
    let url = new URL(router.state.location.href, window.location.origin)
    expect(url.searchParams.get('project_set_ids')).toBe('5')
    expect(url.searchParams.get('project_ids')).toBe('8')
    expect(url.searchParams.get('requirement_ids')).toBe('13')
    expect(router.state.location.href).not.toContain('%22')
    expect(router.state.location.search).toMatchObject({
      project_set_ids: 5,
      project_ids: 8,
      requirement_ids: 13,
    })
    expect(parseInboxSearch(router.state.location.search)).toMatchObject({
      project_set_ids: '5',
      project_ids: '8',
      requirement_ids: '13',
    })

    await act(async () => {
      await navigate('/workspace/work-items?statuses=draft&statuses=done&overdue=true&page=2')
    })
    url = new URL(router.state.location.href, window.location.origin)
    expect(router.state.location.search).toMatchObject({
      statuses: 'draft,completed',
      overdue: 1,
      page: 2,
    })
    expect(parseWorkItemsSearch(router.state.location.search)).toMatchObject({
      statuses: 'draft,completed',
      overdue: '1',
      page: 2,
      view: 'list',
    })
    expect(url.searchParams.getAll('statuses')).toEqual(['draft,completed'])
    expect(url.searchParams.get('overdue')).toBe('1')
    expect(url.searchParams.get('page')).toBe('2')
    expect(router.state.location.href).not.toContain('%22')

    await act(async () => {
      await navigate('/workspace/work-items/71?view=list&layout=gantt')
    })
    url = new URL(router.state.location.href, window.location.origin)
    expect(url.pathname).toBe('/workspace/work-items')
    expect(url.searchParams.get('task_id')).toBe('71')
    expect(url.searchParams.get('view')).toBe('list')
    expect(url.searchParams.get('layout')).toBe('gantt')
    expect(router.state.location.href.match(/\?/g)).toHaveLength(1)
  })

  it('keeps a single main landmark in the composed standalone project route', async () => {
    await renderRoute('/workspace/projects', standaloneReady)

    expect(screen.getAllByRole('main')).toHaveLength(1)
    expect(screen.getByRole('main').className).toContain('overflow-hidden')
    expect(screen.getByTestId('route-projects').tagName).toBe('DIV')
  })

  it('selects project sets in the master view without detail toggle entries', async () => {
    const secondProject = {
      ...projectFixture,
      id: 10,
      name: 'Project Ten',
    }
    const secondBoard = {
      ...boardFixture,
      project: secondProject,
      version_tasks: [{
        ...boardFixture.version_tasks[0],
        id: 32,
        project_new_id: 10,
        title: 'Plan Unit Thirty Two',
      }],
    }
    projectApi.list.mockResolvedValueOnce({
      rows: [projectFixture, secondProject],
      total: 2,
    })
    projectApi.board.mockImplementation(async (projectId: number) => (
      projectId === secondProject.id ? secondBoard : boardFixture
    ))
    const router = await renderRoute('/workspace/projects', standaloneReady)

    const projectCards = Array.from(
      document.querySelectorAll<HTMLButtonElement>('[data-test="project-set-item"]'),
    )
    expect(projectCards).toHaveLength(2)
    expect(projectCards[0].tagName).toBe('BUTTON')
    expect(document.querySelectorAll('[data-test="project-set-detail-toggle"]')).toHaveLength(0)
    expect(document.querySelectorAll('a[href*="/workspace/projects/10"]')).toHaveLength(0)
    await waitFor(() => expect(router.state.location.search.project_id).toBe(9))
    expect(router.state.location.pathname).toBe('/workspace/projects')
    await waitFor(() => expect(
      document.querySelectorAll<HTMLButtonElement>('[data-test="project-set-item"]')[0]
        ?.getAttribute('aria-pressed'),
    ).toBe('true'))
    await waitFor(() => expect(projectApi.board).toHaveBeenCalledWith(
      9,
      ['summary', 'version_tasks'],
      expect.anything(),
    ))

    fireEvent.click(document.querySelectorAll<HTMLButtonElement>('[data-test="project-set-item"]')[1]!)
    await waitFor(() => expect(router.state.location.search.project_id).toBe(10))
    expect(router.state.location.pathname).toBe('/workspace/projects')
    await waitFor(() => expect(projectApi.board).toHaveBeenCalledWith(
      10,
      ['summary', 'version_tasks'],
      expect.anything(),
    ))
    await waitFor(() => expect(
      document.querySelector<HTMLElement>('[data-test="plan-unit-title"]')?.textContent,
    ).toContain('Plan Unit Thirty Two'))
    expect(document.querySelector('[data-test="pm-project-sidebar"]')).toBeTruthy()
    expect(document.querySelector('[data-test="pm-split-layout"]')).toBeTruthy()
    expect(document.querySelector('[data-test="project-management-workbench"]')?.className)
      .toContain('overflow-hidden')
    expect(document.querySelector('[data-slot="operational-page-scroll"]')?.className)
      .toContain('overflow-hidden')
  })

  it('commits a project-set switch immediately and shows the board skeleton while data loads', async () => {
    const secondProject = {
      ...projectFixture,
      id: 10,
      name: 'Project Ten',
    }
    const secondBoard = {
      ...boardFixture,
      project: secondProject,
      version_tasks: [{
        ...boardFixture.version_tasks[0],
        id: 32,
        project_new_id: 10,
        title: 'Plan Unit Thirty Two',
      }],
    }
    let resolveSecondBoard!: (value: typeof secondBoard) => void
    const delayedSecondBoard = new Promise<typeof secondBoard>((resolve) => {
      resolveSecondBoard = resolve
    })
    projectApi.list.mockResolvedValueOnce({
      rows: [projectFixture, secondProject],
      total: 2,
    })
    projectApi.board.mockImplementation(async (projectId: number) => (
      projectId === secondProject.id ? delayedSecondBoard : boardFixture
    ))
    const router = await renderRoute('/workspace/projects?project_id=9', standaloneReady)

    await waitFor(() => expect(
      document.querySelector<HTMLElement>('[data-test="plan-unit-title"]')?.textContent,
    ).toContain(boardFixture.version_tasks[0].title))
    const projectCards = document.querySelectorAll<HTMLButtonElement>(
      '[data-test="project-set-item"]',
    )
    fireEvent.click(projectCards[1]!)

    await waitFor(() => expect(router.state.location.search.project_id).toBe(10))
    expect(document.querySelector('[data-slot="project-board-skeleton"]')).toBeTruthy()
    expect(screen.queryByText(boardFixture.version_tasks[0].title)).toBeNull()

    await act(async () => {
      resolveSecondBoard(secondBoard)
      await delayedSecondBoard
    })
    await waitFor(() => expect(
      document.querySelector<HTMLElement>('[data-test="plan-unit-title"]')?.textContent,
    ).toContain('Plan Unit Thirty Two'))
  })

  it.each(['title', 'row'] as const)(
    'commits a plan-unit detail route from the %s while business prefetch is still pending',
    async (entry) => {
      let resolveBoardRefresh!: (value: typeof boardFixture) => void
      const pendingBoardRefresh = new Promise<typeof boardFixture>((resolve) => {
        resolveBoardRefresh = resolve
      })
      const router = await renderRoute('/workspace/projects?project_id=9', standaloneReady)

      await waitFor(() => {
        const node = document.querySelector<HTMLElement>('[data-test="plan-unit-title"]')
        expect(node).toBeTruthy()
        return node!
      })
      await queryClient.invalidateQueries({
        queryKey: projectBoardQueryOptions(7, 9).queryKey,
        refetchType: 'none',
      })
      const title = await waitFor(() => {
        const node = document.querySelector<HTMLElement>('[data-test="plan-unit-title"]')
        expect(node?.isConnected).toBe(true)
        return node!
      })
      projectApi.board.mockReturnValueOnce(pendingBoardRefresh)

      await act(async () => { fireEvent.click(entry === 'title' ? title : title.closest('tr')!) })

      await waitFor(() => {
        expect(router.state.location.pathname).toBe('/workspace/projects/9/items/31')
        expect(screen.getByTestId('route-project-item-detail')).toBeTruthy()
      })
      expect(screen.queryByTestId('route-projects')).toBeNull()

      await act(async () => {
        resolveBoardRefresh(boardFixture)
        await pendingBoardRefresh
      })
    },
  )

  it('keeps project and item scoped routes full width without the project-set rail', async () => {
    await renderRoute('/workspace/projects/9', standaloneReady)

    expect(await screen.findByTestId('route-project-detail')).toBeTruthy()
    expect(document.querySelector('[data-test="pm-project-sidebar"]')).toBeNull()
    expect(document.querySelector('[data-test="pm-detail-layout"]')).toBeTruthy()
    expect(within(screen.getByRole('navigation', {
      name: copy.projectManagement.title,
    })).getAllByRole('link').map((link) => link.textContent)).toEqual([
      copy.projectManagement.tabs.overview,
      copy.projectManagement.tabs.planUnits,
      copy.projectManagement.tabs.activity,
    ])

    cleanup()
    queryClient.clear()
    await renderRoute('/workspace/projects/9/items/31', standaloneReady)

    expect(await screen.findByTestId('route-project-item-detail')).toBeTruthy()
    expect(document.querySelector('[data-test="pm-project-sidebar"]')).toBeNull()
    expect(document.querySelector('[data-test="pm-detail-layout"]')).toBeTruthy()
  })

  it('keeps exactly one main landmark for an embedded project route', async () => {
    await renderRoute(
      '/workspace/projects/9',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )

    expect(await screen.findByTestId('route-project-detail')).toBeTruthy()
    expect(screen.getAllByRole('main')).toHaveLength(1)
    expect(document.querySelector('[data-slot="standalone-shell"]')).toBeNull()
  })

  it('renders embedded delivery natively without HostContent or iframe fallback', async () => {
    const embeddedReady = access({ embedded: true, token: 'host-token', workspaceId: 8 })
    await renderRoute('/workspace/projects/9/items/31?tab=delivery', embeddedReady)
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.members.deliveryHeading,
    })).toBeTruthy()
    expect(projectApi.board).toHaveBeenCalledTimes(1)
    expect(document.querySelector('[data-slot="host-content"]')).toBeNull()
    expect(document.querySelector('iframe')).toBeNull()
    expect(document.querySelector('[data-slot="standalone-shell"]')).toBeNull()
  })

  it('deduplicates loader prefetch with mounted queries and lazily reads activity', async () => {
    await renderRoute('/workspace/projects/9', standaloneReady)

    expect(await screen.findByRole('heading', { name: projectFixture.name })).toBeTruthy()
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.overview.heading,
    })).toBeTruthy()
    await waitFor(() => {
      expect(projectApi.list).toHaveBeenCalledTimes(1)
      expect(projectApi.board).toHaveBeenCalledTimes(1)
      expect(projectApi.requirements).toHaveBeenCalledTimes(1)
      expect(projectApi.tasks).toHaveBeenCalledTimes(1)
      expect(projectApi.audit).not.toHaveBeenCalled()
      // The full workspace directory is one bounded fetch; referenced owner
      // IDs need no extra hydration request.
      expect(projectApi.users).toHaveBeenCalledTimes(1)
    })

    cleanup()
    queryClient.clear()
    projectApi.list.mockClear()
    projectApi.board.mockClear()
    projectApi.audit.mockClear()
    projectApi.users.mockClear()
    await renderRoute('/workspace/projects/9?tab=activity', standaloneReady)

    expect(await screen.findByRole('heading', { name: projectFixture.name })).toBeTruthy()
    await waitFor(() => {
      expect(projectApi.list).toHaveBeenCalledTimes(1)
      expect(projectApi.board).toHaveBeenCalledTimes(1)
      expect(projectApi.audit).toHaveBeenCalledTimes(1)
      expect(projectApi.users).toHaveBeenCalledTimes(1)
    })
  })

  it('renders the current plan-unit delivery lane from the same board source', async () => {
    const router = await renderRoute('/workspace/projects/9/items/31?tab=delivery', standaloneReady)

    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.members.deliveryHeading,
    })).toBeTruthy()
    expect(router.state.location.search).toMatchObject({ tab: 'delivery' })
    expect(screen.getByRole('link', {
      name: copy.projectManagement.delivery.assignments.heading,
      current: 'page',
    })).toBeTruthy()
    expect(projectApi.board).toHaveBeenCalledTimes(1)

    expect(screen.queryByRole('button', { name: /成员|权限/ })).toBeNull()
  })

  it('renders plan-unit header facts and type actions without capability flags', async () => {
    const writableProject = {
      ...projectFixture,
      can_create_plan_unit: true,
    }
    const writableUnit = {
      ...boardFixture.version_tasks[0],
      type: 'version',
      goal: 'Ship the native project detail',
      owner_user_id: 11,
      starts_at: '2026-08-01',
      due_at: '2026-08-31',
      requirement_count: 0,
      can_update: true,
      can_delete: true,
    }
    projectApi.list.mockResolvedValueOnce({ rows: [writableProject], total: 1 })
    projectApi.board.mockResolvedValueOnce({
      ...boardFixture,
      project: writableProject,
      version_tasks: [writableUnit],
    })
    await renderRoute('/workspace/projects/9/items/31?tab=overview', standaloneReady)

    const meta = await waitFor(() => {
      const value = document.querySelector('[data-slot="page-header-meta"]')
      expect(value).toBeTruthy()
      return value!
    })
    expect(meta.textContent).toContain(`${copy.projectManagement.projects}：${projectFixture.name}`)
    expect(meta.textContent).toContain(`${copy.projectManagement.planUnitDetail.owner}：`)
    expect(await screen.findByText('Teo')).toBeTruthy()
    expect(meta.textContent).toContain(
      `${copy.projectManagement.planUnitDetail.status}：${copy.projectManagement.statusLabels.development}`,
    )
    expect(meta.textContent).toContain(copy.projectManagement.planUnitDetail.dateRange)
    expect(screen.getByRole('button', {
      name: copy.projectManagement.write.actions.createPlanUnit,
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.write.actions.editPlanUnitByType(
        copy.projectManagement.planUnitList.unitVersion,
      ),
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.write.actions.deletePlanUnitByType(
        copy.projectManagement.planUnitList.unitVersion,
      ),
    })).toBeTruthy()
    expect(screen.queryByRole('link', {
      name: copy.projectManagement.planUnitDetail.back,
    })).toBeNull()

    cleanup()
    queryClient.clear()
    await renderRoute('/workspace/projects/9/items/31?tab=overview', standaloneReady)
    expect(await screen.findByRole('heading', { name: boardFixture.version_tasks[0].title })).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.planAI.combinedButton })).toBeTruthy()
    expect(document.querySelector('[data-slot="page-header-actions"]')
      ?.querySelectorAll('button')).toHaveLength(3)
  })

  it('defaults standalone plan-unit detail to requirements and retains native tasks', async () => {
    const requirementsRouter = await renderRoute(
      '/workspace/projects/9/items/31',
      standaloneReady,
    )

    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.content.requirement.heading,
    })).toBeTruthy()
    expect(await screen.findByText(requirementFixture.title)).toBeTruthy()
    expect(screen.getByRole('table', {
      name: copy.projectManagement.content.requirement.tableLabel,
    })).toBeTruthy()
    expect(requirementsRouter.state.location.search).toMatchObject({
      tab: 'requirements',
    })
    const tabs = screen.getByRole('navigation', {
      name: copy.projectManagement.planUnitDetail.heading,
    })
    expect(within(tabs).getByRole('link', {
      name: copy.projectManagement.tabs.tasks,
    })).toBeTruthy()
    expect(within(tabs).getByRole('link', {
      name: copy.projectManagement.delivery.assignments.heading,
    })).toBeTruthy()
    await waitFor(() => {
      expect(projectApi.requirements).toHaveBeenCalledTimes(1)
      expect(projectApi.tasks).not.toHaveBeenCalled()
    })
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.requirement.create,
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.common.edit,
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.common.move,
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.common.delete,
    })).toBeTruthy()

    cleanup()
    queryClient.clear()
    projectApi.requirements.mockClear()
    projectApi.tasks.mockClear()
    await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41',
      standaloneReady,
    )

    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.content.task.heading,
    })).toBeTruthy()
    expect(await screen.findByText(taskFixture.title)).toBeTruthy()
    expect(screen.getByRole('table', {
      name: copy.projectManagement.content.task.tableLabel,
    })).toBeTruthy()
    await waitFor(() => {
      expect(projectApi.requirements).toHaveBeenCalledTimes(1)
      expect(projectApi.tasks).toHaveBeenCalledTimes(1)
    })
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.task.create,
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.task.editLabel(taskFixture.title),
    })).toBeTruthy()
    expect(screen.getByRole('button', {
      name: copy.projectManagement.content.task.deleteLabel(taskFixture.title),
    })).toBeTruthy()
  })

  it('opens an exact task deep link and restores it across browser back and forward', async () => {
    const editableTask = { ...taskFixture, can_update: true }
    projectApi.tasks.mockResolvedValue({ rows: [], total: 1 })
    projectApi.taskDetail.mockResolvedValue(editableTask)
    const router = await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41',
      standaloneReady,
    )

    await act(async () => {
      await router.navigate({
        to: '/workspace/projects/$projectId/items/$projectItemId',
        params: { projectId: '9', projectItemId: '31' },
        search: {
          tab: 'tasks',
          from_item: undefined,
          task_requirement_id: 41,
          task_id: 51,
        },
      })
    })

    const dialog = await screen.findByRole('dialog', {
      name: copy.projectManagement.content.task.editTitle,
    })
    expect(within(dialog).getByDisplayValue(editableTask.title)).toBeTruthy()
    expect(projectApi.taskDetail).toHaveBeenCalledWith(51, expect.any(AbortSignal))

    act(() => { router.history.back() })
    await waitFor(() => {
      expect(router.state.location.search).not.toHaveProperty('task_id')
      expect(screen.queryByRole('dialog', {
        name: copy.projectManagement.content.task.editTitle,
      }) === null).toBe(true)
    })

    act(() => { router.history.forward() })
    expect(await screen.findByRole('dialog', {
      name: copy.projectManagement.content.task.editTitle,
    })).toBeTruthy()
    expect(router.state.location.search).toMatchObject({
      tab: 'tasks',
      task_requirement_id: 41,
      task_id: 51,
    })
  })

  it('keeps an out-of-scope task deep link fail-closed without exposing the target', async () => {
    const mismatchedTask = {
      ...taskFixture,
      project_id: 32,
      can_update: true,
      title: 'Must remain hidden',
    }
    projectApi.tasks.mockResolvedValue({ rows: [], total: 1 })
    projectApi.taskDetail.mockResolvedValue(mismatchedTask)
    const router = await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41&task_id=51',
      standaloneReady,
    )

    await waitFor(() => {
      expect(router.state.location.search).toMatchObject({
        tab: 'tasks',
        task_requirement_id: 41,
        task_id: 51,
      })
    })
    expect(await screen.findByText(
      copy.projectManagement.content.task.deepLinkUnavailableTitle,
    )).toBeTruthy()
    expect(screen.queryByText(mismatchedTask.title)).toBeNull()
    expect(screen.queryByRole('dialog', {
      name: copy.projectManagement.content.task.editTitle,
    })).toBeNull()
  })

  it('opens a PM task deep link for editing when legacy update capability is absent', async () => {
    projectApi.tasks.mockResolvedValue({ rows: [], total: 1 })
    projectApi.taskDetail.mockResolvedValue({ ...taskFixture, can_update: false })
    await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41&task_id=51',
      standaloneReady,
    )

    const input = await screen.findByDisplayValue(taskFixture.title)
    const dialog = await screen.findByRole('dialog', {
      name: copy.projectManagement.content.task.editTitle,
    })
    expect(input).toHaveProperty('disabled', false)
    expect(within(dialog).getByRole('button', {
      name: copy.projectManagement.content.common.save,
    })).toBeTruthy()
  })

  it('opens an exact task deep link even when the surrounding task list fails', async () => {
    projectApi.tasks.mockRejectedValue(new Error('task list unavailable'))
    projectApi.taskDetail.mockResolvedValue({ ...taskFixture, can_update: false })
    await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41&task_id=51',
      standaloneReady,
    )

    const dialog = await screen.findByRole('dialog', {
      name: copy.projectManagement.content.task.editTitle,
    })
    expect(within(dialog).getByDisplayValue(taskFixture.title)).toHaveProperty('disabled', false)
    expect(projectApi.taskDetail).toHaveBeenCalledWith(51, expect.any(AbortSignal))
  })

  it('retains a failed task deep link and recovers through an explicit retry', async () => {
    projectApi.tasks.mockResolvedValue({ rows: [], total: 1 })
    projectApi.taskDetail.mockRejectedValueOnce(new Error('temporary task failure'))
    const router = await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41&task_id=51',
      standaloneReady,
    )

    expect(await screen.findByText(
      copy.projectManagement.content.task.deepLinkLoadFailedTitle,
    )).toBeTruthy()
    expect(router.state.location.search).toMatchObject({ task_id: 51 })

    projectApi.taskDetail.mockResolvedValue({ ...taskFixture, can_update: false })
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.content.common.retry,
    }))
    expect(await screen.findByDisplayValue(taskFixture.title)).toHaveProperty('disabled', false)
    expect(projectApi.taskDetail).toHaveBeenCalledTimes(2)
  })

  it('clears a failed task deep link when its status panel is dismissed', async () => {
    projectApi.tasks.mockResolvedValue({ rows: [], total: 1 })
    projectApi.taskDetail.mockRejectedValue(new Error('temporary task failure'))
    const router = await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41&task_id=51',
      standaloneReady,
    )

    expect(await screen.findByText(
      copy.projectManagement.content.task.deepLinkLoadFailedTitle,
    )).toBeTruthy()
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.content.common.cancel,
    }))

    await waitFor(() => {
      expect(router.state.location.search).not.toHaveProperty('task_id')
      expect(screen.queryByText(
        copy.projectManagement.content.task.deepLinkLoadFailedTitle,
      )).toBeNull()
    })
  })

  it('keeps embedded task deep links behind the host boundary without task reads', async () => {
    const router = await renderRoute(
      '/workspace/projects/9/items/31?tab=tasks&task_requirement_id=41&task_id=51',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )

    await waitFor(() => {
      expect(router.state.location.search).not.toHaveProperty('task_id')
    })
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.content.host.unavailableTitle,
    })).toBeTruthy()
    const tabs = screen.getByRole('navigation', {
      name: copy.projectManagement.planUnitDetail.heading,
    })
    expect(within(tabs).queryByRole('link', {
      name: copy.projectManagement.tabs.tasks,
    })).toBeNull()
    expect(within(tabs).getByRole('link', {
      name: copy.projectManagement.delivery.assignments.heading,
    })).toBeTruthy()
    expect(projectApi.tasks).not.toHaveBeenCalled()
    expect(projectApi.taskDetail).not.toHaveBeenCalled()
    expect(projectApi.requirements).not.toHaveBeenCalled()
  })

  it('routes an embedded requirement row to the exact host task scope', async () => {
    await renderRoute(
      '/workspace/projects/9/items/31?tab=requirements',
      access({ embedded: true, token: 'host-token', workspaceId: 8 }),
    )

    const label = copy.projectManagement.content.requirement.openTasks(requirementFixture.title)
    await screen.findByRole('button', { name: label })
    fireEvent.click(screen.getByRole('button', { name: label }))

    expect(hostNavigation.navigateToWorkItems).toHaveBeenCalledTimes(1)
    expect(hostNavigation.navigateToWorkItems).toHaveBeenCalledWith({
      view: 'list',
      from: 'project-management',
      project_set_ids: '9',
      project_ids: '31',
      requirement_ids: '41',
    })
    expect(projectApi.requirements).toHaveBeenCalledTimes(1)
    expect(projectApi.tasks).not.toHaveBeenCalled()
  })

  it('preserves from_item navigation and scopes item activity through related facts', async () => {
    const legacyRouter = await renderRoute('/workspace/projects/9?tab=members&from_item=31', standaloneReady)
    expect(await screen.findByRole('heading', { name: copy.projectManagement.overview.heading })).toBeTruthy()
    expect(legacyRouter.state.location.search).toMatchObject({ tab: 'overview', from_item: 31 })

    cleanup()
    queryClient.clear()
    projectApi.audit.mockResolvedValue({
      rows: [
        {
          id: 71,
          resource: 'version',
          resource_id: 31,
          action: 'update',
          target: 'Direct Unit Activity',
          username: 'teo',
        },
        {
          id: 72,
          resource: 'version',
          resource_id: 32,
          action: 'update',
          target: 'Other Unit Activity',
          username: 'teo',
        },
        {
          id: 73,
          resource: 'requirement',
          resource_id: 41,
          action: 'update',
          target: 'Scoped Requirement Activity',
          username: 'teo',
        },
        {
          id: 74,
          resource: 'requirement',
          resource_id: 42,
          action: 'update',
          target: 'Other Requirement Activity',
          username: 'teo',
        },
        {
          id: 75,
          resource: 'task',
          resource_id: 51,
          action: 'update',
          target: 'Direct Task Activity',
          username: 'teo',
        },
        {
          id: 76,
          resource: 'task',
          resource_id: 52,
          action: 'update',
          target: 'Requirement Owned Task Activity',
          username: 'teo',
        },
        {
          id: 77,
          resource: 'task',
          resource_id: 53,
          action: 'update',
          target: 'Other Task Activity',
          username: 'teo',
        },
      ],
      total: 7,
    })
    projectApi.requirements.mockResolvedValue({
      rows: [
        requirementFixture,
        {
          ...requirementFixture,
          id: 42,
          version_task_new_id: 32,
          title: 'Other requirement',
        },
      ],
      total: 2,
    })
    projectApi.tasks.mockResolvedValue({
      rows: [
        taskFixture,
        {
          ...taskFixture,
          id: 52,
          project_id: 32,
          requirement_id: 41,
          title: 'Task owned through requirement',
        },
        {
          ...taskFixture,
          id: 53,
          project_id: 32,
          requirement_id: 42,
          title: 'Other task',
        },
      ],
      total: 3,
    })
    await renderRoute('/workspace/projects/9/items/31?tab=activity', standaloneReady)

    expect(await screen.findByText(/Direct Unit Activity/)).toBeTruthy()
    expect(screen.getByText(/Scoped Requirement Activity/)).toBeTruthy()
    expect(screen.getByText(/Direct Task Activity/)).toBeTruthy()
    expect(screen.getByText(/Requirement Owned Task Activity/)).toBeTruthy()
    expect(screen.queryByText(/Other Unit Activity/)).toBeNull()
    expect(screen.queryByText(/Other Requirement Activity/)).toBeNull()
    expect(screen.queryByText(/Other Task Activity/)).toBeNull()
    expect(screen.getByRole('region', {
      name: copy.projectManagement.activity.heading,
    }).querySelector('[data-slot="pagination"]')).toBeNull()
  })

  it('keeps board and activity failures inside retryable business boundaries', async () => {
    projectApi.board.mockRejectedValue(new Error('board unavailable'))
    await renderRoute('/workspace/projects/9', standaloneReady)

    expect(await screen.findByTestId('route-project-detail')).toBeTruthy()
    expect(screen.queryByTestId('route-error')).toBeNull()
    const retryBoard = await screen.findByRole('button', {
      name: copy.projectManagement.common.retry,
    })
    expect(projectApi.board).toHaveBeenCalledTimes(2)
    projectApi.board.mockResolvedValue(boardFixture)
    fireEvent.click(retryBoard)
    expect(await screen.findByRole('heading', { name: projectFixture.name })).toBeTruthy()
    expect(projectApi.board).toHaveBeenCalledTimes(3)

    cleanup()
    queryClient.clear()
    projectApi.audit.mockClear()
    projectApi.audit.mockRejectedValue(new Error('activity unavailable'))
    await renderRoute('/workspace/projects/9?tab=activity', standaloneReady)

    expect(await screen.findByRole('heading', { name: projectFixture.name })).toBeTruthy()
    expect(screen.queryByTestId('route-error')).toBeNull()
    const retryActivity = await screen.findByRole('button', {
      name: copy.projectManagement.common.retry,
    })
    expect(projectApi.audit).toHaveBeenCalledTimes(1)
    projectApi.audit.mockResolvedValue({ rows: [], total: 0 })
    fireEvent.click(retryActivity)
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.activity.project.emptyTitle,
    })).toBeTruthy()
    expect(projectApi.audit).toHaveBeenCalledTimes(2)
  })

  it('drives project editing through the P6 state machine and one mutation request', async () => {
    const writableProject = {
      ...projectFixture,
      owner_user_id: 11,
      starts_at: '2026-08-01',
      due_at: '2026-08-31',
      can_update: true,
      can_delete: true,
      can_create_plan_unit: true,
    }
    projectApi.list.mockResolvedValueOnce({ rows: [writableProject], total: 1 })
    projectApi.board.mockResolvedValueOnce({
      ...boardFixture,
      project: writableProject,
    })
    await renderRoute('/workspace/projects/9', standaloneReady)

    const meta = await waitFor(() => {
      const value = document.querySelector('[data-slot="page-header-meta"]')
      expect(value).toBeTruthy()
      return value!
    })
    expect(meta.textContent).toContain(`${copy.projectManagement.overview.owner}：`)
    expect(await screen.findByText('Teo')).toBeTruthy()
    expect(meta.textContent).toContain(
      `${copy.projectManagement.planUnitDetail.status}：${copy.projectManagement.statusLabels.active}`,
    )
    expect(meta.textContent).toContain(copy.projectManagement.planUnitDetail.dateRange)
    expect(screen.queryByRole('button', {
      name: copy.projectManagement.write.actions.createPlanUnit,
    })).toBeNull()
    fireEvent.click(await screen.findByRole('button', {
      name: copy.projectManagement.write.actions.editProject,
    }))
    const name = await screen.findByLabelText(new RegExp(
      copy.projectManagement.write.projectForm.name,
    ))
    fireEvent.change(name, { target: { value: 'Updated Project' } })
    const save = screen.getByRole('button', {
      name: copy.projectManagement.write.actions.save,
    })
    fireEvent.click(save)
    fireEvent.click(save)

    await waitFor(() => expect(projectApi.updateProject).toHaveBeenCalledTimes(1))
    expect(projectApi.updateProject).toHaveBeenCalledWith(9, expect.objectContaining({
      name: 'Updated Project',
    }))
    expect(await screen.findByText(
      copy.projectManagement.write.feedback.projectUpdated,
    )).toBeTruthy()
    expect(screen.queryByRole('dialog', {
      name: copy.projectManagement.write.projectForm.editTitle,
    })).toBeNull()
  })

  it('drops an open P6 draft when the workspace scope changes', async () => {
    const writableProject = { ...projectFixture, can_update: true }
    projectApi.list.mockResolvedValueOnce({ rows: [writableProject], total: 1 })
    projectApi.board.mockResolvedValueOnce({
      ...boardFixture,
      project: writableProject,
    })
    await renderRoute('/workspace/projects/9', standaloneReady)
    fireEvent.click(await screen.findByRole('button', {
      name: copy.projectManagement.write.actions.editProject,
    }))
    expect(await screen.findByRole('dialog', {
      name: copy.projectManagement.write.projectForm.editTitle,
    })).toBeTruthy()

    act(() => runtimeWorkspaceStore.getState().selectWorkspace(8))
    await waitFor(() => expect(screen.queryByRole('dialog', {
      name: copy.projectManagement.write.projectForm.editTitle,
    })).toBeNull())
    expect(projectApi.updateProject).not.toHaveBeenCalled()
  })

  it('keeps project editing available when legacy capability flags change', async () => {
    const writableProject = { ...projectFixture, can_update: true }
    projectApi.list.mockResolvedValueOnce({ rows: [writableProject], total: 1 })
    projectApi.board.mockResolvedValueOnce({
      ...boardFixture,
      project: writableProject,
    })
    await renderRoute('/workspace/projects/9', standaloneReady)
    fireEvent.click(await screen.findByRole('button', {
      name: copy.projectManagement.write.actions.editProject,
    }))
    expect(await screen.findByRole('dialog', {
      name: copy.projectManagement.write.projectForm.editTitle,
    })).toBeTruthy()

    act(() => queryClient.setQueryData(
      pmKeys.resource(7, 'planUnits', 'board', 9, 'summary-version-tasks'),
      { ...boardFixture, project: { ...writableProject, can_update: false } },
    ))
    await waitFor(() => expect(screen.getByRole('dialog', {
      name: copy.projectManagement.write.projectForm.editTitle,
    })).toBeTruthy())
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.write.actions.save,
    }))

    await waitFor(() => expect(projectApi.updateProject).toHaveBeenCalledTimes(1))
  })

  it('renders list empty, item not-found, and plan-unit filter states with fail-closed row actions', async () => {
    projectApi.list.mockResolvedValueOnce({ rows: [], total: 0 })
    await renderRoute('/workspace/projects', standaloneReady)
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.projectList.emptyTitle,
    })).toBeTruthy()

    cleanup()
    queryClient.clear()
    projectApi.board.mockResolvedValueOnce({ ...boardFixture, version_tasks: [] })
    await renderRoute('/workspace/projects/9/items/31', standaloneReady)
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.planUnitDetail.notFoundTitle,
    })).toBeTruthy()

    cleanup()
    queryClient.clear()
    const legacyRouter = await renderRoute(
      '/workspace/projects/9?tab=plan_units&from_item=31',
      standaloneReady,
    )
    expect(legacyRouter.state.location.pathname).toBe('/workspace/projects/9')
    expect(legacyRouter.state.location.search).toEqual({
      tab: 'items',
      from_item: 31,
    })
    expect(legacyRouter.state.location.href).toContain('tab=items')
    expect(legacyRouter.state.location.href).not.toContain('plan_units')
    const search = await screen.findByRole('textbox', {
      name: copy.projectManagement.planUnitList.searchLabel,
    })
    fireEvent.change(search, { target: { value: 'no-match' } })
    expect(await screen.findByRole('heading', {
      name: copy.projectManagement.planUnitList.filteredEmptyTitle,
    })).toBeTruthy()
    expect(document.querySelector('[data-test="pm-project-sidebar"]')).toBeNull()
    expect(screen.queryByRole('button', {
      name: copy.projectManagement.write.actions.createProject,
    })).toBeNull()
    expect(screen.queryByRole('button', {
      name: copy.projectManagement.write.actions.editPlanUnit,
    })).toBeNull()
    expect(screen.queryByRole('button', {
      name: copy.projectManagement.write.actions.deletePlanUnit,
    })).toBeNull()
  })

  it('redirects invalid numeric route params to the project list', async () => {
    const router = await renderRoute('/workspace/projects/not-a-number', standaloneReady)

    expect(await screen.findByTestId('route-projects')).toBeTruthy()
    expect(router.state.location.pathname).toBe('/workspace/projects')

    cleanup()
    queryClient.clear()
    const itemRouter = await renderRoute(
      '/workspace/projects/9/items/not-a-number',
      standaloneReady,
    )
    expect(await screen.findByTestId('route-projects')).toBeTruthy()
    expect(itemRouter.state.location.pathname).toBe('/workspace/projects')
  })

  it('restores route and search state through back and forward history', async () => {
    const router = await renderRoute('/workspace/projects', standaloneReady)
    await router.navigate({
      to: '/workspace/projects/$projectId',
      params: { projectId: '9' },
      search: { tab: 'overview', from_item: 31 },
    })
    expect(router.state.location.pathname).toBe('/workspace/projects/9')
    expect(router.state.location.search).toMatchObject({
      tab: 'overview',
      from_item: 31,
    })

    router.history.back()
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/workspace/projects')
    })

    router.history.forward()
    await waitFor(() => {
      expect(router.state.location.search).toMatchObject({
        tab: 'overview',
        from_item: 31,
      })
    })
  })

  it('gates the page-fade view transition to path changes only', () => {
    const router = createAppRouter({
      history: createMemoryHistory({ initialEntries: ['/login'] }),
    })
    const resolveTypes = (
      router.options.defaultViewTransition as {
        types: (info: {
          pathChanged: boolean
          hrefChanged: boolean
          hashChanged: boolean
        }) => string[] | false
      }
    ).types
    expect(resolveTypes({ pathChanged: true, hrefChanged: true, hashChanged: false })).toEqual([
      'page-fade',
    ])
    expect(resolveTypes({ pathChanged: false, hrefChanged: true, hashChanged: false })).toBe(false)
    expect(resolveTypes({ pathChanged: false, hrefChanged: false, hashChanged: true })).toBe(false)
  })
})
