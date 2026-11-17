export type ToolRisk = 'read' | 'mutation' | 'high_impact'
export type ToolLifecycle = 'draft' | 'active' | 'deprecated'
export type ExternalChannel = 'mcp' | 'cli'
export type GrantStatus = 'scheduled' | 'active' | 'expired' | 'revoked'
export type ExternalToolingEffectMode = 'permanent' | 'disabled' | 'window'
export type EffectiveAccessStatus = 'unconfigured' | 'disabled' | 'scheduled' | 'active' | 'expired'
export const DEFAULT_CLI_GRANT_TTL_HOURS = 168
export const MAX_CLI_GRANT_TTL_HOURS = 8760

export type OAuthAuthorizeParameters = {
  responseType: 'code'
  clientId: string
  redirectUri: string
  codeChallenge: string
  codeChallengeMethod: 'S256'
  state: string
  resource: string
  scope: string
  channel?: ExternalChannel
}

export type AuthorizeSearch = {
  request_id?: string
  channel?: ExternalChannel
  response_type?: 'code'
  client_id?: string
  redirect_uri?: string
  code_challenge?: string
  code_challenge_method?: 'S256'
  state?: string
  resource?: string
  scope?: string
}

export type DeviceAuthorizeSearch = {
  user_code?: string
}

export type DeviceAuthorizationStatus =
  | 'pending'
  | 'approved'
  | 'denied'
  | 'consumed'
  | 'expired'

export type ToolCatalogItem = {
  name: string
  version: string
  summary: string
  lifecycle: ToolLifecycle
  risk: ToolRisk
  grantScope: string
}

export type ExternalGrant = {
  id: string
  clientName: string
  channel: ExternalChannel
  status: GrantStatus
  toolScopes: string[]
  effectiveFrom: string
  expiresAt: string
}

export type ExternalToolingConfiguration = {
  configured: boolean
  version: number
  mode: ExternalToolingEffectMode
  toolScopes: string[]
  effectiveFrom: string
  expiresAt: string
  cliGrantTtlHours: number
}

export type EffectiveToolingAccess = {
  configured: boolean
  mode: ExternalToolingEffectMode
  status: EffectiveAccessStatus
  tools: ToolCatalogItem[]
  effectiveFrom: string
  expiresAt: string
  cliGrantTtlHours: number
}

export type AuthorizationRequest = {
  id: string
  clientId: string
  clientName: string
  channel: ExternalChannel
  workspaceId: number
  workspaceName: string
  mode: Exclude<ExternalToolingEffectMode, 'disabled'>
  tools: ToolCatalogItem[]
  effectiveFrom: string | null
  expiresAt: string | null
  oauth: boolean
}

export type DeviceAuthorization = AuthorizationRequest & {
  userCode: string
  status: DeviceAuthorizationStatus
  requestExpiresAt: string
}
