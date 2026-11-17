import { useQuery } from '@tanstack/react-query'

import { scmConfigClient } from '../api'

import { scmConfigKeys } from './keys'
import { useWorkspaceIdValue } from './useWorkspaceId'

// On-demand picker query: only fetched when an admin opens the repository
// picker for one connection, so the provider listing is never speculatively
// called for every connection on the page.
export function useAccessibleReposQuery(connectionId: number, enabled: boolean) {
  const workspaceId = useWorkspaceIdValue()
  return useQuery({
    queryKey: [...scmConfigKeys(workspaceId).connections(), 'accessible-repos', connectionId],
    queryFn: ({ signal }) => scmConfigClient.listAccessibleRepos(connectionId, undefined, signal),
    enabled: enabled && connectionId > 0,
    staleTime: 60_000,
  })
}

// 仓库多选的数据面：跟随 cursor 翻页拉全量，令牌能扫到的仓库一次性展示，
// 页数上限是防御值（正常工作区远到不了），避免误配的超大账号拖死页面。
const SCAN_PAGE_CAP = 20

export function useAllAccessibleReposQuery(connectionId: number, enabled: boolean) {
  const workspaceId = useWorkspaceIdValue()
  return useQuery({
    queryKey: [...scmConfigKeys(workspaceId).connections(), 'accessible-repos-all', connectionId],
    queryFn: async ({ signal }) => {
      const repos = []
      let cursor: string | undefined
      for (let page = 0; page < SCAN_PAGE_CAP; page += 1) {
        const listing = await scmConfigClient.listAccessibleRepos(connectionId, cursor, signal)
        repos.push(...listing.repos)
        if (!listing.has_more || !listing.next_cursor) {
          return { repos, truncated: false }
        }
        cursor = listing.next_cursor
      }
      return { repos, truncated: true }
    },
    enabled: enabled && connectionId > 0,
    staleTime: 60_000,
  })
}
