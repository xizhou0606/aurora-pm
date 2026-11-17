import { afterEach, beforeEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'
import { getHostNavigation, putHostNavigation } from './api'

const originalAdapter = platformApi.defaults.adapter

describe('host navigation api', () => {
  beforeEach(() => {
    platformApi.defaults.adapter = async (config) => ({
      data: {
        data: {
          version: 1,
          can_manage: true,
          items: [{
            key: 'projects',
            label: '项目管理',
            path: '/workspace/projects',
            icon: 'briefcase',
            visible: true,
            sort_order: 10,
          }],
        },
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
  })

  it('loads the workspace host menu contract', async () => {
    const result = await getHostNavigation()
    expect(result.items.map((item) => item.key)).toEqual([
      'inbox',
      'bi-screen',
      'reporting',
      'projects',
      'requirements',
      'documents',
      'work-items',
      'design-management',
      'quality-management',
      'testing',
      'workflows',
      'ai-runs',
    ])
    expect(result.items.find((item) => item.key === 'projects')).toMatchObject({
      visible: true,
      path: '/workspace/projects',
    })
    expect(result.items.find((item) => item.key === 'requirements')).toMatchObject({
      key: 'requirements',
      path: '/workspace/requirements',
      visible: true,
    })
    expect(result.items.find((item) => item.key === 'documents')).toMatchObject({
      key: 'documents',
      path: '/workspace/documents',
      visible: true,
    })
  })

  it('saves only key visibility and order', async () => {
    const result = await putHostNavigation({
      items: [{ key: 'projects', visible: false, sort_order: 20 }],
    })
    expect(result.version).toBe(1)
  })
})
