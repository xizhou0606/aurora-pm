import { describe, expect, it } from 'bun:test'

import {
  normalizeAuthorizationRequest,
  normalizeExternalGrants,
  normalizeExternalToolingConfiguration,
  normalizeEffectiveToolingAccess,
  normalizeToolCatalog,
  parseAuthorizeSearch,
  oauthAuthorizeParameters,
  normalizeDeviceAuthorization,
  parseDeviceAuthorizeSearch,
} from './contract'
import {
  buildCliAuthConfiguration,
  buildCliConfiguration,
  buildMcpConfiguration,
} from './model'

const tool = {
  name: 'project.read',
  version: '1.0.0',
  summary: 'Read a PM project',
  lifecycle: 'active',
  risk: 'read',
  authorization: { grant_scope: 'project:read' },
}

describe('MCP/CLI browser contract', () => {
  it('normalizes canonical catalog and rejects partial tool records', () => {
    expect(normalizeToolCatalog({ tools: [tool] })).toEqual([{
      name: 'project.read',
      version: '1.0.0',
      summary: 'Read a PM project',
      lifecycle: 'active',
      risk: 'read',
      grantScope: 'project:read',
    }])
    expect(normalizeToolCatalog({ tools: [{ ...tool, risk: 'unknown' }] })).toBeUndefined()
    expect(normalizeToolCatalog({ tools: [{ ...tool, name: 'task.update', lifecycle: 'draft' }] })?.[0])
      .toMatchObject({ name: 'task.update', lifecycle: 'draft' })
  })

  it('normalizes current-user effective access without exposing draft tools', () => {
    expect(normalizeEffectiveToolingAccess({
      configured: true,
      mode: 'window',
      status: 'active',
      tools: [tool],
      cli_grant_ttl_hours: 30,
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })).toMatchObject({
      mode: 'window', status: 'active', tools: [{ name: 'project.read' }], cliGrantTtlHours: 30,
    })
    expect(normalizeEffectiveToolingAccess({
      configured: false,
      mode: 'disabled',
      status: 'unconfigured',
      tools: [],
      effective_from: null,
      expires_at: null,
    })).toEqual({
      configured: false,
      mode: 'disabled',
      status: 'unconfigured',
      tools: [],
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours: 168,
    })
    expect(normalizeEffectiveToolingAccess({
      configured: true,
      mode: 'permanent',
      status: 'active',
      tools: [{ ...tool, lifecycle: 'draft' }],
      cli_grant_ttl_hours: 168,
      effective_from: null,
      expires_at: null,
    })).toBeUndefined()
  })

  it('requires complete grant and authorization time windows', () => {
    expect(normalizeExternalGrants({ grants: [{
      id: 'grant_1',
      client_name: 'Codex',
      channel: 'mcp',
      status: 'scheduled',
      tool_scopes: ['project:read'],
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    }] })?.[0]?.status).toBe('scheduled')
    expect(normalizeExternalGrants({ grants: [{ id: 'grant_1' }] })).toBeUndefined()

    expect(normalizeAuthorizationRequest({
      id: 'request_1',
      client: { id: 'codex', name: 'Codex' },
      channel: 'cli',
      workspace: { id: 7, name: 'JiGuang' },
      mode: 'window',
      tools: [tool],
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })?.workspaceId).toBe(7)
    expect(normalizeAuthorizationRequest({
      id: 'request_1',
      client: { id: 'codex', name: 'Codex' },
      channel: 'cli',
      workspace: { id: 7, name: 'JiGuang' },
      mode: 'window',
      tools: [],
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })).toBeUndefined()

    expect(normalizeAuthorizationRequest({
      id: 'request_2',
      client: { id: 'pm-mcp', name: 'PM MCP' },
      channel: 'mcp',
      workspace: { id: 7, name: 'JiGuang' },
      mode: 'permanent',
      tools: [tool],
      effective_from: null,
      expires_at: null,
      oauth: true,
    })).toMatchObject({ mode: 'permanent', effectiveFrom: null, expiresAt: null, oauth: true })
  })

  it('normalizes only complete saved server configurations', () => {
    expect(normalizeExternalToolingConfiguration({
      workspace_id: 7,
      version: 3,
      mode: 'window',
      tool_scopes: ['project.read'],
      cli_grant_ttl_hours: 30,
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })).toEqual({
      configured: true,
      version: 3,
      mode: 'window',
      toolScopes: ['project.read'],
      effectiveFrom: '2026-08-27T01:00:00Z',
      expiresAt: '2026-09-27T01:00:00Z',
      cliGrantTtlHours: 30,
    })
    expect(normalizeExternalToolingConfiguration({
      version: 3,
      mode: 'window',
      tool_scopes: [],
      cli_grant_ttl_hours: 30,
      effective_from: '',
      expires_at: '',
    })).toBeUndefined()
    expect(normalizeExternalToolingConfiguration({
      version: 3,
      mode: 'window',
      tool_scopes: ['project.read', 'project.read'],
      cli_grant_ttl_hours: 30,
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-27T01:00:00Z',
    })).toBeUndefined()
    expect(normalizeExternalToolingConfiguration({
      version: 3,
      mode: 'permanent',
      tool_scopes: ['project.read'],
      cli_grant_ttl_hours: 30,
      effective_from: null,
      expires_at: null,
    })).toEqual({
      configured: true,
      version: 3,
      mode: 'permanent',
      toolScopes: ['project.read'],
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours: 30,
    })
    expect(normalizeExternalToolingConfiguration({
      version: 3,
      mode: 'disabled',
      tool_scopes: [],
      cli_grant_ttl_hours: 30,
      effective_from: null,
      expires_at: null,
    })).toEqual({
      configured: true,
      version: 3,
      mode: 'disabled',
      toolScopes: [],
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours: 30,
    })
  })

  it('keeps only a bounded opaque authorization request id', () => {
    expect(parseAuthorizeSearch({ request_id: 'request_123' })).toEqual({ request_id: 'request_123' })
    expect(parseAuthorizeSearch({ request_id: '../../token' })).toEqual({})
    expect(parseAuthorizeSearch({ channel: 'mcp' })).toEqual({ channel: 'mcp' })
    expect(parseAuthorizeSearch({ channel: 'smtp' })).toEqual({})
  })

  it('accepts a complete loopback OAuth authorization request and drops partial or unsafe input', () => {
    const parsed = parseAuthorizeSearch({
      response_type: 'code',
      client_id: 'pm-cli',
      redirect_uri: 'http://127.0.0.1:43123/callback',
      code_challenge: 'A'.repeat(43),
      code_challenge_method: 'S256',
      state: 'opaque-state',
      resource: 'http://localhost:8081/api/v1/tool-runtime',
      scope: 'offline_access pm:tools',
      channel: 'cli',
    })
    expect(oauthAuthorizeParameters(parsed)).toEqual({
      responseType: 'code', clientId: 'pm-cli', redirectUri: 'http://127.0.0.1:43123/callback',
      codeChallenge: 'A'.repeat(43), codeChallengeMethod: 'S256', state: 'opaque-state',
      resource: 'http://localhost:8081/api/v1/tool-runtime', scope: 'pm:tools offline_access', channel: 'cli',
    })
    expect(parseAuthorizeSearch({
      response_type: 'code', client_id: 'pm-cli', redirect_uri: 'https://attacker.example/callback',
    })).toEqual({})
  })

  it('normalizes bounded device codes and complete device authorization facts', () => {
    expect(parseDeviceAuthorizeSearch({ user_code: 'abcd-efgh' })).toEqual({
      user_code: 'ABCD-EFGH',
    })
    expect(parseDeviceAuthorizeSearch({ user_code: '../../secret' })).toEqual({})
    expect(parseDeviceAuthorizeSearch({ user_code: 'AB10-EFGH' })).toEqual({})
    expect(parseDeviceAuthorizeSearch({ user_code: 'ABCD-EFGH-IJKL' })).toEqual({})
    expect(normalizeDeviceAuthorization({
      user_code: 'ABCD-EFGH',
      status: 'pending',
      client: { id: 'pm-cli', name: 'PM CLI' },
      channel: 'cli',
      workspace: { id: 7, name: 'JiGuang' },
      mode: 'window',
      tools: [tool],
      effective_from: '2026-08-27T01:00:00Z',
      expires_at: '2026-09-03T01:00:00Z',
      request_expires_at: '2026-08-27T01:10:00Z',
    })).toMatchObject({
      id: 'ABCD-EFGH',
      userCode: 'ABCD-EFGH',
      status: 'pending',
      clientId: 'pm-cli',
      workspaceId: 7,
      requestExpiresAt: '2026-08-27T01:10:00Z',
    })
    expect(normalizeDeviceAuthorization({
      user_code: 'ABCD-EFGH',
      status: 'unknown',
    })).toBeUndefined()
  })

  it('uses device authorization only in the Agent Prompt command and preserves legacy setup', () => {
    expect(buildCliAuthConfiguration(7)).toContain('--auth-mode device')
    expect(buildCliConfiguration(7)).not.toContain('--auth-mode device')
  })

  it('generates the MCP endpoint without embedding credentials', () => {
    window.__RUNTIME_CONFIG__ = { PM_API_BASE_URL: 'https://pm.example.test/api/v1' }
    const configuration = buildMcpConfiguration()
    expect(configuration).toContain('https://pm.example.test/api/v1/mcp')
    expect(configuration).not.toMatch(/token|authorization/i)
    delete window.__RUNTIME_CONFIG__
  })

  it('generates native configuration for mainstream MCP clients', () => {
    window.__RUNTIME_CONFIG__ = { PM_API_BASE_URL: 'https://pm.example.test/api/v1' }

    expect(buildMcpConfiguration('codex')).toContain('[mcp_servers.pm]')
    expect(buildMcpConfiguration('claude_code')).toContain('"mcpServers"')
    expect(buildMcpConfiguration('goose')).toContain('type: streamable_http')
    expect(buildMcpConfiguration('opencode')).toContain('"type": "remote"')

    delete window.__RUNTIME_CONFIG__
  })
})
