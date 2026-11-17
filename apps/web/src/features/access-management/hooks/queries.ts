import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import { searchDirectoryPage } from '@/shared/query/search-directory-page'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import {
  fetchFeishuOrgStatus,
  listAccessGroups,
  listAccessRoles,
  listAllManagedUsers,
  listManagedUsers,
  listPermissionTree,
  listSubjectPermissions,
} from '../api'
import type { AccessSubjectKind, ManagedUsersQuery } from '../model'
import { accessManagementKeys } from './keys'

export function useManagedUsers(
  query: ManagedUsersQuery,
  enabled = true,
  hydrateAccess = true,
) {
  return useQuery({
    queryKey: accessManagementKeys.users(query, hydrateAccess),
    queryFn: () => listManagedUsers(query, hydrateAccess),
    enabled,
  })
}

export function useAllManagedUsers(enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.allUsers(),
    queryFn: listAllManagedUsers,
    enabled,
  })
}

export function useSubjectMembers(
  kind: AccessSubjectKind,
  subjectId: number | null,
  query: string,
  enabled = true,
) {
  return useInfiniteQuery({
    queryKey: accessManagementKeys.subjectMembers(kind, subjectId ?? 0, query),
    queryFn: ({ pageParam }) => listManagedUsers({
      query,
      page: pageParam,
      pageSize: 20,
      ...(kind === 'group' ? { groupId: subjectId ?? undefined } : { roleId: subjectId ?? undefined }),
    }, false),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => lastPage.hasMore ? lastPage.page + 1 : undefined,
    enabled: Boolean(subjectId) && enabled,
  })
}

export function useInfiniteManagedUsers(query: string, enabled = true, search: SearchOptions = USER_SEARCH) {
  return useInfiniteQuery({
    queryKey: [...accessManagementKeys.userDirectory(query), search],
    queryFn: async ({ pageParam, signal }) => {
      const result = await searchDirectoryPage({
        cursor: pageParam, query, search, pageSize: 20, signal,
        load: (page, query) => listManagedUsers({ query, page, pageSize: 20 }, false, signal),
        next: page => page.hasMore ? page.page + 1 : undefined,
        fields: user => [user.name, user.username, user.email],
      })
      return { ...result.source, items: result.items, hasMore: result.nextCursor !== undefined, nextCursor: result.nextCursor }
    },
    initialPageParam: 1,
    getNextPageParam: lastPage => lastPage.nextCursor,
    enabled,
  })
}

export function useAccessRoles(enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.roles(),
    queryFn: listAccessRoles,
    enabled,
  })
}

export function useAccessGroups(enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.groups(),
    queryFn: listAccessGroups,
    enabled,
  })
}

export function useFeishuOrgStatus(enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.feishuOrg(),
    queryFn: fetchFeishuOrgStatus,
    enabled,
  })
}

export function usePermissionTree(enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.permissionTree(),
    queryFn: listPermissionTree,
    enabled,
  })
}

export function useSubjectPermissions(kind: AccessSubjectKind, subjectId: number | null, enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.subjectPermissions(kind, subjectId ?? 0),
    queryFn: () => listSubjectPermissions(kind, subjectId ?? 0),
    enabled: Boolean(subjectId) && enabled,
  })
}
