export const RESOURCE_ACCESS_MODES = [
  'private',
  'restricted',
  'workspace',
  'link',
  'legacy',
] as const

export type ResourceAccessMode = (typeof RESOURCE_ACCESS_MODES)[number]

export const RESOURCE_ACCESS_LEVELS = ['viewer', 'editor'] as const

export type ResourceAccessLevel = (typeof RESOURCE_ACCESS_LEVELS)[number]

export const RESOURCE_ACCESS_SUBJECT_KINDS = [
  'user',
  'role',
  'user_group',
  'organization_unit',
  'project_set',
] as const

export type ResourceAccessSubjectKind = (typeof RESOURCE_ACCESS_SUBJECT_KINDS)[number]

export type ResourceAccessSubjectCapability = {
  available: boolean
}

export type ResourceAccessSubjectCapabilities = Record<
  ResourceAccessSubjectKind,
  ResourceAccessSubjectCapability
>

export type ResourceAccessSubject = {
  id: string
  kind: ResourceAccessSubjectKind
  name: string
  secondary?: string
}

export type ResourceAccessRule = {
  subjectKind: ResourceAccessSubjectKind
  subjectId: string
  level: ResourceAccessLevel
  includeDescendants: boolean
  subject?: ResourceAccessSubject
}

export type ResourceAccessPolicy = {
  resourceId: string
  mode: ResourceAccessMode
  workspaceLevel: ResourceAccessLevel | null
  rules: readonly ResourceAccessRule[]
  policyRevision: number
  canManageAccess: boolean
  effectiveLevel: ResourceAccessLevel | null
  effectiveSources: readonly string[]
  subjectCapabilities: ResourceAccessSubjectCapabilities
}

export type ResourceAccessDraft = {
  mode: Exclude<ResourceAccessMode, 'legacy'> | 'legacy'
  workspaceLevel: ResourceAccessLevel
  rules: readonly ResourceAccessRule[]
}

export type ResourceAccessPolicyReplacement = {
  mode: Exclude<ResourceAccessMode, 'legacy'>
  workspaceLevel?: ResourceAccessLevel
  rules: readonly ResourceAccessRule[]
  expectedPolicyRevision: number
  idempotencyKey: string
}

export type ResourceAccessSubjectPage = {
  items: readonly ResourceAccessSubject[]
  total: number
  hasMore: boolean
  capabilities: ResourceAccessSubjectCapabilities
}

export type ShareLinkExpiry = '24h' | '7d' | 'never'

export type ResourceShareLink = {
  id: string
  level: ResourceAccessLevel
  expiresAt: string | null
  createdAt: string
  updatedAt?: string
  revokedAt: string | null
}

export type ResourceShareLinkSecret = ResourceShareLink & {
  secret: string
}

export type ResourceAccessGrant = {
  resourceId: string
  accessSecret: string
  level: ResourceAccessLevel
  expiresAt: string
}

export type ResourceShareLinkCreate = {
  level: ResourceAccessLevel
  expiresIn: ShareLinkExpiry
  idempotencyKey: string
}

export type ResourceAccessQueryState<T> =
  | { status: 'loading' }
  | { status: 'empty' }
  | { status: 'error'; retry: () => Promise<T | undefined> }
  | {
    status: 'ready'
    data: T
    refresh: 'idle' | 'refreshing' | 'stale_error'
    retry: () => Promise<T | undefined>
  }
