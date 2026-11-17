import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

import {
  normalizeAuthorizationRequest,
  emptyExternalToolingConfiguration,
  normalizeExternalGrants,
  normalizeExternalToolingConfiguration,
  normalizeEffectiveToolingAccess,
  normalizeRequestId,
  normalizeToolCatalog,
  type AuthorizationRequest,
  type ExternalChannel,
  type ExternalGrant,
  type ExternalToolingEffectMode,
  type ExternalToolingConfiguration,
  type EffectiveToolingAccess,
  type OAuthAuthorizeParameters,
  type ToolCatalogItem,
} from '../model/contract'

export type AuthorizationRequestInput = {
  channel?: ExternalChannel
  oauth?: {
    response_type: OAuthAuthorizeParameters['responseType']
    client_id: string
    redirect_uri: string
    code_challenge: string
    code_challenge_method: OAuthAuthorizeParameters['codeChallengeMethod']
    state: string
    resource: string
    scope: string
  }
}

export async function fetchEffectiveToolingAccess(signal?: AbortSignal): Promise<EffectiveToolingAccess> {
  try {
    const response = await platformApi.get('/external-tooling/effective-access', { signal })
    return normalizeEffectiveToolingAccess(unwrapApiData(response))
      ?? contractError('EXTERNAL_TOOLING_EFFECTIVE_ACCESS_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'EXTERNAL_TOOLING_EFFECTIVE_ACCESS_LOAD_FAILED')
  }
}

export type ExternalToolingConfigurationInput = {
  mode: ExternalToolingEffectMode
  tool_scopes: string[]
  effective_from: string | null
  expires_at: string | null
  cli_grant_ttl_hours: number
}

function contractError(code: string): never {
  throw toSafeApiError(new Error('invalid external tooling response'), code)
}

export async function fetchToolCatalog(signal?: AbortSignal): Promise<ToolCatalogItem[]> {
  try {
    const response = await platformApi.get('/tool-runtime/catalog', { signal })
    return normalizeToolCatalog(unwrapApiData(response)) ?? contractError('TOOL_CATALOG_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'TOOL_CATALOG_LOAD_FAILED')
  }
}

export async function fetchExternalGrants(signal?: AbortSignal): Promise<ExternalGrant[]> {
  try {
    const response = await platformApi.get('/external-tooling/grants', { signal })
    return normalizeExternalGrants(unwrapApiData(response)) ?? contractError('EXTERNAL_GRANTS_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'EXTERNAL_GRANTS_LOAD_FAILED')
  }
}

export async function fetchExternalToolingConfiguration(
  signal?: AbortSignal,
): Promise<ExternalToolingConfiguration> {
  try {
    const response = await platformApi.get('/external-tooling/configuration', { signal })
    return normalizeExternalToolingConfiguration(unwrapApiData(response))
      ?? contractError('EXTERNAL_TOOLING_CONFIGURATION_CONTRACT_INVALID')
  } catch (error) {
    const safeError = toSafeApiError(error, 'EXTERNAL_TOOLING_CONFIGURATION_LOAD_FAILED')
    if (
      safeError.status === 404
      && safeError.code === 'EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND'
    ) return emptyExternalToolingConfiguration()
    throw safeError
  }
}

export async function updateExternalToolingConfiguration(
  input: ExternalToolingConfigurationInput,
): Promise<ExternalToolingConfiguration> {
  try {
    const response = await platformApi.put('/external-tooling/configuration', input)
    return normalizeExternalToolingConfiguration(unwrapApiData(response))
      ?? contractError('EXTERNAL_TOOLING_CONFIGURATION_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'EXTERNAL_TOOLING_CONFIGURATION_SAVE_FAILED')
  }
}

export async function startAuthorization(input: AuthorizationRequestInput): Promise<string> {
  try {
    const response = await platformApi.post('/external-tooling/authorization-requests', input)
    return normalizeRequestId(unwrapApiData(response)) ?? contractError('AUTHORIZATION_REQUEST_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'AUTHORIZATION_REQUEST_START_FAILED')
  }
}

export async function fetchAuthorizationRequest(
  requestId: string,
  signal?: AbortSignal,
): Promise<AuthorizationRequest> {
  try {
    const response = await platformApi.get(
      `/external-tooling/authorization-requests/${encodeURIComponent(requestId)}`,
      { signal },
    )
    return normalizeAuthorizationRequest(unwrapApiData(response)) ?? contractError('AUTHORIZATION_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'AUTHORIZATION_REQUEST_LOAD_FAILED')
  }
}

export async function decideAuthorization(
  requestId: string,
  decision: 'authorize' | 'deny',
  oauth = false,
): Promise<string | undefined> {
  try {
    const response = await platformApi.post(
      `/external-tooling/authorization-requests/${encodeURIComponent(requestId)}/decision`,
      { decision, oauth },
    )
    if (!oauth) return undefined
    const payload = unwrapApiData(response)
    const redirect = payload && typeof payload === 'object' && !Array.isArray(payload)
      ? String((payload as Record<string, unknown>).authorization_redirect ?? '').trim()
      : ''
    try {
      const url = new URL(redirect)
      const host = url.hostname.toLowerCase()
      if (
        url.protocol !== 'http:'
        || !['localhost', '127.0.0.1', '[::1]'].includes(host)
        || url.username
        || url.password
        || url.hash
      ) throw new Error('invalid OAuth callback')
      return url.toString()
    } catch {
      return contractError('AUTHORIZATION_REDIRECT_CONTRACT_INVALID')
    }
  } catch (error) {
    throw toSafeApiError(error, 'AUTHORIZATION_DECISION_FAILED')
  }
}
