import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import {
  PMSafeApiError,
  platformApi,
} from '@/shared/api'
import {
  clearRuntimeSession,
  setRuntimeWorkspace,
  setStandaloneToken,
} from '@/shared/runtime'
import {
  inboxClient,
  withInboxRuntimeFailure,
  type InboxClient,
} from './index'

const originalAdapter = platformApi.defaults.adapter

describe('Inbox PM-local client contract', () => {
  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('inbox-token')
    setRuntimeWorkspace(23)
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: 'https://pm.example.test',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    delete window.__RUNTIME_CONFIG__
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('uses the canonical PM platform client with workspace and timezone headers', async () => {
    let captured: InternalAxiosRequestConfig | undefined
    platformApi.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { data: { summary: { total: 0 } } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }

    await inboxClient.request({ path: '/workbench/todos/summary' })

    expect(captured?.url).toBe('/workbench/todos/summary')
    expect(captured?.baseURL).toBe('https://pm.example.test/api/v1/platform')
    expect(captured?.headers.Authorization).toBe('Bearer inbox-token')
    expect(captured?.headers['X-Workspace-ID']).toBe('23')
    expect(captured?.headers['X-Timezone']).toBe('Asia/Shanghai')
  })

  it('unwraps PM summary and feed envelopes without using host transport', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: config.url?.endsWith('/feed')
        ? { data: [{ id: 31 }], total: 41 }
        : { data: { summary: { total: 1 } } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })

    await expect(inboxClient.request<{ summary: { total: number } }>({
      path: '/workbench/todos/summary',
    })).resolves.toEqual({ summary: { total: 1 } })
    await expect(inboxClient.requestPage<{ id: number }>({
      path: '/workbench/todos/feed',
    })).resolves.toEqual({ items: [{ id: 31 }], total: 41 })
  })

  it('reports only transport and 5xx errors to the Inbox-only fallback boundary', async () => {
    for (const error of [
      new Error('transport'),
      new PMSafeApiError('UPSTREAM_UNAVAILABLE', 503),
    ]) {
      const failure = vi.fn()
      const base: InboxClient = {
        request: vi.fn().mockRejectedValue(error),
        requestPage: vi.fn().mockRejectedValue(error),
      }
      const client = withInboxRuntimeFailure(base, failure)
      await expect(client.request({ path: '/workbench/todos/summary' })).rejects.toBe(error)
      expect(failure).toHaveBeenCalledWith(error)
    }
  })

  it('keeps controlled permission failures inside the native fail-closed state', async () => {
    const error = new PMSafeApiError('FORBIDDEN', 403)
    const failure = vi.fn()
    const base: InboxClient = {
      request: vi.fn().mockRejectedValue(error),
      requestPage: vi.fn().mockRejectedValue(error),
    }
    const client = withInboxRuntimeFailure(base, failure)
    await expect(client.requestPage({ path: '/workbench/todos/feed' })).rejects.toBe(error)
    expect(failure).not.toHaveBeenCalled()
  })
})
