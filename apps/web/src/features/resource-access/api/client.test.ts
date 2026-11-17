import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'

import { resourceAccessClient } from './client'

const originalAdapter = platformApi.defaults.adapter

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
})

const policyDTO = {
  resource_id: 'resource-1',
  mode: 'restricted',
  workspace_level: null,
  rules: [],
  policy_revision: 7,
  can_manage_access: true,
  effective_level: 'editor',
  effective_sources: ['owner'],
  subject_capabilities: { user: { available: true } },
}

describe('resource access HTTP adapter', () => {
  it('keeps paths and snake-case policy replacement in one adapter', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      return { data: { data: policyDTO }, status: 200, statusText: 'OK', headers: {}, config }
    }

    await resourceAccessClient.policy('resource-1')
    await resourceAccessClient.replacePolicy('resource-1', {
      mode: 'restricted',
      rules: [{
        subjectKind: 'user',
        subjectId: '900719925474099312345',
        level: 'viewer',
        includeDescendants: false,
      }],
      expectedPolicyRevision: 7,
      idempotencyKey: 'operation-1',
    })

    expect(requests.map(({ method, url }) => `${method?.toUpperCase()} ${url}`)).toEqual([
      'GET /document-resources/resource-1/access-policy',
      'PUT /document-resources/resource-1/access-policy',
    ])
    expect(JSON.parse(String(requests[1]?.data))).toEqual({
      mode: 'restricted',
      rules: [{ subject_kind: 'user', subject_id: '900719925474099312345', level: 'viewer' }],
      expected_policy_revision: 7,
    })
    expect(requests[1]?.headers['Idempotency-Key']).toBe('operation-1')
  })

  it('uses opaque subject IDs without parsing and rejects mixed-kind pages', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        data: { data: {
          items: [{ kind: 'user', id: '900719925474099312345', display_name: 'User' }],
          total: 1,
          has_more: false,
          capabilities: { user: { available: true } },
        } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const page = await resourceAccessClient.subjects({
      kind: 'user', query: 'User', limit: 20, offset: 0,
    })
    expect(page.items[0]?.id).toBe('900719925474099312345')
    expect(requests[0]?.params).toEqual({ kind: 'user', q: 'User', limit: 20, offset: 0 })

    await expect(resourceAccessClient.subjects({
      kind: 'role', query: '', limit: 20, offset: 0,
    })).rejects.toMatchObject({ code: 'RESOURCE_ACCESS_REQUEST_FAILED' })
  })

  it('rejects a policy response for a different resource', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: { ...policyDTO, resource_id: 'resource-2' } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    await expect(resourceAccessClient.policy('resource-1'))
      .rejects.toMatchObject({ code: 'RESOURCE_ACCESS_REQUEST_FAILED' })
  })

  it('exchanges a link secret in a POST body without placing it in the URL', async () => {
    let request: InternalAxiosRequestConfig | undefined
    platformApi.defaults.adapter = async (config) => {
      request = config
      return { data: { data: {
        resource_id: 'resource-1',
        access_secret: 'access-secret',
        level: 'viewer',
        expires_at: '2026-09-09T00:00:00Z',
      } }, status: 200, statusText: 'OK', headers: {}, config }
    }
    await expect(resourceAccessClient.exchangeLink('fragment-secret')).resolves.toEqual({
      resourceId: 'resource-1',
      accessSecret: 'access-secret',
      level: 'viewer',
      expiresAt: '2026-09-09T00:00:00Z',
    })
    expect(request?.url).toBe('/document-share-links/exchange')
    expect(request?.url).not.toContain('fragment-secret')
    expect(JSON.parse(String(request?.data))).toEqual({ secret: 'fragment-secret' })
  })
})
