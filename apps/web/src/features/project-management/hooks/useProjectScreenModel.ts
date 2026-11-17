import { useCallback } from 'react'

import {
  resolveProjectScreenState,
  usePlatformUsersQuery,
  useProjectBoardQuery,
  type ProjectScreenState,
  type ProjectWorkspaceRoute,
} from '../model'
import type { ProjectOperationAccess } from './useOperationAccess'

export function useProjectScreenModel(
  route: ProjectWorkspaceRoute,
  access?: ProjectOperationAccess,
): ProjectScreenState {
  const projectId = route.projectId ?? 0
  const canView = access ? access.status === 'ready' && access.viewBoard : true
  const boardQuery = useProjectBoardQuery(projectId, projectId > 0 && canView)
  const usersQuery = usePlatformUsersQuery(canView)
  const retry = useCallback(() => {
    void boardQuery.refetch()
  }, [boardQuery.refetch])

  return resolveProjectScreenState({
    route,
    status: boardQuery.status,
    board: boardQuery.data,
    error: boardQuery.error,
    fetching: boardQuery.isFetching,
    users: usersQuery.data ?? [],
    retry,
  })
}
