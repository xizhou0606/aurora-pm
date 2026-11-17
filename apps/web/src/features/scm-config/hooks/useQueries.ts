import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'

import { searchPlatformUsers } from '@/shared/api'
import type { UserSelectOption } from '@/shared/ui'
import { useWorkspaceStore } from '@/shared/state'

import { scmConfigClient } from '../api'

import { scmConfigKeys } from './keys'

// Query keys require a positive workspace ID; disabled queries get a stable
// placeholder key that never reads the cache (reporting 同款守卫). Query
// enablement follows the same workspace guard as the key itself.
function useScmQuery(enabled: boolean, ...scope: readonly unknown[]) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const ready = enabled && workspaceId > 0
  const key = ready
    ? [...scmConfigKeys(workspaceId).root, ...scope]
    : (['pm', 'scm-config', 'disabled', ...scope] as const)
  return { ready, key }
}

export function useConnectionsQuery(enabled: boolean) {
  const { ready, key } = useScmQuery(enabled, 'connections')
  return useQuery({
    queryKey: key,
    queryFn: ({ signal }) => scmConfigClient.listConnections(signal),
    enabled: ready,
  })
}

export function useRepositoriesQuery(enabled: boolean) {
  const { ready, key } = useScmQuery(enabled, 'repositories')
  return useQuery({
    queryKey: key,
    queryFn: ({ signal }) => scmConfigClient.listRepositories(signal),
    enabled: ready,
  })
}

export function useBindingsQuery(status: string | undefined, enabled: boolean) {
  const { ready, key } = useScmQuery(enabled, 'bindings', status ?? 'all')
  return useQuery({
    queryKey: key,
    queryFn: ({ signal }) => scmConfigClient.listBindings(status || undefined, signal),
    enabled: ready,
  })
}

export function useSettingsQuery(enabled: boolean) {
  const { ready, key } = useScmQuery(enabled, 'settings')
  return useQuery({
    queryKey: key,
    queryFn: ({ signal }) => scmConfigClient.getSettings(signal),
    enabled: ready,
  })
}

// 作者绑定的人员选项：一次性拉取工作区成员目录，前端本地过滤（质量模块挑人
// 同款），目录即在职成员；确认绑定时后端仍会校验成员身份并失败关闭，所以
// 不为目录外的历史/停用账号做补水。
const MEMBER_DIRECTORY_LIMIT = 500

function toUserOption(user: { id: number; username: string; display_name?: string; avatar_url?: string }): UserSelectOption {
  return {
    id: user.id,
    name: user.display_name?.trim() || user.username,
    username: user.username,
    ...(user.avatar_url?.trim() ? { avatarUrl: user.avatar_url.trim() } : {}),
  }
}

export function useMemberOptionsQuery(enabled: boolean) {
  const { ready, key } = useScmQuery(enabled, 'members')
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => searchPlatformUsers({ limit: MEMBER_DIRECTORY_LIMIT }, signal),
    enabled: ready,
    staleTime: 120_000,
    retry: false,
  })
  const options = useMemo(
    () => (query.data ? query.data.items.map(toUserOption) : []),
    [query.data],
  )
  return { options, pending: query.isPending, error: query.error, retry: () => void query.refetch() }
}
