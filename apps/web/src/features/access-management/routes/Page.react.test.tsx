import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'bun:test'
import type { InternalAxiosRequestConfig } from 'axios'

import { permissionsStore, seedActionPermissions } from '@/features/access-control'
import { PM_ACTION, PM_PAGE_BY_ROUTE, type PMActionCode } from '@/generated/access-catalog'
import { coreApi } from '@/shared/api'
import { AccessManagementPage } from './Page'
import { WorkspaceManagementPage } from './WorkspaceManagementPage'

const originalAdapter = coreApi.defaults.adapter

afterEach(() => {
  coreApi.defaults.adapter = originalAdapter
  permissionsStore.getState().reset()
})

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
}

function renderPage(actionCodes: readonly PMActionCode[]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
  })
  seedActionPermissions(actionCodes)
  return render(
    <QueryClientProvider client={client}>
      <AccessManagementPage />
    </QueryClientProvider>,
  )
}

function renderWorkspacePage(actionCodes: readonly PMActionCode[]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
  })
  seedActionPermissions(actionCodes)
  return render(
    <QueryClientProvider client={client}>
      <WorkspaceManagementPage />
    </QueryClientProvider>,
  )
}

describe('AccessManagementPage', () => {
  it('fails closed with the shared empty state when the action set denies access', async () => {
    renderPage([])
    expect(await screen.findByText('无权访问人员与权限')).toBeTruthy()
    expect(screen.queryByRole('button', { name: '审批通过' })).toBeNull()
    expect(screen.queryByRole('tab', { name: '工作区管理' })).toBeNull()
  })

  it('allows the default group to view personnel without role or group directory grants', async () => {
    const reads: string[] = []
    coreApi.defaults.adapter = async (config) => {
      reads.push(String(config.url))
      return response(config, {
        items: [{
          id: 8,
          display_name: '默认组用户',
          username: 'default-user',
          status: 'active',
          group_id: 1,
          role_ids: [],
        }],
        page: 1,
        page_size: 20,
        total: 1,
        has_more: false,
      })
    }

    renderPage([PM_ACTION.AccessManagementPersonnelView])

    expect(await screen.findByText('默认组用户')).toBeTruthy()
    expect(screen.queryByText('无权访问人员与权限')).toBeNull()
    expect(reads.some((url) => url.endsWith('/users'))).toBe(true)
    expect(reads.filter((url) => url.endsWith('/roles'))).toEqual([])
    expect(reads.filter((url) => url.endsWith('/groups'))).toEqual([])
    expect(screen.queryByRole('button', { name: '配置归属和角色' })).toBeNull()
  })

  it('generates a forced password reset link and optionally queues notification', async () => {
    const writes: { url?: string; data?: string }[] = []
    coreApi.defaults.adapter = async (config) => {
      if (config.method !== 'get') {
        writes.push({ url: config.url, data: String(config.data || '') })
        return response(config, {
          url: 'https://pm.test/reset-password#token=once',
          expires_at: '2026-09-09T05:00:00Z',
          notification_queued: true,
        })
      }
      return response(config, {
        items: [{ id: 8, display_name: '需要改密', username: 'reset-user', status: 'active', group_id: 1, role_ids: [] }],
        page: 1,
        page_size: 20,
        total: 1,
        has_more: false,
      })
    }

    renderPage([
      PM_ACTION.AccessManagementPersonnelView,
      PM_ACTION.AccessManagementPersonnelResetPassword,
    ])

    fireEvent.click(await screen.findByRole('button', { name: '重置密码' }))
    fireEvent.click(screen.getByText('同时发送 outbox 通知给该用户'))
    expect(screen.getByRole('checkbox', { name: '同时发送 outbox 通知给该用户' }).getAttribute('aria-checked')).toBe('true')
    fireEvent.click(screen.getByRole('button', { name: '生成改密链接' }))

    await waitFor(() => expect(writes).toEqual([{
      url: '/access/admin/users/8/password-reset-links',
      data: '{"notify":true}',
    }]))
    expect(await screen.findByDisplayValue('https://pm.test/reset-password#token=once')).toBeTruthy()
    expect(screen.getByText('通知已写入 outbox。')).toBeTruthy()
  })

  it('shows workspace administration and reads members in the selected workspace', async () => {
    const reads: string[] = []
    const memberReads: Array<Record<string, unknown>> = []
    coreApi.defaults.adapter = async (config) => {
      reads.push(String(config.url))
      if (config.url === '/workspaces') return response(config, {
        items: [{
          id: 3,
          name: '产品工作区',
          slug: 'product',
          description: '产品交付',
          enabled: true,
          is_default: false,
          member_count: 1,
        }],
      })
      if (config.url === '/workspaces/3/members') {
        memberReads.push(config.params as Record<string, unknown>)
        return response(config, {
          items: [{
            user_id: 8,
            username: 'member',
            display_name: '工作区成员',
            email: 'member@example.com',
            status: 'active',
            role: 'owner',
            enabled: true,
          }],
          page: Number(config.params?.page) || 1,
          page_size: Number(config.params?.page_size) || 20,
          total: 21,
          has_more: Number(config.params?.page) !== 2,
        })
      }
      if (config.url === '/access/admin/users') return response(config, {
        items: [
          {
            id: 8,
            display_name: '工作区成员',
            username: 'member',
            email: 'member@example.com',
            avatar_url: '/member.png',
            status: 'active',
            role_ids: [],
          },
          {
            id: 9,
            display_name: '可添加成员',
            username: 'available',
            email: 'available@example.com',
            avatar_url: '/available.png',
            status: 'active',
            role_ids: [],
          },
          {
            id: 10,
            display_name: '第二位成员',
            username: 'second',
            email: 'second@example.com',
            avatar_url: '/second.png',
            status: 'active',
            role_ids: [],
          },
        ],
        page: 1,
        page_size: 100,
        total: 3,
        has_more: false,
      })
      return response(config, { items: [] })
    }

    renderWorkspacePage([
      PM_ACTION.AccessManagementWorkspacesView,
      PM_ACTION.AccessManagementWorkspacesUpdate,
      PM_ACTION.AccessManagementWorkspacesDelete,
      PM_ACTION.AccessManagementWorkspacesMembersView,
      PM_ACTION.AccessManagementWorkspacesMembersCreate,
      PM_ACTION.AccessManagementWorkspacesMembersUpdate,
      PM_ACTION.AccessManagementWorkspacesMembersDelete,
      PM_ACTION.AccessManagementPersonnelView,
    ])

    expect(await screen.findByText('产品工作区')).toBeTruthy()
    expect(await screen.findByText('工作区成员')).toBeTruthy()
    expect(screen.getByText('负责人')).toBeTruthy()
    expect(reads).toContain('/workspaces')
    expect(reads).toContain('/workspaces/3/members')
    expect(memberReads[0]?.page_size).toBe(20)
    expect(screen.getByText('每页条数')).toBeTruthy()
    expect(screen.getByRole('table', { name: '工作区成员列表' }).closest('[data-slot="data-table"]')?.className).toContain('h-150')
    expect(screen.queryByText('成员必须来自现有人员目录；此处只管理人员与工作区的归属关系。')).toBeNull()

    const memberSearch = screen.getByRole('searchbox', { name: '搜索工作区成员' })
    fireEvent.change(memberSearch, { target: { value: 'member@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: '搜索' }))
    await waitFor(() => {
      expect(memberReads.some((params) => params.q === 'member@example.com' && params.page === 1)).toBe(true)
    })

    fireEvent.click(screen.getByRole('button', { name: '下一页' }))
    await waitFor(() => {
      expect(memberReads.some((params) => params.q === 'member@example.com' && params.page === 2)).toBe(true)
    })

    fireEvent.click(screen.getByRole('button', { name: '添加成员' }))
    const memberDialog = await screen.findByRole('dialog', { name: '添加成员到 产品工作区' })
    expect(memberDialog.className).toContain('max-w-4xl')
    const availableMember = await screen.findByRole('checkbox', { name: '选择 可添加成员' })
    expect(screen.queryByRole('checkbox', { name: '选择 工作区成员' })).toBeNull()
    expect(availableMember.querySelector('[data-slot="user-preview-avatar"]')).toBeTruthy()
    fireEvent.click(within(availableMember).getByText('可添加成员'))
    const secondMember = await screen.findByRole('checkbox', { name: '选择 第二位成员' })
    fireEvent.click(within(secondMember).getByText('第二位成员'))
    expect(within(memberDialog).getByText('已选择 2 人')).toBeTruthy()
    expect(screen.getByRole('checkbox', { name: '移除 可添加成员' })).toBeTruthy()
    expect(screen.getByRole('checkbox', { name: '移除 第二位成员' })).toBeTruthy()
    fireEvent.click(within(memberDialog).getByText('取消'))

    fireEvent.click(screen.getByRole('button', { name: '删除' }))
    expect(await screen.findByText('确认删除工作区：产品工作区？')).toBeTruthy()
  })

  it('shows centralized labels instead of raw account status filter values', async () => {
    coreApi.defaults.adapter = async (config) => response(config, {
      items: [],
      page: 1,
      page_size: 20,
      total: 0,
      has_more: false,
    })

    renderPage([
      PM_ACTION.AccessManagementPersonnelView,
      PM_ACTION.AccessManagementRolesView,
      PM_ACTION.AccessManagementGroupsView,
    ])

    const filter = await screen.findByRole('combobox', { name: '账号状态筛选' })
    expect(filter.textContent).toContain('全部状态')
    expect(filter.textContent).not.toContain('all')

    fireEvent.click(filter)
    expect(await screen.findByRole('option', { name: '全部状态' })).toBeTruthy()
    expect(screen.getByRole('option', { name: '待审批' })).toBeTruthy()
    expect(screen.getByRole('option', { name: '已启用' })).toBeTruthy()
    expect(screen.getByRole('option', { name: '已驳回' })).toBeTruthy()
    expect(screen.getByRole('option', { name: '已停用' })).toBeTruthy()

    const activeOption = screen.getByRole('option', { name: '已启用' })
    fireEvent.pointerDown(activeOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(activeOption)
    await waitFor(() => {
      expect(filter.textContent).toContain('已启用')
      expect(filter.textContent).not.toContain('active')
    })
  })

  it('approves a pending user and edits single-group/multiple-role assignments', async () => {
    const writes: { method?: string; url?: string; data?: string }[] = []
    coreApi.defaults.adapter = async (config) => {
      if (config.method !== 'get') {
        writes.push({ method: config.method, url: config.url, data: String(config.data || '') })
        return response(config, {})
      }
      if (config.url?.endsWith('/users')) return response(config, {
        items: [{
          id: 8,
          display_name: '李华',
          username: 'lihua',
          email: 'lihua@example.com',
          status: 'pending_approval',
          group_id: 1,
          role_ids: [],
        }],
      })
      if (config.url?.endsWith('/groups')) return response(config, {
        items: [{ id: 1, code: 'default-users', name: '默认用户组', enabled: true, system_managed: true, is_default: true }],
      })
      if (config.url?.endsWith('/roles')) return response(config, {
        items: [{ id: 2, code: 'project-manager', name: '项目经理', description: '负责项目交付', enabled: true }],
      })
      return response(config, { items: [] })
    }

    renderPage([
      PM_ACTION.AccessManagementPersonnelView,
      PM_ACTION.AccessManagementPersonnelApprove,
      PM_ACTION.AccessManagementPersonnelReject,
      PM_ACTION.AccessManagementPersonnelBan,
      PM_ACTION.AccessManagementPersonnelUnban,
      PM_ACTION.AccessManagementPersonnelUpdateAccess,
      PM_ACTION.AccessManagementRolesView,
      PM_ACTION.AccessManagementGroupsView,
    ])
    fireEvent.click(await screen.findByRole('button', { name: '审批通过' }))
    expect(screen.getByRole('dialog').className).toContain('max-w-lg')
    fireEvent.click(screen.getByRole('button', { name: '确认操作' }))
    await waitFor(() => expect(writes.some((call) => call.url?.endsWith('/users/8/approve'))).toBe(true))

    fireEvent.click(await screen.findByRole('button', { name: '配置归属和角色' }))
    expect(screen.getByRole('dialog').className).toContain('max-w-xl')
    const groupSelect = screen.getByRole('combobox', { name: '用户组' })
    expect(groupSelect.textContent).toContain('默认用户组')
    expect(groupSelect.textContent).not.toBe('1')
    fireEvent.click(screen.getByRole('checkbox', { name: /^项目经理/ }))
    fireEvent.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/users/8/access') && call.data?.includes('"role_ids":[2]')
    ))).toBe(true))
  })

  it('shows role/group tabs, protected default group and edits a three-level permission tree', async () => {
    const pageCode = PM_PAGE_BY_ROUTE['/workspace/pm/access-management']
    const personnelModuleCode = PM_ACTION.AccessManagementPersonnelView.replace(/\.[^.]+$/, '')
    const writes: { url?: string; data?: string }[] = []
    let effectivePermissionReads = 0
    const grantedActions = [
      PM_ACTION.AccessManagementRolesView,
      PM_ACTION.AccessManagementRolesCreate,
      PM_ACTION.AccessManagementRolesUpdate,
      PM_ACTION.AccessManagementRolesDelete,
      PM_ACTION.AccessManagementGroupsView,
      PM_ACTION.AccessManagementGroupsCreate,
      PM_ACTION.AccessManagementGroupsUpdate,
      PM_ACTION.AccessManagementGroupsDelete,
      PM_ACTION.AccessManagementPermissionsView,
      PM_ACTION.AccessManagementPermissionsViewRole,
      PM_ACTION.AccessManagementPermissionsViewGroup,
      PM_ACTION.AccessManagementPermissionsAssignRole,
      PM_ACTION.AccessManagementPermissionsAssignGroup,
      PM_ACTION.AccessManagementPersonnelView,
      PM_ACTION.AccessManagementRolesAssign,
      PM_ACTION.AccessManagementGroupsAssign,
    ]
    coreApi.defaults.adapter = async (config) => {
      if (config.method !== 'get') {
        writes.push({ url: config.url, data: String(config.data || '') })
        return response(config, {})
      }
      if (config.url?.endsWith('/users')) {
        const params = config.params as Record<string, unknown> | undefined
        if (params?.group_id || params?.role_id) return response(config, { items: [], page: 1, page_size: 20, total: 0, has_more: false })
        return response(config, {
          items: [
            {
              id: 18,
              display_name: '候选用户',
              username: 'candidate',
              email: 'candidate@example.com',
              status: 'active',
              group_id: 9,
              role_ids: [2],
            },
            {
              id: 19,
              display_name: '可添加用户',
              username: 'available',
              email: 'available@example.com',
              status: 'active',
              group_id: 1,
              role_ids: [],
            },
          ],
          page: 1,
          page_size: 20,
          total: 2,
          has_more: false,
        })
      }
      if (config.url?.endsWith('/roles')) return response(config, {
        items: [{ id: 2, code: 'project-manager', name: '项目经理', enabled: true }],
      })
      if (config.url?.endsWith('/groups')) return response(config, {
        items: [{ id: 1, code: 'default-users', name: '默认用户组', enabled: true, system_managed: true, is_default: true }],
      })
      if (config.url?.endsWith('/access/me/action-permissions')) {
        effectivePermissionReads += 1
        return response(config, { action_codes: grantedActions })
      }
      if (config.url?.endsWith('/action-permissions')) return response(config, { action_codes: [] })
      return response(config, { items: [{
        code: pageCode, name: '人员与权限', node_type: 'page', children: [{
          code: personnelModuleCode, name: '人员列表', node_type: 'module', children: [
            { code: PM_ACTION.AccessManagementPersonnelView, name: '查看人员', node_type: 'action', grantable: true },
            { code: PM_ACTION.RequirementsRoleOwnersAssign, name: '配置需求角色负责人', node_type: 'action', grantable: false },
          ],
        }],
      }] })
    }

    renderPage(grantedActions)
    fireEvent.click(screen.getByRole('tab', { name: '权限主体管理' }))
    expect(await screen.findByText('项目经理')).toBeTruthy()
    expect(screen.getByRole('button', { name: '编辑 项目经理' }).textContent).toBe('编辑')
    expect(screen.getByRole('button', { name: '删除 项目经理' }).textContent).toBe('删除')
    expect(await screen.findByText('人员列表')).toBeTruthy()
    const ungrantable = screen.getByRole('checkbox', { name: '配置需求角色负责人' })
    expect(
      ungrantable.matches(':disabled')
      || ungrantable.hasAttribute('data-disabled')
      || ungrantable.getAttribute('aria-disabled') === 'true',
    ).toBe(true)
    expect(screen.getByText('暂不可独立授权')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '收起 人员列表' }))
    expect(screen.queryByRole('checkbox', { name: /^查看人员/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '展开 人员列表' }))
    fireEvent.click(screen.getByRole('checkbox', { name: /^查看人员/ }))
    fireEvent.click(screen.getByRole('button', { name: '保存权限' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/roles/2/action-permissions') && call.data?.includes(PM_ACTION.AccessManagementPersonnelView)
    ))).toBe(true))
    await waitFor(() => expect(effectivePermissionReads).toBeGreaterThan(0))

    fireEvent.click(screen.getByRole('button', { name: '添加成员' }))
    expect(await screen.findByText('添加成员到 项目经理')).toBeTruthy()
    const availableRoleMember = await screen.findByRole('checkbox', { name: '选择 可添加用户' })
    expect(screen.queryByRole('checkbox', { name: '选择 候选用户' })).toBeNull()
    fireEvent.click(within(availableRoleMember).getByText('可添加用户'))
    fireEvent.click(screen.getByRole('button', { name: '添加 1 人' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/users/19/roles')
      && call.data?.includes('"role_ids":[2]')
    ))).toBe(true))

    fireEvent.click(screen.getByRole('button', { name: '新建角色' }))
    expect(screen.queryByLabelText('稳定编码')).toBeNull()
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '设计师' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/roles')
      && call.data?.includes('"name":"设计师"')
      && !call.data?.includes('"code"')
    ))).toBe(true))

    fireEvent.click(screen.getByRole('tab', { name: '用户组' }))
    expect(await screen.findByText('默认用户组')).toBeTruthy()
    expect(screen.getByText('系统保护')).toBeTruthy()
    expect(screen.queryByRole('button', { name: '删除 默认用户组' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '编辑 默认用户组' }))
    expect(await screen.findByText('默认用户组不能停用或删除。')).toBeTruthy()
    const enabledCheckbox = screen.getByRole('checkbox', { name: /启用该权限主体/ })
    expect(
      enabledCheckbox.matches(':disabled')
      || enabledCheckbox.hasAttribute('data-disabled')
      || enabledCheckbox.getAttribute('aria-disabled') === 'true',
    ).toBe(true)
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '默认成员组' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/groups/1')
      && call.data?.includes('"name":"默认成员组"')
      && call.data?.includes('"enabled":true')
    ))).toBe(true))
    fireEvent.click(screen.getByRole('checkbox', { name: /^查看人员/ }))
    fireEvent.click(screen.getByRole('button', { name: '保存权限' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/groups/1/action-permissions')
      && call.data?.includes(PM_ACTION.AccessManagementPersonnelView)
    ))).toBe(true))
    fireEvent.click(screen.getByRole('button', { name: '添加成员' }))
    expect(await screen.findByText('添加成员到 默认用户组')).toBeTruthy()
    const availableGroupMember = await screen.findByRole('checkbox', { name: '选择 候选用户' })
    expect(screen.queryByRole('checkbox', { name: '选择 可添加用户' })).toBeNull()
    fireEvent.click(within(availableGroupMember).getByText('候选用户'))
    fireEvent.click(screen.getByRole('button', { name: '添加 1 人' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/users/18/group')
      && call.data?.includes('"group_id":1')
    ))).toBe(true))
  })

  it('removes role and group members only after confirmation while keeping the default group assigned', async () => {
    const writes: { url?: string; data?: string }[] = []
    const grantedActions = [
      PM_ACTION.AccessManagementRolesView,
      PM_ACTION.AccessManagementGroupsView,
      PM_ACTION.AccessManagementPersonnelView,
      PM_ACTION.AccessManagementRolesAssign,
      PM_ACTION.AccessManagementGroupsAssign,
    ]
    const memberPage = (item: Record<string, unknown>) => ({
      items: [item],
      page: 1,
      page_size: 20,
      total: 1,
      has_more: false,
    })
    coreApi.defaults.adapter = async (config) => {
      if (config.method !== 'get') {
        writes.push({ url: config.url, data: String(config.data || '') })
        return response(config, {})
      }
      if (config.url?.endsWith('/roles')) return response(config, {
        items: [{ id: 2, code: 'project-manager', name: '项目经理', enabled: true }],
      })
      if (config.url?.endsWith('/groups')) return response(config, {
        items: [
          { id: 1, code: 'default-users', name: '默认用户组', enabled: true, system_managed: true, is_default: true },
          { id: 5, code: 'delivery', name: '交付组', enabled: true },
        ],
      })
      if (config.url?.endsWith('/access/me/action-permissions')) {
        return response(config, { action_codes: grantedActions })
      }
      if (config.url?.endsWith('/users')) {
        const params = config.params as Record<string, unknown> | undefined
        if (params?.role_id === 2) return response(config, memberPage({
          id: 21,
          display_name: '角色成员',
          username: 'role-member',
          status: 'active',
          group_id: 5,
          role_ids: [2, 3],
        }))
        if (params?.group_id === 1) return response(config, memberPage({
          id: 22,
          display_name: '默认成员',
          username: 'default-member',
          status: 'active',
          group_id: 1,
          role_ids: [],
        }))
        if (params?.group_id === 5) return response(config, memberPage({
          id: 23,
          display_name: '普通组成员',
          username: 'group-member',
          status: 'active',
          group_id: 5,
          role_ids: [3],
        }))
        return response(config, { items: [], page: 1, page_size: 20, total: 0, has_more: false })
      }
      return response(config, { items: [] })
    }

    renderPage(grantedActions)
    fireEvent.click(screen.getByRole('tab', { name: '权限主体管理' }))

    const removeRoleMember = await screen.findByRole('button', { name: '从 项目经理 移除 角色成员' })
    fireEvent.click(removeRoleMember)
    expect(await screen.findByText('确认从 项目经理 移除 角色成员？')).toBeTruthy()
    expect(screen.getByText('确认后，该用户将立即失去此角色带来的权限，其他角色和用户组保持不变。')).toBeTruthy()
    expect(writes.some((call) => call.url?.endsWith('/users/21/roles'))).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: '确认移除' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/users/21/roles') && call.data?.includes('"role_ids":[3]')
    ))).toBe(true))

    fireEvent.click(screen.getByRole('tab', { name: '用户组' }))
    expect(await screen.findByText('默认成员')).toBeTruthy()
    expect(screen.getByText('默认用户组成员需通过加入其他用户组来变更归属。')).toBeTruthy()
    expect(screen.queryByRole('button', { name: '从 默认用户组 移除 默认成员' })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: '交付组' }))
    const removeGroupMember = await screen.findByRole('button', { name: '从 交付组 移除 普通组成员' })
    fireEvent.click(removeGroupMember)
    expect(await screen.findByText('确认后，该用户将移入默认用户组，原用户组权限将立即失效，已有角色保持不变。')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '确认移除' }))
    await waitFor(() => expect(writes.some((call) => (
      call.url?.endsWith('/users/23/group') && call.data?.includes('"group_id":1')
    ))).toBe(true))
  })

  it('keeps system-managed roles read-only while preserving member assignment', async () => {
    const writes: string[] = []
    const grantedActions = [
      PM_ACTION.AccessManagementRolesView,
      PM_ACTION.AccessManagementRolesUpdate,
      PM_ACTION.AccessManagementRolesDelete,
      PM_ACTION.AccessManagementPermissionsView,
      PM_ACTION.AccessManagementPermissionsViewRole,
      PM_ACTION.AccessManagementPermissionsAssignRole,
      PM_ACTION.AccessManagementPersonnelView,
      PM_ACTION.AccessManagementRolesAssign,
    ]
    const accessActionSegments = PM_ACTION.AccessManagementPersonnelView.split('.')
    const accessPageCode = accessActionSegments.slice(0, 2).join('.')
    const personnelModuleCode = accessActionSegments.slice(0, 3).join('.')
    coreApi.defaults.adapter = async (config) => {
      if (config.method !== 'get') {
        writes.push(`${config.method} ${config.url}`)
        return response(config, {})
      }
      if (config.url?.endsWith('/roles')) return response(config, {
        items: [{
          id: 1,
          code: 'access-administrator',
          name: '系统管理员',
          description: '系统内置角色',
          enabled: true,
          system_managed: true,
        }],
      })
      if (config.url?.endsWith('/users')) return response(config, {
        items: [], page: 1, page_size: 20, total: 0, has_more: false,
      })
      if (config.url?.endsWith('/action-permissions')) return response(config, {
        action_codes: [PM_ACTION.AccessManagementPersonnelView],
      })
      return response(config, { items: [{
        code: accessPageCode, name: '人员与权限', node_type: 'page', children: [{
          code: personnelModuleCode, name: '人员列表', node_type: 'view', children: [{
            code: PM_ACTION.AccessManagementPersonnelView,
            name: '查看人员',
            node_type: 'action',
            grantable: true,
          }],
        }],
      }] })
    }

    renderPage(grantedActions)
    fireEvent.click(screen.getByRole('tab', { name: '权限主体管理' }))

    expect(await screen.findByText('系统管理员')).toBeTruthy()
    expect(screen.getByText('系统保护')).toBeTruthy()
    expect(screen.getByText('系统内置主体的名称、状态和操作权限不可修改。')).toBeTruthy()
    expect(screen.queryByRole('button', { name: '编辑 系统管理员' })).toBeNull()
    expect(screen.queryByRole('button', { name: '删除 系统管理员' })).toBeNull()
    expect(screen.getByRole('button', { name: '保存权限' }).matches(':disabled')).toBe(true)
    expect((await screen.findByRole('checkbox', { name: '查看人员' })).hasAttribute('data-disabled')).toBe(true)
    expect(screen.getByRole('button', { name: '添加成员' })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: '保存权限' }))
    expect(writes).toEqual([])
  })
})
