import {
  normalizePlatformUsers,
  type PlatformUser,
  type PlatformUserPage,
  type PlatformUserQuery,
} from '@/shared/api'
import type { WorkItemsClient } from './client'
import { array, optionalNumber, optionalString, record } from './contracts'
import type { WorkItemUserOption } from '../model/types'

function rows(value: unknown): unknown[] {
  if (Array.isArray(value)) return value
  const source = record(value)
  if (Array.isArray(source.items)) return source.items
  if (Array.isArray(source.data)) return source.data
  return array(record(source.data).items)
}

function normalizeUsers(value: unknown): WorkItemUserOption[] {
  return rows(value).map((entry) => {
    const source = record(entry)
    const id = optionalNumber(source.id)
    const label = optionalString(source.display_name)
      || optionalString(source.name)
      || optionalString(source.username)
      || optionalString(source.email)
    if (!id || !label) throw new Error('invalid_work_item_user_directory')
    return {
      id,
      label,
      username: optionalString(source.username),
      avatarUrl: optionalString(source.avatar_url),
    }
  })
}

export function platformUsersToWorkItemUsers(
  users: readonly PlatformUser[],
): WorkItemUserOption[] {
  return normalizeUsers(users)
}

export async function requestPlatformUsersForWorkItems(
  client: WorkItemsClient,
  signal?: AbortSignal,
): Promise<PlatformUser[]> {
  const raw = await client.requestPMUsers<unknown>(signal)
  const users = normalizePlatformUsers(raw)
  if (!Array.isArray(raw) || users.length !== raw.length) {
    throw new Error('invalid_work_item_user_directory')
  }
  return users
}

export async function searchPlatformUsersForWorkItems(
  client: WorkItemsClient,
  query: PlatformUserQuery,
  signal?: AbortSignal,
): Promise<PlatformUserPage> {
  if (client.requestPMUserPage) return client.requestPMUserPage(query, signal)
  const items = await requestPlatformUsersForWorkItems(client, signal)
  return { items, total: items.length }
}

export async function requestPMWorkItemUsers(
  client: WorkItemsClient,
  signal?: AbortSignal,
): Promise<WorkItemUserOption[]> {
  return platformUsersToWorkItemUsers(
    await requestPlatformUsersForWorkItems(client, signal),
  )
}
