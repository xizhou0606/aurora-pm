import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { coreApi, platformApi } from '@/shared/api'
import { clearRuntimeSession, setRuntimeWorkspace, setStandaloneToken } from '@/shared/runtime'
import { qualityClient } from './client'

describe('quality PM transport contract', () => {
  const originalPlatformAdapter = platformApi.defaults.adapter
  const originalCoreAdapter = coreApi.defaults.adapter

  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('quality-token')
    setRuntimeWorkspace(23)
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: 'https://pm.example.test',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalPlatformAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    delete window.__RUNTIME_CONFIG__
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('keeps quality and scope on PM platform while users use PM core', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      const data = config.url === '/quality/bugs'
        ? { data: { items: [], total: 0, summary: { total: 0 } } }
        : { data: { project_sets: [], projects: [], requirements: [] } }
      return { data, status: 200, statusText: 'OK', headers: {}, config }
    }
    coreApi.defaults.adapter = async (config) => {
      requests.push(config)
      const data = { data: { items: [{ id: 7, display_name: 'PM user' }], total: 1 } }
      return { data, status: 200, statusText: 'OK', headers: {}, config }
    }

    await expect(qualityClient.list({ limit: 50, offset: 0 })).resolves.toMatchObject({ total: 0 })
    await qualityClient.dashboard({ severity_key: 'p0' })
    await expect(qualityClient.scope()).resolves.toEqual({
      projectSets: [], projects: [], requirements: [],
    })
    await expect(qualityClient.users()).resolves.toMatchObject({ total: 1 })
    await qualityClient.sceneOptions('environment')

    expect(requests.map(({ baseURL, url }) => ({ baseURL, url }))).toEqual([
      { baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs' },
      { baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/dashboard' },
      { baseURL: 'https://pm.example.test/api/v1/platform', url: '/project-requirement-scope-options' },
      { baseURL: 'https://pm.example.test/api/v1/core', url: '/directory/users' },
      { baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/scene-options/environment' },
    ])
    expect(requests[1].params).toEqual({ severity_key: 'p0' })
    expect(requests.every((request) => request.headers['X-Workspace-ID'] === '23')).toBe(true)
  })

  it('uses only PM quality detail, activity, write, and upload routes', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      const payload = config.url?.endsWith('/activities')
        ? [{ id: 1, quality_bug_id: 31, user_name: 'PM user', action: 'created' }]
        : config.url?.endsWith('/comments') && config.method === 'get'
          ? [{ id: 2, quality_bug_id: 31, user_id: 7, body: 'looks good' }]
        : config.url?.endsWith('/materials/upload')
          ? { url: 'https://cdn.example.test/evidence.png' }
          : config.url?.startsWith('/quality/scene-options/')
            ? { id: 1, kind: 'browser', name: 'Safari', built_in: false, legacy: false }
          : config.method === 'delete'
            ? { deleted: true }
            : {
                id: 31,
                workspace_id: 23,
                title: 'PM quality issue',
                can_update: true,
                can_delete: false,
                allowed_actions: ['update', 'submit_review'],
              }
      return {
        data: { data: payload },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await qualityClient.detail(31)
    await qualityClient.activities(31, 0, 50)
    await qualityClient.comments(31)
    await qualityClient.createComment(31, 'looks good')
    await qualityClient.deleteComment(31, 2)
    await qualityClient.create({ requirement_id: 101, title: 'PM quality issue' })
    await qualityClient.patch(31, { status: 'review', resolution_reason: 'fixed' })
    await qualityClient.assign(31, { assignee_user_id: 7 })
    await qualityClient.batchAssign([31, 32], { assignee_user_id: 7 })
    await qualityClient.transition(31, { status: 'review', resolution_reason: 'fixed' })
    await qualityClient.remove(31)
    await qualityClient.batchRemove([31, 32])
    await qualityClient.upload(0, new File(['proof'], 'proof.png', { type: 'image/png' }))
    await qualityClient.createSceneOption('browser', 'Safari')
    await qualityClient.deleteSceneOption('browser', 1)

    expect(requests.map(({ method, baseURL, url }) => ({
      method: method?.toUpperCase(), baseURL, url,
    }))).toEqual([
      { method: 'GET', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31' },
      { method: 'GET', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31/activities' },
      { method: 'GET', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31/comments' },
      { method: 'POST', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31/comments' },
      { method: 'DELETE', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31/comments/2' },
      { method: 'POST', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs' },
      { method: 'PATCH', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31' },
      { method: 'PATCH', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31/assignment' },
      { method: 'POST', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/batch-assignment' },
      { method: 'PATCH', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31/status' },
      { method: 'DELETE', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/31' },
      { method: 'POST', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/batch-delete' },
      { method: 'POST', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/bugs/materials/upload' },
      { method: 'POST', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/scene-options/browser' },
      { method: 'DELETE', baseURL: 'https://pm.example.test/api/v1/platform', url: '/quality/scene-options/browser/1' },
    ])
    expect(requests[1].params).toEqual({ offset: 0, limit: 50 })
    expect(JSON.parse(String(
      requests.find((request) => request.url === '/quality/bugs/31/comments' && request.method === 'post')?.data,
    ))).toEqual({ body: 'looks good' })
    expect(JSON.parse(String(
      requests.find((request) => request.url === '/quality/bugs/batch-assignment')?.data,
    ))).toEqual({ ids: [31, 32], assignee_user_id: 7 })
    expect(JSON.parse(String(
      requests.find((request) => request.url === '/quality/bugs/batch-delete')?.data,
    ))).toEqual({ ids: [31, 32] })
    expect(requests.every((request) => request.headers['X-Workspace-ID'] === '23')).toBe(true)
    const uploadForm = requests.find((request) => (
      request.url === '/quality/bugs/materials/upload'
    ))?.data as FormData
    expect(uploadForm.get('requirement_id')).toBeNull()
    expect(uploadForm.get('file')).toBeInstanceOf(File)
  })
})
