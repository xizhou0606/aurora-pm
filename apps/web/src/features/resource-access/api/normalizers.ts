import { PMSafeApiError } from '@/shared/api'
import type { AccessRequest, AccessRequestContext } from '../model/requests'

export function normalizeRequestContext(value: unknown): AccessRequestContext {
  const dto = object(value)
  if (!Array.isArray(dto.items) || typeof dto.can_request !== 'boolean' || typeof dto.can_review !== 'boolean' || typeof dto.has_access !== 'boolean') return invalid()
  const items = dto.items.map((value): AccessRequest => {
    const item = object(value)
    if (!isOneOf(item.status, ['pending', 'approved', 'rejected', 'revoked', 'expired'] as const) || typeof item.read !== 'boolean' || typeof item.reason !== 'string') return invalid()
    const grantLevel = item.grant_level ?? 'editor'
    if (!isOneOf(grantLevel, ['viewer', 'editor'] as const)) return invalid()
    return { id: nonNegativeInteger(item.id), grantLevel, grantExpiresAt: nullableISO(item.grant_expires_at), applicantUserId: nonNegativeInteger(item.applicant_user_id), applicantName: text(item.applicant_name), reason: item.reason, status: item.status, read: item.read, createdAt: nullableISO(item.created_at) ?? invalid(), expiresAt: nullableISO(item.expires_at) ?? invalid() }
  })
  const resourceType = dto.resource_type ?? 'rich_text'
  if (!isOneOf(resourceType, ['rich_text', 'bitable'] as const)) return invalid()
  const ownerUserId = nonNegativeInteger(dto.owner_user_id)
  const ownerName = ownerUserId === 0 && dto.owner_name === '' ? '' : text(dto.owner_name)
  return { resourceId: opaqueId(dto.resource_id), resourceType, ownerUserId, ownerName, canRequest: dto.can_request && ownerUserId > 0, canReview: dto.can_review, hasAccess: dto.has_access, hasMore: dto.has_more === true, pendingCount: nonNegativeInteger(dto.pending_count), items }
}

import {
  RESOURCE_ACCESS_LEVELS,
  RESOURCE_ACCESS_MODES,
  RESOURCE_ACCESS_SUBJECT_KINDS,
  unavailableSubjectCapabilities,
  type ResourceAccessGrant,
  type ResourceAccessLevel,
  type ResourceAccessMode,
  type ResourceAccessPolicy,
  type ResourceAccessRule,
  type ResourceAccessSubject,
  type ResourceAccessSubjectCapabilities,
  type ResourceAccessSubjectKind,
  type ResourceAccessSubjectPage,
  type ResourceShareLink,
  type ResourceShareLinkSecret,
} from '../model'
import type {
  ResourceAccessGrantDTO,
  ResourceAccessPolicyDTO,
  ResourceAccessRuleDTO,
  ResourceAccessSubjectDTO,
  ResourceAccessSubjectPageDTO,
  ResourceShareLinkDTO,
} from './contracts'

function invalid(): never {
  throw new PMSafeApiError('RESOURCE_ACCESS_CONTRACT_INVALID')
}

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return invalid()
  return value as Record<string, unknown>
}

function text(value: unknown): string {
  return typeof value === 'string' && value.trim() ? value.trim() : invalid()
}

function optionalText(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function opaqueId(value: unknown): string {
  const id = typeof value === 'string' ? value : ''
  return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$/.test(id) ? id : invalid()
}

function nonNegativeInteger(value: unknown): number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : invalid()
}

function nullableISO(value: unknown): string | null {
  if (value === null || value === undefined || value === '') return null
  const iso = text(value)
  return Number.isNaN(Date.parse(iso)) ? invalid() : iso
}

function isOneOf<T extends string>(value: unknown, values: readonly T[]): value is T {
  return typeof value === 'string' && values.includes(value as T)
}

function level(value: unknown, nullable = false): ResourceAccessLevel | null {
  if (nullable && (value === null || value === undefined || value === '')) return null
  return isOneOf(value, RESOURCE_ACCESS_LEVELS) ? value : invalid()
}

function kind(value: unknown): ResourceAccessSubjectKind {
  return isOneOf(value, RESOURCE_ACCESS_SUBJECT_KINDS) ? value : invalid()
}

function mode(value: unknown): ResourceAccessMode {
  return isOneOf(value, RESOURCE_ACCESS_MODES) ? value : invalid()
}

export function normalizeSubject(value: unknown): ResourceAccessSubject {
  const dto = object(value) as ResourceAccessSubjectDTO
  const secondary = optionalText(dto.secondary_text)
  return {
    id: opaqueId(dto.id),
    kind: kind(dto.kind),
    name: text(dto.display_name),
    ...(secondary ? { secondary } : {}),
  }
}

function normalizeRule(value: unknown): ResourceAccessRule {
  const dto = object(value) as ResourceAccessRuleDTO
  const subject = dto.subject === undefined ? undefined : normalizeSubject(dto.subject)
  const subjectKind = kind(dto.subject_kind)
  const subjectId = opaqueId(dto.subject_id)
  if (subject && (subject.kind !== subjectKind || subject.id !== subjectId)) return invalid()
  return {
    subjectKind,
    subjectId,
    level: level(dto.level) ?? invalid(),
    includeDescendants: dto.include_descendants === true,
    ...(subject ? { subject } : {}),
  }
}

export function normalizeSubjectCapabilities(value: unknown): ResourceAccessSubjectCapabilities {
  const source = value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {}
  const capabilities = unavailableSubjectCapabilities()
  for (const subjectKind of RESOURCE_ACCESS_SUBJECT_KINDS) {
    const raw = source[subjectKind]
    if (typeof raw === 'boolean') capabilities[subjectKind] = { available: raw }
    else if (raw && typeof raw === 'object') {
      capabilities[subjectKind] = { available: (raw as { available?: unknown }).available === true }
    }
  }
  return capabilities
}

export function normalizePolicy(value: unknown): ResourceAccessPolicy {
  const dto = object(value) as ResourceAccessPolicyDTO
  const accessMode = mode(dto.mode)
  const rules = Array.isArray(dto.rules) ? dto.rules.map(normalizeRule) : invalid()
  if (accessMode !== 'restricted' && rules.length > 0) return invalid()
  const workspaceLevel = level(dto.workspace_level, true)
  if (accessMode === 'workspace' && !workspaceLevel) return invalid()
  const sources = Array.isArray(dto.effective_sources)
    ? dto.effective_sources.map(text)
    : invalid()
  return {
    resourceId: text(dto.resource_id),
    mode: accessMode,
    workspaceLevel,
    rules,
    policyRevision: nonNegativeInteger(dto.policy_revision),
    canManageAccess: dto.can_manage_access === true,
    effectiveLevel: level(dto.effective_level, true),
    effectiveSources: sources,
    subjectCapabilities: normalizeSubjectCapabilities(dto.subject_capabilities),
  }
}

export function normalizeSubjectPage(value: unknown): ResourceAccessSubjectPage {
  const dto = object(value) as ResourceAccessSubjectPageDTO
  const items = Array.isArray(dto.items) ? dto.items.map(normalizeSubject) : invalid()
  const total = nonNegativeInteger(dto.total)
  return {
    items,
    total,
    hasMore: typeof dto.has_more === 'boolean' ? dto.has_more : items.length < total,
    capabilities: normalizeSubjectCapabilities(dto.subject_capabilities ?? dto.capabilities),
  }
}

export function normalizeShareLink(value: unknown): ResourceShareLink {
  const dto = object(value) as ResourceShareLinkDTO
  return {
    id: text(dto.id),
    level: level(dto.level) ?? invalid(),
    expiresAt: nullableISO(dto.expires_at),
    createdAt: nullableISO(dto.created_at) ?? invalid(),
    ...(dto.updated_at ? { updatedAt: nullableISO(dto.updated_at) ?? invalid() } : {}),
    revokedAt: nullableISO(dto.revoked_at),
  }
}

export function normalizeShareLinkSecret(value: unknown): ResourceShareLinkSecret {
  const dto = object(value) as ResourceShareLinkDTO
  return { ...normalizeShareLink(dto), secret: dto.secret === '' ? '' : text(dto.secret) }
}

export function normalizeShareLinks(value: unknown): readonly ResourceShareLink[] {
  const items = Array.isArray(value) ? value : object(value).items
  return Array.isArray(items) ? items.map(normalizeShareLink) : invalid()
}

export function normalizeAccessGrant(value: unknown): ResourceAccessGrant {
  const dto = object(value) as ResourceAccessGrantDTO
  const accessSecret = text(dto.access_secret)
  const expiresAt = nullableISO(dto.expires_at)
  if (accessSecret.length > 256 || /[\r\n]/.test(accessSecret) || !expiresAt) return invalid()
  return {
    resourceId: opaqueId(dto.resource_id),
    accessSecret,
    level: level(dto.level) ?? invalid(),
    expiresAt,
  }
}
