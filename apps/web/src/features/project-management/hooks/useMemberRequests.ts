import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import { getProjectMembers, getProjectMemberRemovalImpact, putProjectMembers, type ProjectMember, type PutMembersPayload } from '../api'

export function useMemberRoster(workspaceId: number, revision: number, unitId: number) {
  return useQuery({
    queryKey: pmKeys.resource(workspaceId || 1, 'planUnits', unitId, 'members', revision),
    enabled: workspaceId > 0 && unitId > 0,
    retry: false,
    refetchOnWindowFocus: false,
    queryFn: async ({ signal }) => {
      const members: ProjectMember[] = []
      for (let page = 1; ; page += 1) {
        const result = await getProjectMembers(unitId, { page, page_size: 200 }, signal)
        members.push(...result.items)
        if (!result.has_more || result.items.length === 0) return members
      }
    },
  })
}

export function useMemberImpactRequest(workspaceId: number, revision: number, unitId: number) {
  const client = useQueryClient()
  return (userId: number) => client.fetchQuery({
    queryKey: pmKeys.resource(workspaceId, 'planUnits', unitId, 'member-removal', userId, revision),
    staleTime: 0,
    retry: false,
    queryFn: ({ signal }) => getProjectMemberRemovalImpact(unitId, userId, signal),
  })
}

export function useMemberSaveRequest(workspaceId: number, unitId: number) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (payload: PutMembersPayload) => putProjectMembers(unitId, payload),
    onSuccess: () => {
      for (const resource of ['planUnits', 'projectSets', 'tasks', 'globalWorkItems'] as const) {
        void client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, resource) })
      }
    },
  })
}
