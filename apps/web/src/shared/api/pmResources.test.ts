import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'

import {
  clearRuntimeSession,
  setRuntimeWorkspace,
  setStandaloneToken,
} from '@/shared/runtime'
import { coreApi, platformApi } from './client'
import { requestPMScopeOptions, requestPMUsers, searchPlatformUsers } from './pmResources'

describe('PM-owned shared API resources', () => {
  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('native-token')
    setRuntimeWorkspace(23)
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: 'https://pm.example.com',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    delete window.__RUNTIME_CONFIG__
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('loads hierarchy options only from the canonical PM scope endpoint', async () => {
    let captured: InternalAxiosRequestConfig | null = null
    platformApi.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { data: { project_sets: [{ id: 1 }] } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const capturedRequest = () => captured as InternalAxiosRequestConfig | null

    await expect(requestPMScopeOptions()).resolves.toEqual({ project_sets: [{ id: 1 }] })
    expect(capturedRequest()?.method).toBe('get')
    expect(capturedRequest()?.url).toBe('/project-requirement-scope-options')
  })

  it('loads the PM-local workspace user directory', async () => {
    let captured: InternalAxiosRequestConfig | null = null
    coreApi.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { data: [{ id: 7, display_name: 'PM owner' }] },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const capturedRequest = () => captured as InternalAxiosRequestConfig | null

    await expect(requestPMUsers()).resolves.toEqual([{ id: 7, display_name: 'PM owner' }])
    expect(capturedRequest()?.baseURL).toBe('https://pm.example.com/api/v1/core')
    expect(capturedRequest()?.method).toBe('get')
    expect(capturedRequest()?.url).toBe('/directory/users')
    expect(capturedRequest()?.params).toEqual({ limit: 50, offset: 0 })
    expect(capturedRequest()?.headers.Authorization).toBe('Bearer native-token')
    expect(capturedRequest()?.headers['X-Workspace-ID']).toBe('23')
  })

  it('loads only one bounded directory page instead of downloading every user', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    coreApi.defaults.adapter = async (config) => {
      requests.push(config)
      const rows = Array.from({ length: 50 }, (_, index) => ({ id: index + 1 }))
      return {
        data: { data: rows },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await expect(requestPMUsers<unknown[]>()).resolves.toHaveLength(50)
    expect(requests.map((request) => request.params)).toEqual([
      { limit: 50, offset: 0 },
    ])
  })

  it('supports remote search and exact selected-value hydration without a full scan', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    coreApi.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        data: { data: { items: [{ id: 81, username: 'teo', display_name: 'Teo' }], total: 1 } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await expect(searchPlatformUsers({ q: 'teo', limit: 50 })).resolves.toMatchObject({ total: 1 })
    await expect(searchPlatformUsers({ ids: [81], limit: 100 })).resolves.toMatchObject({
      items: [{ id: 81, username: 'teo', display_name: 'Teo' }],
    })
    expect(requests.map((request) => request.params)).toEqual([
      { q: 'teo', limit: 50, offset: 0 },
      { ids: '81', limit: 100, offset: 0 },
    ])
  })
})
