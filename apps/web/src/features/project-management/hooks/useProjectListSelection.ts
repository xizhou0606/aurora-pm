import { useCallback, useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'

import type { ProjectListState, ProjectWorkspaceRoute } from '../model'

export function useProjectListSelection(
  route: ProjectWorkspaceRoute,
  state: ProjectListState,
) {
  const navigate = useNavigate()
  const firstProjectId = state.status === 'ready' ? state.items[0]?.id : undefined
  const select = useCallback((projectId: number) => {
    void navigate({
      to: '/workspace/projects',
      search: { project_id: projectId },
    })
  }, [navigate])

  useEffect(() => {
    if (route.kind !== 'list' || route.projectId || !firstProjectId) return
    void navigate({
      to: '/workspace/projects',
      search: { project_id: firstProjectId },
      replace: true,
    })
  }, [firstProjectId, navigate, route.kind, route.projectId])

  return select
}
