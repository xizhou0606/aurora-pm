import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { HOST_NAVIGATION_CATALOG } from '@/app/shell/menu-catalog'
import {
  platformApi,
  toSafeApiError,
  unwrapApiData,
} from '@/shared/api'

export type HostNavigationItem = {
  key: string
  label: string
  path: string
  icon: string
  visible: boolean
  sort_order: number
}

export type HostNavigationConfig = {
  version: number
  host_key: 'one2all'
  items: HostNavigationItem[]
  can_manage: boolean
}

export type HostNavigationUpdate = {
  items: Array<Pick<HostNavigationItem, 'key' | 'visible' | 'sort_order'>>
}

const hostNavigationKey = (workspaceId: number) => [
  'host-navigation',
  workspaceId,
] as const

export function hydrateHostNavigation(config: HostNavigationConfig): HostNavigationConfig {
  const overrides = new Map(config.items.map((item) => [item.key, item]))
  const hasCompleteOverrides = HOST_NAVIGATION_CATALOG.every((entry) => overrides.has(entry.hostKey))
  const items = HOST_NAVIGATION_CATALOG.map((entry) => {
    const override = overrides.get(entry.hostKey)
    return {
      key: entry.hostKey,
      label: entry.label,
      path: entry.path,
      icon: entry.hostIcon,
      visible: override?.visible ?? true,
      sort_order: hasCompleteOverrides
        ? (override?.sort_order ?? entry.sortOrder)
        : entry.sortOrder,
    }
  })
  if (hasCompleteOverrides) {
    items.sort((left, right) => {
      if (left.sort_order === right.sort_order) return left.key.localeCompare(right.key)
      return left.sort_order - right.sort_order
    })
  }
  return { ...config, items }
}

export async function getHostNavigation(
  signal?: AbortSignal,
): Promise<HostNavigationConfig> {
  try {
    const response = await platformApi.get('/host-navigation/one2all', { signal })
    const result = unwrapApiData<HostNavigationConfig>(response)
    if (!result || !Array.isArray(result.items)) {
      throw new Error('invalid host navigation response')
    }
    return hydrateHostNavigation(result)
  } catch (error) {
    throw toSafeApiError(error, 'HOST_NAVIGATION_LOAD_FAILED')
  }
}

export async function putHostNavigation(
  body: HostNavigationUpdate,
): Promise<HostNavigationConfig> {
  try {
    const response = await platformApi.put('/host-navigation/one2all', body)
    const result = unwrapApiData<HostNavigationConfig>(response)
    if (!result || !Array.isArray(result.items)) {
      throw new Error('invalid host navigation response')
    }
    return hydrateHostNavigation(result)
  } catch (error) {
    throw toSafeApiError(error, 'HOST_NAVIGATION_SAVE_FAILED')
  }
}

export function useHostNavigationQuery(workspaceId: number) {
  return useQuery({
    queryKey: hostNavigationKey(workspaceId),
    queryFn: ({ signal }) => getHostNavigation(signal),
    enabled: workspaceId > 0,
  })
}

export function useHostNavigationMutation(workspaceId: number) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: putHostNavigation,
    onSuccess: (data) => {
      queryClient.setQueryData(hostNavigationKey(workspaceId), data)
    },
  })
}
