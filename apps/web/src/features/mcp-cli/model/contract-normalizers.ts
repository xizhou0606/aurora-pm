import type {
  ExternalToolingConfiguration,
  AuthorizationRequest,
  AuthorizeSearch,
  OAuthAuthorizeParameters,
  ToolCatalogItem,
  EffectiveToolingAccess,
  ExternalGrant,
} from './contract-types'
import {
  DEFAULT_CLI_GRANT_TTL_HOURS,
  MAX_CLI_GRANT_TTL_HOURS,
} from './contract-types'

import {
  safeLoopbackRedirect,
  safeOAuthResource,
  safeOAuthState,
  safeOAuthScope,
} from './contract-oauth-validators'

type RecordValue = Record<string, unknown>

function record(value: unknown): RecordValue | undefined {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as RecordValue
    : undefined
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function oneOf<T extends string>(value: unknown, values: readonly T[]): T | undefined {
  const candidate = text(value) as T
  return values.includes(candidate) ? candidate : undefined
}

function positiveInteger(value: unknown): number | undefined {
  const candidate = Number(value)
  return Number.isInteger(candidate) && candidate > 0 ? candidate : undefined
}

function isoDateTime(value: unknown): string | undefined {
  const candidate = text(value)
  return candidate && Number.isFinite(Date.parse(candidate)) ? candidate : undefined
}

export function normalizeToolCatalog(value: unknown): ToolCatalogItem[] | undefined {
  const container = record(value)
  const rows = Array.isArray(value)
    ? value
    : Array.isArray(container?.tools)
      ? container.tools
      : undefined
  if (!rows) return undefined
  const tools = rows.map((raw) => {
    const item = record(raw)
    const authorization = record(item?.authorization)
    const name = text(item?.name)
    const version = text(item?.version)
    const summary = text(item?.summary)
    const lifecycle = oneOf(item?.lifecycle, ['draft', 'active', 'deprecated'] as const)
    const risk = oneOf(item?.risk, ['read', 'mutation', 'high_impact'] as const)
    const grantScope = text(authorization?.grant_scope ?? item?.grant_scope)
    if (!name || !version || !summary || !lifecycle || !risk || !grantScope) return undefined
    return { name, version, summary, lifecycle, risk, grantScope }
  })
  return tools.every(Boolean) ? tools as ToolCatalogItem[] : undefined
}

export function emptyExternalToolingConfiguration(): ExternalToolingConfiguration {
  return {
    configured: false,
    version: 0,
    mode: 'disabled',
    toolScopes: [],
    effectiveFrom: '',
    expiresAt: '',
    cliGrantTtlHours: DEFAULT_CLI_GRANT_TTL_HOURS,
  }
}

export function normalizeExternalToolingConfiguration(
  value: unknown,
): ExternalToolingConfiguration | undefined {
  const item = record(value)
  if (!item || !Array.isArray(item.tool_scopes)) return undefined
  const scopes = item.tool_scopes.map(text)
  if (scopes.some((scope) => !scope) || new Set(scopes).size !== scopes.length) return undefined
  const mode = oneOf(item.mode, ['permanent', 'disabled', 'window'] as const)
  const version = positiveInteger(item.version)
  const cliGrantTtlHours = positiveInteger(item.cli_grant_ttl_hours)
  if (!mode || !version || !cliGrantTtlHours || cliGrantTtlHours > MAX_CLI_GRANT_TTL_HOURS) return undefined
  if (mode === 'disabled') {
    return {
      configured: true,
      version,
      mode,
      toolScopes: scopes,
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours,
    }
  }
  if (!scopes.length) return undefined
  if (mode === 'permanent') {
    return {
      configured: true,
      version,
      mode,
      toolScopes: scopes,
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours,
    }
  }
  const effectiveFrom = isoDateTime(item.effective_from)
  const expiresAt = isoDateTime(item.expires_at)
  if (!effectiveFrom || !expiresAt) return undefined
  return {
    configured: true,
    version,
    mode,
    toolScopes: scopes,
    effectiveFrom,
    expiresAt,
    cliGrantTtlHours,
  }
}

export function normalizeAuthorizationRequest(value: unknown): AuthorizationRequest | undefined {
  const item = record(value)
  const client = record(item?.client)
  const workspace = record(item?.workspace)
  const id = text(item?.id)
  const clientId = text(client?.id)
  const clientName = text(client?.name)
  const channel = oneOf(item?.channel, ['mcp', 'cli'] as const)
  const workspaceId = positiveInteger(workspace?.id)
  const workspaceName = text(workspace?.name)
  const mode = oneOf(item?.mode, ['permanent', 'window'] as const)
  const tools = normalizeToolCatalog(item?.tools)
  const oauth = item?.oauth === true
  const hasFiniteGrantWindow = mode === 'window' || channel === 'cli'
  const effectiveFrom = hasFiniteGrantWindow ? isoDateTime(item?.effective_from) : null
  const expiresAt = hasFiniteGrantWindow ? isoDateTime(item?.expires_at) : null
  if (!id || !clientId || !clientName || !channel || !workspaceId || !workspaceName || !mode || !tools?.length) {
    return undefined
  }
  if (hasFiniteGrantWindow && (!effectiveFrom || !expiresAt)) return undefined
  return {
    id, clientId, clientName, channel, workspaceId, workspaceName, mode, tools,
    effectiveFrom: effectiveFrom ?? null,
    expiresAt: expiresAt ?? null,
    oauth,
  }
}

export function normalizeToolCatalogItem(value: unknown): ToolCatalogItem | undefined {
  const item = record(value)
  const authorization = record(item?.authorization)
  const name = text(item?.name)
  const version = text(item?.version)
  const summary = text(item?.summary)
  const lifecycle = oneOf(item?.lifecycle, ['draft', 'active', 'deprecated'] as const)
  const risk = oneOf(item?.risk, ['read', 'mutation', 'high_impact'] as const)
  const grantScope = text(authorization?.grant_scope ?? item?.grant_scope)
  if (!name || !version || !summary || !lifecycle || !risk || !grantScope) return undefined
  return { name, version, summary, lifecycle, risk, grantScope }
}

export function normalizeEffectiveToolingAccess(value: unknown): EffectiveToolingAccess | undefined {
  const item = record(value)
  if (!item || typeof item.configured !== 'boolean') return undefined
  const mode = oneOf(item.mode, ['permanent', 'disabled', 'window'] as const)
  const status = oneOf(item.status, ['unconfigured', 'disabled', 'scheduled', 'active', 'expired'] as const)
  const tools = normalizeToolCatalog({ tools: item.tools })
  const cliGrantTtlHours = positiveInteger(item.cli_grant_ttl_hours)
  if (!mode || !status || !tools || tools.some((tool) => tool.lifecycle === 'draft')) return undefined
  if (!item.configured) {
    return mode === 'disabled' && status === 'unconfigured' && tools.length === 0
      ? {
          configured: false,
          mode,
          status,
          tools,
          effectiveFrom: '',
          expiresAt: '',
          cliGrantTtlHours: DEFAULT_CLI_GRANT_TTL_HOURS,
        }
      : undefined
  }
  if (!cliGrantTtlHours || cliGrantTtlHours > MAX_CLI_GRANT_TTL_HOURS) return undefined
  if (mode === 'window') {
    const effectiveFrom = isoDateTime(item.effective_from)
    const expiresAt = isoDateTime(item.expires_at)
    if (!effectiveFrom || !expiresAt || status === 'disabled' || status === 'unconfigured') return undefined
    return { configured: true, mode, status, tools, effectiveFrom, expiresAt, cliGrantTtlHours }
  }
  if (item.effective_from != null || item.expires_at != null) return undefined
  if (mode === 'disabled' && status !== 'disabled') return undefined
  if (mode === 'permanent' && status !== 'active') return undefined
  return { configured: true, mode, status, tools, effectiveFrom: '', expiresAt: '', cliGrantTtlHours }
}

export function normalizeExternalGrants(value: unknown): ExternalGrant[] | undefined {
  const container = record(value)
  const rows = Array.isArray(value)
    ? value
    : Array.isArray(container?.grants)
      ? container.grants
      : undefined
  if (!rows) return undefined
  const grants = rows.map((raw) => {
    const item = record(raw)
    const id = text(item?.id)
    const clientName = text(item?.client_name)
    const channel = oneOf(item?.channel, ['mcp', 'cli'] as const)
    const status = oneOf(item?.status, ['scheduled', 'active', 'expired', 'revoked'] as const)
    const scopes = Array.isArray(item?.tool_scopes)
      ? item.tool_scopes.map(text).filter(Boolean)
      : undefined
    const effectiveFrom = isoDateTime(item?.effective_from)
    const expiresAt = isoDateTime(item?.expires_at)
    if (!id || !clientName || !channel || !status || !scopes || !effectiveFrom || !expiresAt) return undefined
    return { id, clientName, channel, status, toolScopes: scopes, effectiveFrom, expiresAt }
  })
  return grants.every(Boolean) ? grants as ExternalGrant[] : undefined
}

export function normalizeRequestId(value: unknown): string | undefined {
  const item = record(value)
  const requestId = text(item?.request_id)
  return /^[A-Za-z0-9_-]{1,128}$/.test(requestId) ? requestId : undefined
}

export function parseAuthorizeSearch(search: Record<string, unknown>): AuthorizeSearch {
  const requestId = text(search.request_id)
  const result: AuthorizeSearch = {}
  if (/^[A-Za-z0-9_-]{1,128}$/.test(requestId)) result.request_id = requestId
  const channel = oneOf(search.channel, ['mcp', 'cli'] as const)
  if (channel) result.channel = channel
  const oauthKeys = [
    'response_type', 'client_id', 'redirect_uri', 'code_challenge',
    'code_challenge_method', 'state', 'resource', 'scope',
  ] as const
  if (!oauthKeys.some((key) => search[key] !== undefined)) return result
  const responseType = oneOf(search.response_type, ['code'] as const)
  const clientId = text(search.client_id)
  const redirectUri = safeLoopbackRedirect(search.redirect_uri)
  const codeChallenge = text(search.code_challenge)
  const codeChallengeMethod = oneOf(search.code_challenge_method, ['S256'] as const)
  const state = safeOAuthState(search.state)
  const resource = safeOAuthResource(search.resource)
  const scope = safeOAuthScope(search.scope)
  if (
    !responseType
    || !clientId
    || clientId.length > 255
    || !redirectUri
    || !/^[A-Za-z0-9_-]{43}$/.test(codeChallenge)
    || !codeChallengeMethod
    || !state
    || !resource
    || !scope
  ) return result.request_id ? { request_id: result.request_id } : {}
  return {
    ...result,
    response_type: responseType,
    client_id: clientId,
    redirect_uri: redirectUri,
    code_challenge: codeChallenge,
    code_challenge_method: codeChallengeMethod,
    state,
    resource,
    scope,
  }
}

export function oauthAuthorizeParameters(search: AuthorizeSearch): OAuthAuthorizeParameters | undefined {
  if (
    !search.response_type || !search.client_id || !search.redirect_uri || !search.code_challenge
    || !search.code_challenge_method || !search.state || !search.resource || !search.scope
  ) return undefined
  return {
    responseType: search.response_type,
    clientId: search.client_id,
    redirectUri: search.redirect_uri,
    codeChallenge: search.code_challenge,
    codeChallengeMethod: search.code_challenge_method,
    state: search.state,
    resource: search.resource,
    scope: search.scope,
    channel: search.channel,
  }
}
