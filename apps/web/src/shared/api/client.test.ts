import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'
import {
  coreApi,
  domainApiBase,
  platformApi,
  sameOriginApi,
  standaloneLoginUrl,
  standaloneRedirectPath,
} from './client'
import { pmSessionRequestConfig } from './context'
import {
  applyEmbeddedSession,
  clearRuntimeSession,
  getRuntimeToken,
  setRuntimeWorkspace,
  setStandaloneToken,
} from '@/shared/runtime'

function captureRequest(client: typeof coreApi) {
  let captured: InternalAxiosRequestConfig | null = null
  client.defaults.adapter = async (config) => {
    captured = config
    return {
      data: { data: [] },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  return () => captured
}

function rejectRequest(client: typeof coreApi, error: Record<string, unknown>) {
  client.defaults.adapter = async (config) => {
    Object.assign(error, { config })
    return Promise.reject(error)
  }
}

describe('PM domain API client', () => {
  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('')
    clearRuntimeSession()
    window.__RUNTIME_CONFIG__ = {
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    delete window.__RUNTIME_CONFIG__
    delete window.__MICRO_APP_ENVIRONMENT__
    delete window.microApp
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('normalizes empty, root, version, and domain base URL variants', () => {
    expect(domainApiBase('core', '')).toBe('/api/v1/core')
    expect(domainApiBase('platform', 'https://pm.example.com/')).toBe(
      'https://pm.example.com/api/v1/platform',
    )
    expect(domainApiBase('core', 'https://pm.example.com/api/v1')).toBe(
      'https://pm.example.com/api/v1/core',
    )
    expect(domainApiBase('platform', 'https://pm.example.com/api/v1/core/')).toBe(
      'https://pm.example.com/api/v1/platform',
    )
    expect(domainApiBase('core', 'https://pm.example.com/api/v1/platform')).toBe(
      'https://pm.example.com/api/v1/core',
    )
  })

  it('attaches auth, workspace, and timezone headers to both domains', async () => {
    setStandaloneToken('jwt-for-api')
    setRuntimeWorkspace(12)
    const coreRequest = captureRequest(coreApi)
    const platformRequest = captureRequest(platformApi)

    await coreApi.get('/auth/me')
    await platformApi.get('/project-sets')

    expect(coreRequest()?.baseURL).toBe('/api/v1/core')
    expect(platformRequest()?.baseURL).toBe('/api/v1/platform')
    for (const config of [coreRequest(), platformRequest()]) {
      expect(config?.headers.Authorization).toBe('Bearer jwt-for-api')
      expect(config?.headers['X-Workspace-ID']).toBe('12')
      expect(config?.headers['X-Timezone']).toBe('Asia/Shanghai')
    }
  })

  it('omits only workspace context for explicitly session-scoped requests', async () => {
    setStandaloneToken('jwt-for-session-api')
    setRuntimeWorkspace(12)
    const capturedRequest = captureRequest(coreApi)

    await coreApi.get('/auth/me', pmSessionRequestConfig())

    expect(capturedRequest()?.headers.Authorization).toBe(
      'Bearer jwt-for-session-api',
    )
    expect(capturedRequest()?.headers['X-Workspace-ID']).toBeUndefined()
    expect(capturedRequest()?.headers['X-Timezone']).toBe('Asia/Shanghai')

    await coreApi.get('/workspaces', pmSessionRequestConfig())

    expect(capturedRequest()?.headers.Authorization).toBe(
      'Bearer jwt-for-session-api',
    )
    expect(capturedRequest()?.headers['X-Workspace-ID']).toBeUndefined()
    expect(capturedRequest()?.headers['X-Timezone']).toBe('Asia/Shanghai')
  })

  it('keeps a valid host session and the raw error for a 403', async () => {
    const dispatch = vi.fn()
    const rawError = {
      response: {
        status: 403,
        data: { code: 'IDENTITY_NOT_PROVISIONED' },
      },
    }
    window.__MICRO_APP_ENVIRONMENT__ = true
    window.microApp = { addDataListener: vi.fn(), dispatch }
    applyEmbeddedSession({ token: 'host-jwt', workspaceId: 12 })
    rejectRequest(coreApi, rawError)

    await expect(coreApi.get('/auth/me')).rejects.toBe(rawError)

    expect(getRuntimeToken()).toBe('host-jwt')
    expect(dispatch).not.toHaveBeenCalled()
  })

  it('clears and notifies only for an explicit expired-session response', async () => {
    const dispatch = vi.fn()
    const authExpired = vi.fn()
    const rawError = {
      response: {
        status: 401,
        data: { code: 'SESSION_EXPIRED' },
      },
    }
    window.__MICRO_APP_ENVIRONMENT__ = true
    window.microApp = { addDataListener: vi.fn(), dispatch }
    applyEmbeddedSession({ token: 'expired-host-jwt', workspaceId: 12 })
    window.addEventListener('pm:auth-expired', authExpired, { once: true })
    rejectRequest(coreApi, rawError)

    await expect(coreApi.get('/auth/me')).rejects.toBe(rawError)

    expect(getRuntimeToken()).toBe('')
    expect(authExpired).toHaveBeenCalledTimes(1)
    expect(dispatch).toHaveBeenCalledWith({ type: 'auth-expired' })
  })

  it('does not expire an existing session for the login request exception', async () => {
    const dispatch = vi.fn()
    const rawError = {
      response: {
        status: 401,
        data: { code: 'SESSION_EXPIRED' },
      },
    }
    window.__MICRO_APP_ENVIRONMENT__ = true
    window.microApp = { addDataListener: vi.fn(), dispatch }
    applyEmbeddedSession({ token: 'existing-host-jwt', workspaceId: 12 })
    rejectRequest(coreApi, rawError)

    await expect(coreApi.post('/auth/login?source=pm')).rejects.toBe(rawError)

    expect(getRuntimeToken()).toBe('existing-host-jwt')
    expect(dispatch).not.toHaveBeenCalled()
  })

  it('keeps Feishu session exchange on the PM Web origin without runtime auth context', async () => {
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: 'https://pm-api.example.com',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
    setStandaloneToken('existing-pm-session')
    setRuntimeWorkspace(12)
    const capturedRequest = captureRequest(sameOriginApi)

    await sameOriginApi.post('/api/v1/core/auth/session/exchange', {
      login_code: 'one-time-login-code',
    })

    expect(capturedRequest()?.baseURL).toBeUndefined()
    expect(capturedRequest()?.url).toBe('/api/v1/core/auth/session/exchange')
    expect(capturedRequest()?.headers.Authorization).toBeUndefined()
    expect(capturedRequest()?.headers['X-Workspace-ID']).toBeUndefined()
    expect(JSON.stringify(capturedRequest())).not.toContain('existing-pm-session')
  })

  it('does not clear a fresh session when an older token request expires late', async () => {
    const authExpired = vi.fn()
    const rawError = {
      response: {
        status: 401,
        data: { code: 'SESSION_EXPIRED' },
      },
    }
    setStandaloneToken('expired-previous-jwt')
    window.addEventListener('pm:auth-expired', authExpired, { once: true })
    coreApi.defaults.adapter = async (config) => {
      setStandaloneToken('fresh-login-jwt')
      Object.assign(rawError, { config })
      return Promise.reject(rawError)
    }

    await expect(coreApi.get('/auth/me')).rejects.toBe(rawError)

    expect(getRuntimeToken()).toBe('fresh-login-jwt')
    expect(authExpired).not.toHaveBeenCalled()
  })

  it('preserves a standalone deep link in the login redirect without credentials', () => {
    const redirect = standaloneRedirectPath(
      '/pm-app/workspace/projects/9/items/31',
      '?tab=delivery',
      '/pm-app/',
    )
    const login = new URL(
      standaloneLoginUrl(redirect, '/pm-app/', 'https://pm.example.com'),
    )

    expect(redirect).toBe('/workspace/projects/9/items/31?tab=delivery')
    expect(login.pathname).toBe('/pm-app/login')
    expect(login.searchParams.get('redirect')).toBe(redirect)
    expect(login.toString()).not.toContain('token')
  })
})
