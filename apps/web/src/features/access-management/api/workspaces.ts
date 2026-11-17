import { coreApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import type {
  ManagedWorkspace,
  ManagedWorkspaceMember,
  ManagedWorkspaceMembersQuery,
  ManagedWorkspaceMembersResult,
  WorkspaceDraft,
  WorkspaceMemberDraft,
} from '../model'
import { isAccountStatus, workspaceRole } from '../model'
import { listPayload, positiveNumber, rows } from './client-utils'

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {}
}

function id(value: unknown): number | null {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function normalizeWorkspace(value: unknown): ManagedWorkspace | null {
  const source = record(value)
  const workspaceId = id(source.id)
  const name = text(source.name)
  const slug = text(source.slug)
  if (!workspaceId || !name || !slug) return null
  return {
    id: workspaceId,
    name,
    slug,
    description: text(source.description),
    enabled: source.enabled === true,
    isDefault: source.is_default === true,
    memberCount: Math.max(0, Number(source.member_count) || 0),
  }
}

function normalizeWorkspaceMember(value: unknown): ManagedWorkspaceMember | null {
  const source = record(value)
  const userId = id(source.user_id)
  const role = workspaceRole(source.role)
  if (!userId || !role || !isAccountStatus(source.status)) return null
  return {
    userId,
    username: text(source.username),
    displayName: text(source.display_name),
    email: text(source.email),
    ...(text(source.avatar_url) ? { avatarUrl: text(source.avatar_url) } : {}),
    status: source.status,
    role,
    enabled: source.enabled === true,
  }
}

export async function listManagedWorkspaces(): Promise<ManagedWorkspace[]> {
  try {
    const payload = listPayload(await coreApi.get('/workspaces', { params: { include_disabled: true } }))
    return rows(payload.items, normalizeWorkspace)
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACES_LOAD_FAILED')
  }
}

export async function saveManagedWorkspace(input: WorkspaceDraft): Promise<ManagedWorkspace> {
  try {
    const response = input.id
      ? await coreApi.patch(`/workspaces/${input.id}`, {
          name: input.name,
          description: input.description,
          enabled: input.enabled,
        })
      : await coreApi.post('/workspaces', {
          name: input.name,
          slug: input.slug,
          description: input.description,
        })
    const workspace = normalizeWorkspace(unwrapApiData(response))
    if (!workspace) throw new Error('invalid_workspace_response')
    return workspace
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_SAVE_FAILED')
  }
}

export async function deleteManagedWorkspace(workspaceId: number): Promise<void> {
  try {
    await coreApi.delete(`/workspaces/${workspaceId}`)
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_DELETE_FAILED')
  }
}

export async function setManagedWorkspaceDefault(workspaceId: number): Promise<ManagedWorkspace> {
  try {
    const response = await coreApi.patch(`/workspaces/${workspaceId}`, { is_default: true })
    const workspace = normalizeWorkspace(unwrapApiData(response))
    if (!workspace) throw new Error('invalid_workspace_response')
    return workspace
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_SET_DEFAULT_FAILED')
  }
}

export async function listManagedWorkspaceMembers(
  workspaceId: number,
  input: ManagedWorkspaceMembersQuery,
): Promise<ManagedWorkspaceMembersResult> {
  try {
    const payload = listPayload(await coreApi.get(`/workspaces/${workspaceId}/members`, {
      params: {
        include_disabled: true,
        ...(input.query.trim() ? { q: input.query.trim() } : {}),
        page: input.page,
        page_size: input.pageSize,
      },
    }))
    const normalized = rows(payload.items, normalizeWorkspaceMember)
    if (typeof payload.total !== 'number') {
      const query = input.query.trim().toLocaleLowerCase()
      const filtered = query ? normalized.filter((member) => (
        [member.displayName, member.username, member.email].some((value) => (
          value.toLocaleLowerCase().includes(query)
        ))
      )) : normalized
      const offset = (input.page - 1) * input.pageSize
      return {
        items: filtered.slice(offset, offset + input.pageSize),
        page: input.page,
        pageSize: input.pageSize,
        total: filtered.length,
        hasMore: offset + input.pageSize < filtered.length,
      }
    }
    return {
      items: normalized,
      page: positiveNumber(payload.page, input.page),
      pageSize: positiveNumber(payload.page_size, input.pageSize),
      total: positiveNumber(payload.total, 0),
      hasMore: payload.has_more === true,
    }
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_MEMBERS_LOAD_FAILED')
  }
}

export async function listAllManagedWorkspaceMembers(workspaceId: number): Promise<ManagedWorkspaceMember[]> {
  const members = new Map<number, ManagedWorkspaceMember>()
  let page = 1

  while (true) {
    const result = await listManagedWorkspaceMembers(workspaceId, { query: '', page, pageSize: 100 })
    const previousSize = members.size
    result.items.forEach((member) => members.set(member.userId, member))

    if (!result.hasMore || result.items.length === 0 || members.size === previousSize) break
    if (result.total > 0 && members.size >= result.total) break
    page = Math.max(page + 1, result.page + 1)
  }

  return [...members.values()]
}

export async function saveManagedWorkspaceMember(input: WorkspaceMemberDraft, editing: boolean): Promise<void> {
  try {
    if (editing) {
      await coreApi.patch(`/workspaces/${input.workspaceId}/members/${input.userId}`, {
        role: input.role,
        enabled: input.enabled,
      })
      return
    }
    await coreApi.post(`/workspaces/${input.workspaceId}/members`, {
      user_id: input.userId,
      role: input.role,
      enabled: input.enabled,
    })
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_MEMBER_SAVE_FAILED')
  }
}

export async function saveManagedWorkspaceMembers(inputs: readonly WorkspaceMemberDraft[]): Promise<void> {
  await Promise.all(inputs.map((input) => saveManagedWorkspaceMember(input, false)))
}

export async function deleteManagedWorkspaceMember(workspaceId: number, userId: number): Promise<void> {
  try {
    await coreApi.delete(`/workspaces/${workspaceId}/members/${userId}`)
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_MEMBER_DELETE_FAILED')
  }
}
