import { afterEach, describe, expect, it } from 'bun:test'

import { coreApi } from '@/shared/api'
import {
  addUsersToSubject,
  changeUserStatus,
  createAccessSubject,
  createPasswordResetLink,
  deleteAccessSubject,
  listAccessGroups,
  listManagedUsers,
  listPermissionTree,
  replaceSubjectPermissions,
  replaceUserAccess,
  removeUserFromSubject,
  updateAccessSubject,
} from './client'

const originalAdapter = coreApi.defaults.adapter

afterEach(() => {
  coreApi.defaults.adapter = originalAdapter
})

describe('access management API adapter', () => {
  it('creates a password reset link with optional outbox notification', async () => {
    let requestBody = ''
    coreApi.defaults.adapter = async (config) => {
      requestBody = String(config.data || '')
      return {
        data: { data: { url: 'https://pm.test/reset-password#token=once', expires_at: '2026-09-09T05:00:00Z', notification_queued: true } },
        status: 201, statusText: 'Created', headers: {}, config,
      }
    }

    await expect(createPasswordResetLink({ userId: 7, notify: true })).resolves.toEqual({
      url: 'https://pm.test/reset-password#token=once',
      expiresAt: '2026-09-09T05:00:00Z',
      notificationQueued: true,
    })
    expect(requestBody).toContain('"notify":true')
  })

  it('normalizes user, default-group and permission-tree contracts fail closed', async () => {
    let usersParams: Record<string, unknown> | undefined
    coreApi.defaults.adapter = async (config) => ({
      data: {
        data: config.url?.endsWith('/users') ? {
          items: [{
            id: 7,
            display_name: '王小明',
            username: 'wang',
            email: 'wang@example.com',
            avatar_url: '/avatars/wang.png',
            status: 'pending_approval',
            group_id: 2,
            role_ids: [3],
          }, { id: 8, status: 'unknown' }],
          capabilities: {
            view: true,
            approve: true,
            manage_assignments: true,
          },
        } : config.url?.endsWith('/groups') ? {
          items: [{ id: 2, code: 'default-users', name: '默认用户组', enabled: true, system_managed: true }],
        } : config.url?.endsWith('/roles') ? {
          items: [{ id: 3, code: 'pm', name: 'PM', enabled: true }],
        } : {
          items: [{
            code: 'access.people',
            name: '人员管理',
            node_type: 'page',
            children: [{ code: 'access.people.list', name: '人员列表', node_type: 'view', children: [
              { code: 'access.people.detail', name: '人员详情', node_type: 'view', children: [
                {
                  code: 'access.people.list.view',
                  name: '查看',
                  node_type: 'action',
                  tools: ['user.search', 'user.search', '', 7],
                  object_policy: { code: 'task.creator', kind: 'creator_override' },
                },
              ] },
            ] }],
          }],
        },
      },
      status: (() => {
        if (config.url?.endsWith('/users')) usersParams = config.params as Record<string, unknown>
        return 200
      })(),
      statusText: 'OK',
      headers: {},
      config,
    })

    const users = await listManagedUsers({
      status: 'pending_approval',
      query: 'wang@example.com',
      page: 1,
      pageSize: 20,
      groupId: 2,
      roleId: 3,
    })
    expect(users.items).toHaveLength(1)
    expect(users.page).toBe(1)
    expect(users.total).toBe(1)
    expect(usersParams).toMatchObject({
      status: 'pending_approval',
      q: 'wang@example.com',
      group_id: 2,
      role_id: 3,
      page: 1,
      page_size: 20,
    })
    expect(users.items[0]?.group?.isDefault).toBe(true)
    expect(users.items[0]?.avatarUrl).toBe('/avatars/wang.png')
    expect(users.items[0]?.groupId).toBe(2)
    expect(users.items[0]?.roleIds).toEqual([3])
    expect(users.items[0]?.roles).toMatchObject([{ id: 3, code: 'pm' }])
    expect(await listAccessGroups()).toMatchObject([{ code: 'default-users', systemManaged: true }])
    const permissionTree = await listPermissionTree()
    expect(permissionTree[0]?.children[0]?.type).toBe('view')
    expect(permissionTree[0]?.children[0]?.children[0]?.type).toBe('view')
    expect(permissionTree[0]?.children[0]?.children[0]?.children[0]?.type).toBe('action')
    expect(permissionTree[0]?.children[0]?.children[0]?.children[0]?.tools).toEqual(['user.search'])
    expect(permissionTree[0]?.children[0]?.children[0]?.children[0]?.objectPolicy).toEqual({
      code: 'task.creator', kind: 'creator_override',
    })
  })

  it('adds members to roles and groups without replacing unrelated role assignments', async () => {
    const calls: { url?: string; data?: string }[] = []
    coreApi.defaults.adapter = async (config) => {
      calls.push({ url: config.url, data: String(config.data || '') })
      return { data: { data: {} }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const user = {
      id: 9,
      name: '成员',
      username: 'member',
      email: 'member@example.com',
      status: 'active' as const,
      groupId: 2,
      roleIds: [3],
      group: null,
      roles: [],
    }

    await addUsersToSubject('role', 4, [user])
    await addUsersToSubject('group', 5, [user])

    expect(calls).toEqual([
      { url: '/access/admin/users/9/roles', data: '{"role_ids":[3,4]}' },
      { url: '/access/admin/users/9/group', data: '{"group_id":5}' },
    ])
  })

  it('removes a role while preserving other roles and moves group members to the default group', async () => {
    const calls: { url?: string; data?: string }[] = []
    coreApi.defaults.adapter = async (config) => {
      calls.push({ url: config.url, data: String(config.data || '') })
      return { data: { data: {} }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const user = {
      id: 9,
      name: '成员',
      username: 'member',
      email: 'member@example.com',
      status: 'active' as const,
      groupId: 5,
      roleIds: [3, 4],
      group: null,
      roles: [],
    }

    await removeUserFromSubject({ kind: 'role', subjectId: 4, user })
    await removeUserFromSubject({ kind: 'group', subjectId: 5, user, fallbackGroupId: 2 })

    expect(calls).toEqual([
      { url: '/access/admin/users/9/roles', data: '{"role_ids":[3]}' },
      { url: '/access/admin/users/9/group', data: '{"group_id":2}' },
    ])
  })

  it('uses transactional replacement and status endpoints', async () => {
    const calls: { method?: string; url?: string; data?: string }[] = []
    coreApi.defaults.adapter = async (config) => {
      calls.push({ method: config.method, url: config.url, data: String(config.data || '') })
      return { data: { data: {} }, status: 200, statusText: 'OK', headers: {}, config }
    }

    await changeUserStatus({ userId: 9, intent: 'ban', reason: '风险账号' })
    await replaceUserAccess({ userId: 9, groupId: 2, roleIds: [3, 4] })
    await replaceSubjectPermissions({ kind: 'group', subjectId: 2, permissionCodes: ['a.b.c'] })
    await createAccessSubject({ kind: 'role', code: 'designer', name: '设计师', description: '设计', enabled: true })
    await updateAccessSubject({ kind: 'group', id: 2, code: 'default-users', name: '默认用户组', description: '默认', enabled: true })
    await deleteAccessSubject('role', 3)

    expect(calls.map(({ method, url }) => `${method} ${url}`)).toEqual([
      'post /access/admin/users/9/ban',
      'put /access/admin/users/9/access',
      'put /access/admin/groups/2/action-permissions',
      'post /access/admin/roles',
      'patch /access/admin/groups/2',
      'delete /access/admin/roles/3',
    ])
    expect(calls[1]?.data).toContain('"group_id":2')
    expect(calls[1]?.data).toContain('"role_ids":[3,4]')
    expect(calls[2]?.data).toContain('"action_codes":["a.b.c"]')
    expect(calls[3]?.data).toContain('"name":"设计师"')
    expect(calls[3]?.data).not.toContain('"code"')
    expect(calls[4]?.data).not.toContain('"code"')
  })
})
