import { copy } from '@/shared/copy'
import type { PlatformUser } from '../api'

export type OwnerDirectoryItem = {
  id: number
  label: string
  username?: string
  avatarUrl?: string
}

export type OwnerDirectoryState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty' }
  | {
      status: 'ready'
      items: readonly OwnerDirectoryItem[]
      onQueryChange?: (query: string) => void
      onLoadMore?: () => void
      hasMore?: boolean
      loadingMore?: boolean
    }

export type TeamDirectoryItem = {
  id: number
  label: string
}

export type TeamDirectoryState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty'; reason?: 'directory' }
  | { status: 'ready'; items: readonly TeamDirectoryItem[] }

export function ownerDirectoryItems(
  users: readonly PlatformUser[],
): OwnerDirectoryItem[] {
  const items = new Map<number, OwnerDirectoryItem>()
  for (const user of users) {
    const id = Number(user.id)
    if (!Number.isInteger(id) || id <= 0 || items.has(id)) continue
    const username = user.username?.trim() || undefined
    const avatarUrl = user.avatar_url?.trim() || undefined
    items.set(id, {
      id,
      label: user.display_name?.trim() ||
        username ||
        copy.projectManagement.write.owner.fallback(id),
      ...(username ? { username } : {}),
      ...(avatarUrl ? { avatarUrl } : {}),
    })
  }
  return [...items.values()]
}
