import type {
  AccessGroup,
  AccessRole,
  FeishuOrgSyncStats,
  FeishuOrgSyncStatus,
  ManagedUser,
  PermissionNode,
} from '../model'
import { isAccountStatus } from '../model'
import type {
  AccessGroupDTO,
  AccessRoleDTO,
  ManagedUserDTO,
  PermissionNodeDTO,
} from './contracts'

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {}
}

function positiveId(value: unknown): number | null {
  const id = Number(value)
  return Number.isSafeInteger(id) && id > 0 ? id : null
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function codes(value: unknown): string[] {
  return Array.isArray(value)
    ? [...new Set(value.map(text).filter(Boolean))]
    : []
}

export function normalizeRole(value: unknown): AccessRole | null {
  const dto = record(value) as AccessRoleDTO
  const id = positiveId(dto.id)
  const code = text(dto.code)
  const name = text(dto.name)
  if (!id || !code || !name) return null
  return {
    id,
    code,
    name,
    description: text(dto.description),
    enabled: dto.enabled === true,
    systemManaged: dto.system_managed === true,
    permissionCodes: codes(dto.action_codes ?? dto.permission_codes),
  }
}

export function normalizeGroup(value: unknown): AccessGroup | null {
  const dto = record(value) as AccessGroupDTO
  const role = normalizeRole(dto)
  if (!role) return null
  return {
    ...role,
    systemManaged: dto.system_managed === true,
    isDefault: dto.is_default === true || role.code === 'default-users',
  }
}

export function normalizeUser(
  value: unknown,
  groups: readonly AccessGroup[] = [],
  directoryRoles: readonly AccessRole[] = [],
): ManagedUser | null {
  const dto = record(value) as ManagedUserDTO
  const id = positiveId(dto.id)
  if (!id || !isAccountStatus(dto.status)) return null
  const group = normalizeGroup(dto.group)
    ?? groups.find((candidate) => candidate.id === positiveId(dto.group_id))
    ?? null
  const embeddedRoles = Array.isArray(dto.roles)
    ? dto.roles.map(normalizeRole).filter((role): role is AccessRole => Boolean(role))
    : []
  const referencedRoleIds = Array.isArray(dto.role_ids)
    ? dto.role_ids.map(positiveId).filter((roleId): roleId is number => Boolean(roleId))
    : []
  const roles = embeddedRoles.length > 0
    ? embeddedRoles
    : directoryRoles.filter((role) => referencedRoleIds.includes(role.id))
  return {
    id,
    name: text(dto.display_name) || text(dto.name),
    username: text(dto.username),
    email: text(dto.email),
    ...(text(dto.avatar_url) ? { avatarUrl: text(dto.avatar_url) } : {}),
    status: dto.status,
    groupId: positiveId(dto.group_id) ?? group?.id ?? null,
    roleIds: [...new Set([...referencedRoleIds, ...embeddedRoles.map((role) => role.id)])],
    group: group ? {
      id: group.id,
      code: group.code,
      name: group.name,
      systemManaged: group.systemManaged,
      isDefault: group.isDefault,
    } : null,
    roles: roles.map(({ id: roleId, code, name }) => ({ id: roleId, code, name })),
  }
}

export function normalizePermissionNode(value: unknown): PermissionNode | null {
  const dto = record(value) as PermissionNodeDTO
  const code = text(dto.code)
  const name = text(dto.name)
  const type = dto.node_type ?? dto.type
  if (!code || !name || (type !== 'page' && type !== 'view' && type !== 'module' && type !== 'action')) return null
  const children = Array.isArray(dto.children)
    ? dto.children.map(normalizePermissionNode).filter((node): node is PermissionNode => Boolean(node))
    : []
  if (type === 'action' && children.length > 0) return null
  const grantable = type === 'action'
    ? dto.grantable === true
    : children.some((child) => child.grantable)
  const tools = type === 'action' && Array.isArray(dto.tools)
    ? [...new Set(dto.tools.map(text).filter(Boolean))]
    : []
  const rawObjectPolicy = record(dto.object_policy)
  const objectPolicyCode = text(rawObjectPolicy.code)
  const objectPolicyKind = text(rawObjectPolicy.kind)
  const objectPolicy = type === 'action' && objectPolicyCode && objectPolicyKind
    ? { code: objectPolicyCode, kind: objectPolicyKind }
    : null
  return { code, name, type, grantable, tools, objectPolicy, children }
}

export function normalizeFeishuOrgStatus(value: unknown): FeishuOrgSyncStatus {
  const recordValue = record(value)
  return {
    hasSynced: recordValue.has_synced === true,
    syncedAt: text(recordValue.synced_at),
    departments: Math.max(0, Math.trunc(Number(recordValue.departments)) || 0),
    mappedUsers: Math.max(0, Math.trunc(Number(recordValue.mapped_users)) || 0),
    unmappedUsers: Math.max(0, Math.trunc(Number(recordValue.unmapped_users)) || 0),
  }
}

export function normalizeFeishuOrgStats(value: unknown): FeishuOrgSyncStats {
  const recordValue = record(value)
  return {
    departments: Math.max(0, Math.trunc(Number(recordValue.departments)) || 0),
    contacts: Math.max(0, Math.trunc(Number(recordValue.contacts)) || 0),
    matchedUsers: Math.max(0, Math.trunc(Number(recordValue.matched_users)) || 0),
    unmatchedContacts: Math.max(0, Math.trunc(Number(recordValue.unmatched_contacts)) || 0),
  }
}
