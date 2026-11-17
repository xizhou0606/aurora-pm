import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'
import { hostAuthApi, hostCoreApiBase, requestHostCore } from './hostAuth'
import {
  clearRuntimeSession,
  setRuntimeWorkspace,
  setStandaloneToken,
} from '@/shared/runtime'

function captureRequest() {
  let captured: InternalAxiosRequestConfig | null = null
  hostAuthApi.defaults.adapter = async (config) => {
    captured = config
    return {
      data: { data: {} },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  return () => captured
}

describe('main-site auth API', () => {
  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('')
    clearRuntimeSession()
    window.__RUNTIME_CONFIG__ = {
      PM_HOST_API_BASE_URL: 'https://one2all.example.com/api/v1',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    delete window.__RUNTIME_CONFIG__
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('normalizes supported host roots to the core API', () => {
    expect(hostCoreApiBase('https://one2all.example.com/')).toBe(
      'https://one2all.example.com/api/v1/core',
    )
    expect(hostCoreApiBase('https://one2all.example.com/api/v1')).toBe(
      'https://one2all.example.com/api/v1/core',
    )
    expect(hostCoreApiBase('https://one2all.example.com/api/v1/platform/')).toBe(
      'https://one2all.example.com/api/v1/core',
    )
    expect(hostCoreApiBase('https://one2all.example.com/api/v1/work')).toBe(
      'https://one2all.example.com/api/v1/core',
    )
    expect(hostCoreApiBase('https://one2all.example.com/api/v1/core')).toBe(
      'https://one2all.example.com/api/v1/core',
    )
    expect(() => hostCoreApiBase('  ')).toThrow(
      '未配置主站认证地址 PM_HOST_API_BASE_URL',
    )
  })

  it('uses the host origin and all available runtime context headers', async () => {
    setStandaloneToken('main-site-token')
    setRuntimeWorkspace(17)
    const capturedRequest = captureRequest()

    await hostAuthApi.post('/auth/logout')

    expect(capturedRequest()?.baseURL).toBe(
      'https://one2all.example.com/api/v1/core',
    )
    expect(capturedRequest()?.headers.Authorization).toBe('Bearer main-site-token')
    expect(capturedRequest()?.headers['X-Workspace-ID']).toBe('17')
    expect(capturedRequest()?.headers['X-Timezone']).toBe('Asia/Shanghai')
  })

  it('omits auth and workspace headers before the initial login', async () => {
    const capturedRequest = captureRequest()

    await hostAuthApi.post('/auth/login', { username: 'teo', password: 'secret' })

    expect(capturedRequest()?.headers.Authorization).toBeUndefined()
    expect(capturedRequest()?.headers['X-Workspace-ID']).toBeUndefined()
    expect(capturedRequest()?.headers['X-Timezone']).toBe('Asia/Shanghai')
  })

  it('reads a safe relative core path and rejects alternate origins', async () => {
    const capturedRequest = captureRequest()

    await expect(requestHostCore('/public/modules')).resolves.toEqual({})
    expect(capturedRequest()?.url).toBe('/public/modules')
    await expect(requestHostCore('//evil.example/public/modules')).rejects.toMatchObject({
      code: 'HOST_CORE_PATH_INVALID',
    })
  })
})
