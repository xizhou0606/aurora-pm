export const accountStatuses = ['pending_approval', 'active', 'rejected', 'banned'] as const
export type AccountStatus = (typeof accountStatuses)[number]
export type AccountStatusFilter = AccountStatus | 'all'

export type AccessRole = {
  id: number
  code: string
  name: string
  description: string
  enabled: boolean
  systemManaged: boolean
  permissionCodes: string[]
}

export type AccessGroup = AccessRole & {
  isDefault: boolean
}

export type ManagedUser = {
  id: number
  name: string
  username: string
  email: string
  avatarUrl?: string
  status: AccountStatus
  groupId: number | null
  roleIds: number[]
  group: Pick<AccessGroup, 'id' | 'code' | 'name' | 'systemManaged' | 'isDefault'> | null
  roles: Pick<AccessRole, 'id' | 'code' | 'name'>[]
}

export type ManagedUsersQuery = {
  status?: AccountStatusFilter
  query?: string
  page: number
  pageSize: number
  groupId?: number
  roleId?: number
}

export type ManagedUsersResult = {
  items: ManagedUser[]
  page: number
  pageSize: number
  total: number
  hasMore: boolean
}

export type AccessCapabilities = {
  view: boolean
  approve: boolean
  reject: boolean
  ban: boolean
  unban: boolean
  manageAssignments: boolean
  managePermissions: boolean
  resetPassword: boolean
}

export type PasswordResetLink = {
  url: string
  expiresAt: string
  notificationQueued: boolean
}

export type PasswordResetLinkRequest = {
  userId: number
  notify: boolean
}

export type PermissionNode = {
  code: string
  name: string
  type: 'page' | 'view' | 'module' | 'action'
  grantable: boolean
  tools: string[]
  objectPolicy: { code: string; kind: string } | null
  children: PermissionNode[]
}

export type AccessSubjectKind = 'role' | 'group'
export type AccessSubject = AccessRole | AccessGroup
export type AccessSubjectPolicy = {
  protected: boolean
  canEditMetadata: boolean
  canEditEnabled: boolean
  canDelete: boolean
  canReplacePermissions: boolean
}
export type UserStatusIntent = 'approve' | 'reject' | 'ban' | 'unban'

export type SubjectDraft = {
  kind: AccessSubjectKind
  id?: number
  code: string
  name: string
  description: string
  enabled: boolean
}

export type UserStatusChange = {
  userId: number
  intent: UserStatusIntent
  reason: string
}

export type UserAccessReplacement = {
  userId: number
  groupId: number
  roleIds: number[]
}

export type SubjectPermissionReplacement = {
  kind: AccessSubjectKind
  subjectId: number
  permissionCodes: string[]
}

export type SubjectMembersAdd = {
  kind: AccessSubjectKind
  subjectId: number
  users: ManagedUser[]
}

export type SubjectMemberRemoval =
  | {
      kind: 'role'
      subjectId: number
      user: ManagedUser
    }
  | {
      kind: 'group'
      subjectId: number
      user: ManagedUser
      fallbackGroupId: number
    }

export function isAccountStatus(value: unknown): value is AccountStatus {
  return accountStatuses.includes(value as AccountStatus)
}

export function accessSubjectPolicy(kind: AccessSubjectKind, subject: AccessSubject): AccessSubjectPolicy {
  const systemRole = kind === 'role' && subject.systemManaged
  const defaultGroup = kind === 'group' && 'isDefault' in subject && subject.isDefault
  const protectedGroup = kind === 'group' && (subject.systemManaged || defaultGroup)

  return {
    protected: systemRole || protectedGroup,
    canEditMetadata: !systemRole,
    canEditEnabled: !systemRole && !defaultGroup,
    canDelete: !systemRole && !protectedGroup,
    canReplacePermissions: !systemRole,
  }
}

export function actionCodes(nodes: readonly PermissionNode[]): string[] {
  return nodes.flatMap((node) => node.type === 'action'
    ? (node.grantable ? [node.code] : [])
    : actionCodes(node.children))
}

// 飞书组织架构同步（展示数据，不参与授权）。
export type FeishuOrgSyncStatus = {
  hasSynced: boolean
  syncedAt: string
  departments: number
  mappedUsers: number
  unmappedUsers: number
}

export type FeishuOrgSyncStats = {
  departments: number
  contacts: number
  matchedUsers: number
  unmatchedContacts: number
}

export type FeishuOrgSyncResult = {
  stats: FeishuOrgSyncStats
  status: FeishuOrgSyncStatus
}
