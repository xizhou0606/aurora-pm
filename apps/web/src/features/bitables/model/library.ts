export type ResourceMenuAction = 'open' | 'share' | 'permissions' | 'copyLink' | 'duplicate' | 'shortcuts' | 'pin' | 'favorite' | 'owner' | 'location' | 'name' | 'delete'
export type ResourceDirectoryItem = { id: string; targetId: string; name: string; shortcut: boolean; parentFolderId?: number; pinned: boolean; favorited: boolean; actions: readonly ResourceMenuAction[] }
export type ResourceDialog =
  | { kind: 'closed' }
  | { kind: 'share'; id: string; resourceId?: string; purpose?: 'share' | 'permissions' }
  | { kind: 'name' | 'duplicate' | 'shortcuts' | 'location' | 'owner' | 'delete'; id: string; name: string; shortcut: boolean; parentFolderId?: number; ownerUserId?: number }
