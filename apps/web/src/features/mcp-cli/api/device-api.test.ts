import { afterEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'

import {
  decideDeviceAuthorization,
  fetchDeviceAuthorization,
} from './device-api'

const originalAdapter = platformApi.defaults.adapter

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
})

const request = {
  user_code: 'ABCD-EFGH',
  status: 'pending',
  client: { id: 'pm-cli', name: 'PM CLI' },
  channel: 'cli',
  workspace: { id: 7, name: 'JiGuang' },
  mode: 'window',
  tools: [{
    name: 'project.read',
    version: '1.0.0',
    summary: 'Read project',
    lifecycle: 'active',
    risk: 'read',
    grant_scope: 'project:read',
  }],
  effective_from: '2026-08-27T01:00:00Z',
  expires_at: '2026-09-03T01:00:00Z',
  request_expires_at: '2026-08-27T01:10:00Z',
}

describe('device authorization API adapter', () => {
  it('reads by user code and submits an explicit decision without redirect facts', async () => {
    const calls: Array<{ method?: string; url?: string; params?: unknown; data?: unknown }> = []
    platformApi.defaults.adapter = async (config) => {
      calls.push({
        method: config.method,
        url: config.url,
        params: config.params,
        data: config.data,
      })
      const data = config.method === 'get' ? request : { user_code: 'ABCD-EFGH', status: 'approved' }
      return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
    }

    await expect(fetchDeviceAuthorization('ABCD-EFGH')).resolves.toMatchObject({
      userCode: 'ABCD-EFGH',
      status: 'pending',
    })
    await expect(decideDeviceAuthorization('ABCD-EFGH', 'authorize')).resolves.toBe('approved')

    expect(calls[0]).toMatchObject({
      method: 'get',
      url: '/external-tooling/device-authorizations',
      params: { user_code: 'ABCD-EFGH' },
    })
    expect(JSON.parse(String(calls[1]?.data))).toEqual({
      user_code: 'ABCD-EFGH',
      decision: 'authorize',
    })
  })

  it('fails closed when a decision response exposes an unknown status', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: { user_code: 'ABCD-EFGH', status: 'redirecting' } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    await expect(decideDeviceAuthorization('ABCD-EFGH', 'deny')).rejects.toMatchObject({
      code: 'DEVICE_AUTHORIZATION_DECISION_CONTRACT_INVALID',
    })
  })
})
