import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'

import {
  canRetainStaleApiData,
  isSafeApiError,
  PM_API_ERROR_CODES,
  type PMScopeOptions,
} from '@/shared/api'
import {
  normalizeQualitySceneOptions,
  normalizeUsers,
  qualityKeys,
} from '../api'
import type {
  QualitySceneOptionCatalogs,
  QualitySceneOptionKind,
  QualitySceneOptionsState,
  QualityUser,
  QualityUsersState,
} from '../model/types'
import type { QualityEnvironment } from '../components/environment'

function isWorkspaceAccessError(error: unknown): boolean {
  return isSafeApiError(error)
    && error.status === 403
    && error.code === PM_API_ERROR_CODES.WORKSPACE_FORBIDDEN
}

export type QualityScopeState =
  | { status: 'loading' }
  | { status: 'unavailable'; reason: 'workspace_access' }
  | { status: 'error'; retry: () => Promise<void> }
  | { status: 'ready'; data: PMScopeOptions }

export function useQualityScopeQuery(environment: QualityEnvironment): QualityScopeState {
  const query = useQuery({
    queryKey: environment.workspaceId > 0
      ? qualityKeys(environment.workspaceId).scope
      : ['pm', 'quality', 'disabled', 'scope'],
    queryFn: ({ signal }) => environment.client.scope(signal),
    enabled: environment.allowed,
    staleTime: 5 * 60_000,
  })
  if (query.isPending) return { status: 'loading' }
  if (query.isError && isWorkspaceAccessError(query.error)) {
    return { status: 'unavailable', reason: 'workspace_access' }
  }
  if (query.isError) return { status: 'error', retry: async () => { await query.refetch() } }
  return { status: 'ready', data: query.data }
}

const USER_DIRECTORY_LIMIT = 500
const USER_HYDRATION_LIMIT = 100

function normalizeSelectedUserIds(selectedIds: readonly number[]): number[] {
  return [...new Set(selectedIds)]
    .filter((id) => Number.isInteger(id) && id > 0)
    .sort((left, right) => left - right)
    .slice(0, USER_HYDRATION_LIMIT)
}

// The directory is loaded once per workspace and filtered locally in the selects
// (same pattern as the requirement-management assignee pickers); typing never
// issues a request. The hydration query only covers ids missing from the
// directory response (inactive users or beyond the first page limit).
export function useQualityUsersQuery(
  environment: QualityEnvironment,
  selectedIds: readonly number[] = [],
): QualityUsersState {
  const workspaceId = environment.workspaceId > 0 ? environment.workspaceId : 1
  const directoryQuery = useQuery({
    queryKey: qualityKeys(workspaceId).userDirectory,
    queryFn: ({ signal }) => environment.client.users({ limit: USER_DIRECTORY_LIMIT }, signal),
    enabled: environment.allowed,
    staleTime: 120_000,
    retry: false,
    retryOnMount: false,
    refetchOnMount: false,
  })
  const normalizedSelectedIds = useMemo(
    () => normalizeSelectedUserIds(selectedIds),
    [selectedIds],
  )
  const directoryItems = useMemo(
    () => (directoryQuery.data ? normalizeUsers(directoryQuery.data) : undefined),
    [directoryQuery.data],
  )
  const missingIds = useMemo(() => {
    if (!directoryItems) return []
    const known = new Set(directoryItems.map((item) => item.id))
    return normalizedSelectedIds.filter((id) => !known.has(id))
  }, [directoryItems, normalizedSelectedIds])
  const hydrationQuery = useQuery({
    queryKey: qualityKeys(workspaceId).selectedDirectoryUsers(missingIds),
    queryFn: ({ signal }) => environment.client.users({ ids: missingIds, limit: USER_HYDRATION_LIMIT }, signal),
    enabled: environment.allowed && missingIds.length > 0,
    staleTime: 120_000,
  })
  const hydrationItems = useMemo(
    () => (hydrationQuery.data ? normalizeUsers(hydrationQuery.data) : []),
    [hydrationQuery.data],
  )
  const error = directoryQuery.error || hydrationQuery.error
  const rows = useMemo(() => {
    if (error && !canRetainStaleApiData(error)) return []
    const byId = new Map<number, QualityUser>()
    for (const item of hydrationItems) byId.set(item.id, item)
    for (const item of directoryItems ?? []) byId.set(item.id, item)
    return [...byId.values()]
  }, [error, hydrationItems, directoryItems])
  if (directoryQuery.isPending || (missingIds.length > 0 && hydrationQuery.isPending)) {
    return { status: 'loading', rows }
  }
  if (directoryQuery.isError || hydrationQuery.isError) {
    return {
      status: 'error',
      rows,
      retry: async () => {
        await Promise.all([
          directoryQuery.refetch(),
          ...(missingIds.length > 0 ? [hydrationQuery.refetch()] : []),
        ])
      },
    }
  }
  return { status: 'ready', rows }
}

export function useQualitySceneOptionsQuery(
  environment: QualityEnvironment,
  kind: QualitySceneOptionKind,
  enabled = true,
): QualitySceneOptionsState {
  const query = useQuery({
    queryKey: environment.workspaceId > 0
      ? qualityKeys(environment.workspaceId).sceneOptions(kind)
      : ['pm', 'quality', 'disabled', 'scene-options', kind],
    queryFn: async ({ signal }) => normalizeQualitySceneOptions(
      await environment.client.sceneOptions(kind, signal),
    ),
    enabled: environment.allowed && enabled,
    staleTime: 5 * 60_000,
  })
  const rows = query.data || []
  if (!enabled) return { status: 'ready', rows }
  if (query.isPending) return { status: 'loading', rows }
  if (query.isError) {
    return { status: 'error', rows, retry: async () => { await query.refetch() } }
  }
  return { status: 'ready', rows }
}

export function useQualitySceneOptionCatalogs(
  environment: QualityEnvironment,
  enabled = true,
): QualitySceneOptionCatalogs {
  const sceneEnvironment = useQualitySceneOptionsQuery(environment, 'environment', enabled)
  const operatingSystem = useQualitySceneOptionsQuery(environment, 'operating_system', enabled)
  const browser = useQualitySceneOptionsQuery(environment, 'browser', enabled)
  return { environment: sceneEnvironment, operating_system: operatingSystem, browser }
}

export function useQualityEnvironmentsQuery(
  environment: QualityEnvironment,
): QualitySceneOptionsState {
  return useQualitySceneOptionsQuery(environment, 'environment')
}
