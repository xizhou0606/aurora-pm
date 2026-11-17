import { afterEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'

import {
  decideAuthorization,
  fetchEffectiveToolingAccess,
  fetchAuthorizationRequest,
  fetchExternalGrants,
  fetchExternalToolingConfiguration,
  fetchToolCatalog,
  startAuthorization,
  updateExternalToolingConfiguration,
} from './api'

const originalAdapter = platformApi.defaults.adapter

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
})

describe('MCP/CLI browser API adapter', () => {
  it('uses the platform Tool Runtime and Grant endpoints', async () => {
    const calls: Array<{ method?: string; url?: string; data?: unknown }> = []
    platformApi.defaults.adapter = async (config) => {
      calls.push({ method: config.method, url: config.url, data: config.data })
      const data = config.url === '/tool-runtime/catalog'
        ? { tools: [{ name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grant_scope: 'project:read' }] }
        : config.url === '/external-tooling/effective-access'
          ? {
              configured: true,
              mode: 'permanent',
              status: 'active',
              tools: [{ name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grant_scope: 'project:read' }],
              cli_grant_ttl_hours: 168,
              effective_from: null,
              expires_at: null,
            }
        : config.url === '/external-tooling/grants'
          ? { grants: [] }
          : config.url === '/external-tooling/configuration'
              ? {
                  workspace_id: 7,
                  version: 1,
                  mode: 'window',
                tool_scopes: ['project.read'],
                grant_scopes: ['project:read'],
                cli_grant_ttl_hours: 168,
                effective_from: '2026-08-27T01:00:00Z',
                expires_at: '2026-09-27T01:00:00Z',
              }
          : config.url === '/external-tooling/authorization-requests'
            ? { request_id: 'request_1' }
            : config.url?.endsWith('/decision')
              ? {}
              : {
                  id: 'request_1', client: { id: 'codex', name: 'Codex' }, channel: 'mcp',
                  workspace: { id: 7, name: 'JiGuang' },
                  mode: 'window',
                  tools: [{ name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grant_scope: 'project:read' }],
                  effective_from: '2026-08-27T01:00:00Z', expires_at: '2026-09-27T01:00:00Z',
                }
      return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
    }

    await fetchToolCatalog()
    await fetchEffectiveToolingAccess()
    await fetchExternalGrants()
    await fetchExternalToolingConfiguration()
    await updateExternalToolingConfiguration({
      mode: 'window',
      tool_scopes: ['project.read'],
      cli_grant_ttl_hours: 168,
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })
    await startAuthorization({ channel: 'mcp' })
    await fetchAuthorizationRequest('request_1')
    await decideAuthorization('request_1', 'authorize')

    expect(calls.map((call) => call.url)).toEqual([
      '/tool-runtime/catalog',
      '/external-tooling/effective-access',
      '/external-tooling/grants',
      '/external-tooling/configuration',
      '/external-tooling/configuration',
      '/external-tooling/authorization-requests',
      '/external-tooling/authorization-requests/request_1',
      '/external-tooling/authorization-requests/request_1/decision',
    ])
    const configurationCall = calls.find((call) => call.method === 'put')
    expect(JSON.parse(String(configurationCall?.data))).toEqual({
      mode: 'window',
      tool_scopes: ['project.read'],
      cli_grant_ttl_hours: 168,
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })
    const authorizationCall = calls.find((call) => call.url === '/external-tooling/authorization-requests')
    expect(JSON.parse(String(authorizationCall?.data))).toEqual({ channel: 'mcp' })
    const decisionCall = calls.find((call) => call.url?.endsWith('/decision'))
    expect(JSON.parse(String(decisionCall?.data))).toEqual({ decision: 'authorize', oauth: false })
  })

  it('starts OAuth with PKCE facts and returns only a validated loopback callback', async () => {
    const calls: Array<{ url?: string; data?: unknown }> = []
    platformApi.defaults.adapter = async (config) => {
      calls.push({ url: config.url, data: config.data })
      const data = config.url === '/external-tooling/authorization-requests'
        ? { request_id: 'oauth_request_1' }
        : { authorization_redirect: 'http://127.0.0.1:43123/callback?code=code-1&state=state-1&iss=http%3A%2F%2Flocalhost%3A8081' }
      return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
    }
    await startAuthorization({
      channel: 'cli',
      oauth: {
        response_type: 'code', client_id: 'pm-cli', redirect_uri: 'http://127.0.0.1:43123/callback',
        code_challenge: 'A'.repeat(43), code_challenge_method: 'S256', state: 'state-1',
        resource: 'http://localhost:8081/api/v1/tool-runtime', scope: 'pm:tools offline_access',
      },
    })
    await expect(decideAuthorization('oauth_request_1', 'authorize', true))
      .resolves.toContain('http://127.0.0.1:43123/callback')
    expect(JSON.parse(String(calls[0].data))).toMatchObject({
      channel: 'cli', oauth: { client_id: 'pm-cli', code_challenge_method: 'S256' },
    })
    expect(JSON.parse(String(calls[1].data))).toEqual({ decision: 'authorize', oauth: true })
  })

  it('normalizes a missing server configuration to a fail-closed empty form', async () => {
    platformApi.defaults.adapter = async () => {
      throw {
        response: {
          status: 404,
          data: { error: { code: 'EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND' } },
        },
      }
    }
    await expect(fetchExternalToolingConfiguration()).resolves.toEqual({
      configured: false,
      version: 0,
      mode: 'disabled',
      toolScopes: [],
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours: 168,
    })
  })

  it('fails visibly when the catalog contract is not installed', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: { placeholder: true } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    await expect(fetchToolCatalog()).rejects.toMatchObject({
      code: 'TOOL_CATALOG_CONTRACT_INVALID',
    })
  })
})
