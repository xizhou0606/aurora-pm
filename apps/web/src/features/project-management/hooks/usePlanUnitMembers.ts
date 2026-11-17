import { useCallback, useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { useWorkspaceStore } from '@/shared/state'
import type { ProjectMember, ScrumVersionTaskNew } from '../api'
import {
  planUnitMembersQueryKey,
  projectBoardQueryOptions,
  usePlanUnitMembersQueries,
} from '../model'

/**
 * Plan-unit member rows (owner + participants union) for a list of plan
 * units, loaded per unit only while the participants column is visible.
 *
 * Owns the query-cache side effects of member management for list surfaces:
 * `refreshAfterMembersSaved` invalidates the member queries and the project
 * board so owner/participant cells reflect a dialog save immediately.
 */
export function usePlanUnitMembers(input: {
  projectId: number
  rows: ScrumVersionTaskNew[]
  enabled: boolean
}) {
  const { projectId, rows, enabled } = input
  const queryClient = useQueryClient()
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const rowIds = useMemo(() => rows.map((row) => row.id), [rows])
  const membersQueries = usePlanUnitMembersQueries(rowIds, enabled)

  const membersByUnit = useMemo(() => {
    const map = new Map<number, ProjectMember[]>()
    rowIds.forEach((id, index) => {
      const page = membersQueries[index]?.data
      if (page) map.set(id, page.items)
    })
    return map
  }, [membersQueries, rowIds])

  const membersPendingUnits = useMemo(() => {
    const pending = new Set<number>()
    rowIds.forEach((id, index) => {
      if (membersQueries[index]?.isLoading) pending.add(id)
    })
    return pending
  }, [membersQueries, rowIds])

  const refreshAfterMembersSaved = useCallback(() => {
    if (!Number.isInteger(workspaceId) || workspaceId <= 0) return
    void queryClient.invalidateQueries({ queryKey: planUnitMembersQueryKey(workspaceId) })
    void queryClient.invalidateQueries({
      queryKey: projectBoardQueryOptions(workspaceId, projectId).queryKey,
    })
  }, [projectId, queryClient, workspaceId])

  return { membersByUnit, membersPendingUnits, refreshAfterMembersSaved }
}
