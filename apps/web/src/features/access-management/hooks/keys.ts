import { pmKeys } from '@/shared/query'
import type { AccessSubjectKind, ManagedUsersQuery } from '../model'

const root = [...pmKeys.session(), 'access-management'] as const

export const accessManagementKeys = {
  root,
  users: (query?: ManagedUsersQuery, hydrateAccess = true) => [
    ...root,
    'users',
    query ?? {},
    { hydrateAccess },
  ] as const,
  allUsers: () => [...root, 'users', 'all'] as const,
  subjectMembers: (kind: AccessSubjectKind, subjectId: number, query: string) => [
    ...root,
    'subject-members',
    kind,
    subjectId,
    { query },
  ] as const,
  userDirectory: (query: string) => [...root, 'user-directory', { query }] as const,
  roles: () => [...root, 'roles'] as const,
  groups: () => [...root, 'groups'] as const,
  feishuOrg: () => [...root, 'feishu-org'] as const,
  permissionTree: () => [...root, 'permission-tree'] as const,
  subjectPermissions: (kind: AccessSubjectKind, subjectId: number) => [
    ...root,
    kind === 'role' ? 'roles' : 'groups',
    subjectId,
    'action-permissions',
  ] as const,
  subjects: (kind: AccessSubjectKind) => kind === 'role'
    ? accessManagementKeys.roles()
    : accessManagementKeys.groups(),
  workspaces: () => [...root, 'workspaces'] as const,
  workspaceMembers: (workspaceId: number) => [...root, 'workspaces', workspaceId, 'members'] as const,
  allWorkspaceMembers: (workspaceId: number) => [
    ...accessManagementKeys.workspaceMembers(workspaceId),
    'all',
  ] as const,
  workspaceMemberPage: (workspaceId: number, query: { query: string; page: number; pageSize: number }) => [
    ...accessManagementKeys.workspaceMembers(workspaceId),
    query,
  ] as const,
}
