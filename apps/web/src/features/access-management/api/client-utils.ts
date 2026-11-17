import { unwrapApiData } from '@/shared/api'
import type { ManagedUsersQuery } from '../model'

type ListPayload = {
  items?: unknown
  capabilities?: unknown
  page?: unknown
  page_size?: unknown
  total?: unknown
  has_more?: unknown
}

export function listPayload(value: unknown): ListPayload {
  const payload = unwrapApiData<unknown>(value)
  return payload && typeof payload === 'object' ? payload as ListPayload : {}
}

export function rows<T>(value: unknown, normalize: (entry: unknown) => T | null): T[] {
  return Array.isArray(value)
    ? value.map(normalize).filter((entry): entry is T => Boolean(entry))
    : []
}

export function positiveNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : fallback
}

export function legacyFilteredItems(value: unknown, input: ManagedUsersQuery): unknown[] {
  if (!Array.isArray(value)) return []
  const query = input.query?.trim().toLocaleLowerCase() ?? ''
  return value.filter((entry) => {
    if (!entry || typeof entry !== 'object') return false
    const row = entry as Record<string, unknown>
    const statusMatches = !input.status || input.status === 'all' || row.status === input.status
    const queryMatches = !query || [row.username, row.email].some((field) => (
      typeof field === 'string' && field.toLocaleLowerCase().includes(query)
    ))
    const groupMatches = !input.groupId || Number(row.group_id) === input.groupId
    const roleMatches = !input.roleId || (
      Array.isArray(row.role_ids) && row.role_ids.some((roleID) => Number(roleID) === input.roleId)
    )
    return statusMatches && queryMatches && groupMatches && roleMatches
  })
}