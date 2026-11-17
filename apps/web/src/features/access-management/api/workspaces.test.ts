import { afterEach, describe, expect, it } from 'bun:test'

import { coreApi } from '@/shared/api'
import {
  deleteManagedWorkspace,
  deleteManagedWorkspaceMember,
  listManagedWorkspaceMembers,
  listManagedWorkspaces,
  saveManagedWorkspace,
  saveManagedWorkspaceMember,
  saveManagedWorkspaceMembers,
  setManagedWorkspaceDefault,
} from './workspaces'

const originalAdapter = coreApi.defaults.adapter

afterEach(() => {
  coreApi.defaults.adapter = originalAdapter
})

describe('workspace administration API adapter', () => {
  it('normalizes workspace and member list contracts and ignores malformed rows', async () => {
    const memberParams: Record<string, unknown>[] = []
    coreApi.defaults.adapter = async (config) => {
      if (config.url?.endsWith('/members')) {
        memberParams.push(config.params as Record<string, unknown>)
      }
      return {
        data: {
          data: config.url?.endsWith('/members') ? {
            page: 2,
            page_size: 10,
            total: 11,
            has_more: false,
            items: [
              {
                user_id: 8,
                username: 'member',
                display_name: '工作区成员',
                email: 'member@example.com',
                status: 'active',
                role: 'owner',
                enabled: true,
              },
              { user_id: 0, status: 'unknown', role: 'admin' },
            ],
          } : {
            items: [
              {
                id: 3,
                name: '产品工作区',
                slug: 'product',
                description: '产品交付',
                enabled: true,
                is_default: false,
                member_count: 4,
              },
              { id: 0, name: '', slug: '' },
            ],
          },
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await expect(listManagedWorkspaces()).resolves.toMatchObject([
      { id: 3, name: '产品工作区', slug: 'product', memberCount: 4 },
    ])
    await expect(listManagedWorkspaceMembers(3, { query: '成员', page: 2, pageSize: 10 })).resolves.toMatchObject({
      items: [{ userId: 8, displayName: '工作区成员', role: 'owner', enabled: true }],
      page: 2,
      pageSize: 10,
      total: 11,
      hasMore: false,
    })
    expect(memberParams).toEqual([{
      include_disabled: true,
      q: '成员',
      page: 2,
      page_size: 10,
    }])
  })

  it('uses the expected workspace and membership mutation routes', async () => {
    const calls: Array<{ method?: string; url?: string; data?: string }> = []
    coreApi.defaults.adapter = async (config) => {
      calls.push({ method: config.method, url: config.url, data: String(config.data || '') })
      const isCreate = config.method === 'post' && config.url === '/workspaces'
      const isUpdate = config.method === 'patch' && config.url === '/workspaces/3'
      const isSetDefault = config.method === 'patch' && config.url === '/workspaces/4'
      return {
        data: {
          data: isCreate || isUpdate || isSetDefault ? {
            id: isSetDefault ? 4 : 3,
            name: '产品工作区',
            slug: 'product',
            description: '',
            enabled: true,
            is_default: isSetDefault,
            member_count: 0,
          } : {},
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await saveManagedWorkspace({ name: '产品工作区', slug: 'product', description: '', enabled: true })
    await saveManagedWorkspace({ id: 3, name: '产品工作区', slug: 'product', description: '', enabled: true })
    await deleteManagedWorkspace(3)
    await expect(setManagedWorkspaceDefault(4)).resolves.toMatchObject({ id: 4, isDefault: true })
    await saveManagedWorkspaceMember({ workspaceId: 3, userId: 8, role: 'member', enabled: true }, false)
    await saveManagedWorkspaceMembers([
      { workspaceId: 3, userId: 9, role: 'member', enabled: true },
      { workspaceId: 3, userId: 10, role: 'member', enabled: true },
    ])
    await saveManagedWorkspaceMember({ workspaceId: 3, userId: 8, role: 'owner', enabled: false }, true)
    await deleteManagedWorkspaceMember(3, 8)

    expect(calls.map(({ method, url }) => `${method} ${url}`)).toEqual([
      'post /workspaces',
      'patch /workspaces/3',
      'delete /workspaces/3',
      'patch /workspaces/4',
      'post /workspaces/3/members',
      'post /workspaces/3/members',
      'post /workspaces/3/members',
      'patch /workspaces/3/members/8',
      'delete /workspaces/3/members/8',
    ])
    expect(calls[3]?.data).toContain('"is_default":true')
    expect(calls[4]?.data).toContain('"user_id":8')
    expect(calls[5]?.data).toContain('"user_id":9')
    expect(calls[6]?.data).toContain('"user_id":10')
    expect(calls[7]?.data).not.toContain('user_id')
  })
})
