import type { AccountStatus } from './types'

export type ManagedWorkspace = {
  id: number
  name: string
  slug: string
  description: string
  enabled: boolean
  isDefault: boolean
  memberCount: number
}

export type WorkspaceDraft = {
  id?: number
  name: string
  slug: string
  description: string
  enabled: boolean
}

export type WorkspaceMemberRole = 'owner' | 'member'

export type ManagedWorkspaceMember = {
  userId: number
  username: string
  displayName: string
  email: string
  avatarUrl?: string
  status: AccountStatus
  role: WorkspaceMemberRole
  enabled: boolean
}

export type ManagedWorkspaceMembersQuery = {
  query: string
  page: number
  pageSize: number
}

export type ManagedWorkspaceMembersResult = {
  items: ManagedWorkspaceMember[]
  page: number
  pageSize: number
  total: number
  hasMore: boolean
}

export type WorkspaceMemberDraft = {
  workspaceId: number
  userId: number
  role: WorkspaceMemberRole
  enabled: boolean
}

export function workspaceRole(value: unknown): WorkspaceMemberRole | null {
  return value === 'owner' || value === 'member' ? value : null
}
