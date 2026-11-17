import { coreApi, isSafeApiError, toSafeApiError, unwrapApiData } from '@/shared/api'
import type {
  AccessGroup,
  AccessRole,
  FeishuOrgSyncResult,
  FeishuOrgSyncStatus,
  ManagedUser,
  ManagedUsersQuery,
  ManagedUsersResult,
  PermissionNode,
  SubjectPermissionReplacement,
  SubjectDraft,
  SubjectMemberRemoval,
  UserAccessReplacement,
  UserStatusChange,
  PasswordResetLink,
  PasswordResetLinkRequest,
} from '../model'
import {
  normalizeFeishuOrgStats,
  normalizeFeishuOrgStatus,
  normalizeGroup,
  normalizePermissionNode,
  normalizeRole,
  normalizeUser,
} from '../api/normalizers'
import { legacyFilteredItems, listPayload, positiveNumber, rows } from './client-utils'

const ACCESS_ADMIN_PATH = '/access/admin'

export async function listManagedUsers(input: ManagedUsersQuery, hydrateAccess = true, signal?: AbortSignal): Promise<ManagedUsersResult> {
  try {
    const [response, roles, groups] = await Promise.all([
      coreApi.get(`${ACCESS_ADMIN_PATH}/users`, {
        signal,
        params: {
          ...(input.status && input.status !== 'all' ? { status: input.status } : {}),
          ...(input.query?.trim() ? { q: input.query.trim() } : {}),
          ...(input.groupId ? { group_id: input.groupId } : {}),
          ...(input.roleId ? { role_id: input.roleId } : {}),
          page: input.page,
          page_size: input.pageSize,
        },
      }),
      hydrateAccess ? listAccessRoles() : Promise.resolve([]),
      hydrateAccess ? listAccessGroups() : Promise.resolve([]),
    ])
    const payload = listPayload(response)
    if (typeof payload.total !== 'number') {
      const filtered = legacyFilteredItems(payload.items, input)
      const offset = (input.page - 1) * input.pageSize
      return {
        items: rows(filtered.slice(offset, offset + input.pageSize), (entry) => normalizeUser(entry, groups, roles)),
        page: input.page,
        pageSize: input.pageSize,
        total: filtered.length,
        hasMore: offset + input.pageSize < filtered.length,
      }
    }
    return {
      items: rows(payload.items, (entry) => normalizeUser(entry, groups, roles)),
      page: positiveNumber(payload.page, input.page),
      pageSize: positiveNumber(payload.page_size, input.pageSize),
      total: positiveNumber(payload.total, 0),
      hasMore: payload.has_more === true,
    }
  } catch (error) {
    if (isSafeApiError(error)) throw error
    throw toSafeApiError(error, 'ACCESS_USERS_LOAD_FAILED')
  }
}

export async function listAllManagedUsers(): Promise<ManagedUser[]> {
  const users = new Map<number, ManagedUser>()
  let page = 1

  while (true) {
    const result = await listManagedUsers({ page, pageSize: 100 }, false)
    const previousSize = users.size
    result.items.forEach((user) => users.set(user.id, user))

    if (!result.hasMore || result.items.length === 0 || users.size === previousSize) break
    if (result.total > 0 && users.size >= result.total) break
    page = Math.max(page + 1, result.page + 1)
  }

  return [...users.values()]
}

export async function addUsersToSubject(
  kind: 'role' | 'group',
  subjectId: number,
  users: ManagedUser[],
): Promise<void> {
  try {
    await Promise.all(users.map((user) => kind === 'group'
      ? replaceUserGroup(user.id, subjectId)
      : replaceUserRoles(user.id, [...new Set([...user.roleIds, subjectId])])))
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_MEMBERS_SAVE_FAILED')
  }
}

export async function removeUserFromSubject(input: SubjectMemberRemoval): Promise<void> {
  try {
    if (input.kind === 'role') {
      await replaceUserRoles(
        input.user.id,
        input.user.roleIds.filter((roleId) => roleId !== input.subjectId),
      )
      return
    }
    if (input.fallbackGroupId === input.subjectId) {
      throw new Error('fallback_group_must_differ_from_subject')
    }
    await replaceUserGroup(input.user.id, input.fallbackGroupId)
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_MEMBER_REMOVE_FAILED')
  }
}

export async function replaceUserGroup(userId: number, groupId: number): Promise<void> {
  try {
    await coreApi.put(`${ACCESS_ADMIN_PATH}/users/${userId}/group`, { group_id: groupId })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_USER_GROUP_SAVE_FAILED')
  }
}

export async function replaceUserRoles(userId: number, roleIds: number[]): Promise<void> {
  try {
    await coreApi.put(`${ACCESS_ADMIN_PATH}/users/${userId}/roles`, { role_ids: roleIds })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_USER_ROLES_SAVE_FAILED')
  }
}

export async function listAccessRoles(): Promise<AccessRole[]> {
  try {
    const payload = listPayload(await coreApi.get(`${ACCESS_ADMIN_PATH}/roles`))
    return rows(payload.items, normalizeRole)
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_ROLES_LOAD_FAILED')
  }
}

export async function listAccessGroups(): Promise<AccessGroup[]> {
  try {
    const payload = listPayload(await coreApi.get(`${ACCESS_ADMIN_PATH}/groups`))
    return rows(payload.items, normalizeGroup)
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_GROUPS_LOAD_FAILED')
  }
}

export async function listPermissionTree(): Promise<PermissionNode[]> {
  try {
    const payload = listPayload(await coreApi.get(`${ACCESS_ADMIN_PATH}/permission-tree`))
    return rows(payload.items, normalizePermissionNode)
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_PERMISSION_TREE_LOAD_FAILED')
  }
}

export async function listSubjectPermissions(kind: 'role' | 'group', subjectId: number): Promise<string[]> {
  const collection = kind === 'role' ? 'roles' : 'groups'
  try {
    const response = await coreApi.get(`${ACCESS_ADMIN_PATH}/${collection}/${subjectId}/action-permissions`)
    const payload = unwrapApiData<{ action_codes?: unknown }>(response)
    return Array.isArray(payload?.action_codes)
      ? [...new Set(payload.action_codes.filter((code): code is string => typeof code === 'string' && Boolean(code.trim())))]
      : []
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_PERMISSIONS_LOAD_FAILED')
  }
}

export async function createAccessSubject(input: SubjectDraft): Promise<void> {
  const collection = input.kind === 'role' ? 'roles' : 'groups'
  try {
    await coreApi.post(`${ACCESS_ADMIN_PATH}/${collection}`, {
      ...(input.kind === 'group' ? { code: input.code } : {}),
      name: input.name,
      description: input.description,
      enabled: input.enabled,
    })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_CREATE_FAILED')
  }
}

export async function updateAccessSubject(input: SubjectDraft & { id: number }): Promise<void> {
  const collection = input.kind === 'role' ? 'roles' : 'groups'
  try {
    await coreApi.patch(`${ACCESS_ADMIN_PATH}/${collection}/${input.id}`, {
      name: input.name,
      description: input.description,
      enabled: input.enabled,
    })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_UPDATE_FAILED')
  }
}

export async function deleteAccessSubject(kind: 'role' | 'group', subjectId: number): Promise<void> {
  const collection = kind === 'role' ? 'roles' : 'groups'
  try {
    await coreApi.delete(`${ACCESS_ADMIN_PATH}/${collection}/${subjectId}`)
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_DELETE_FAILED')
  }
}

export async function changeUserStatus(input: UserStatusChange): Promise<void> {
  try {
    await coreApi.post(`${ACCESS_ADMIN_PATH}/users/${input.userId}/${input.intent}`, {
      reason: input.reason,
    })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_USER_STATUS_SAVE_FAILED')
  }
}

export async function replaceUserAccess(input: UserAccessReplacement): Promise<void> {
  try {
    await coreApi.put(`${ACCESS_ADMIN_PATH}/users/${input.userId}/access`, {
      group_id: input.groupId,
      role_ids: input.roleIds,
    })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_USER_ASSIGNMENTS_SAVE_FAILED')
  }
}

export async function createPasswordResetLink(
  input: PasswordResetLinkRequest,
): Promise<PasswordResetLink> {
  try {
    const response = await coreApi.post(`${ACCESS_ADMIN_PATH}/users/${input.userId}/password-reset-links`, {
      notify: input.notify,
    })
    const payload = unwrapApiData<Record<string, unknown>>(response)
    const url = String(payload?.url || '').trim()
    const expiresAt = String(payload?.expires_at || '').trim()
    if (!url || !expiresAt) throw new Error('invalid_password_reset_link')
    return {
      url,
      expiresAt,
      notificationQueued: payload?.notification_queued === true,
    }
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_PASSWORD_RESET_LINK_FAILED')
  }
}

export async function replaceSubjectPermissions(input: SubjectPermissionReplacement): Promise<void> {
  const collection = input.kind === 'role' ? 'roles' : 'groups'
  try {
    await coreApi.put(`${ACCESS_ADMIN_PATH}/${collection}/${input.subjectId}/action-permissions`, {
      action_codes: input.permissionCodes,
    })
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_SUBJECT_PERMISSIONS_SAVE_FAILED')
  }
}

export async function fetchFeishuOrgStatus(): Promise<FeishuOrgSyncStatus> {
  try {
    const payload = listPayload(await coreApi.get(`${ACCESS_ADMIN_PATH}/groups/feishu-org/status`))
    return normalizeFeishuOrgStatus(payload)
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_FEISHU_ORG_STATUS_FAILED')
  }
}

export async function syncFeishuOrg(): Promise<FeishuOrgSyncResult> {
  try {
    const payload = listPayload(await coreApi.post(`${ACCESS_ADMIN_PATH}/groups/feishu-org/sync`))
    const body = payload && typeof payload === 'object' ? payload as Record<string, unknown> : {}
    return {
      stats: normalizeFeishuOrgStats(body.stats),
      status: normalizeFeishuOrgStatus(body.status),
    }
  } catch (error) {
    throw toSafeApiError(error, 'ACCESS_FEISHU_ORG_SYNC_FAILED')
  }
}
