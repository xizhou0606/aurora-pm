import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import type { Environment } from '../components/environment'
import { biScreenKey } from '../api/queryKeys'
import type {
	RouteState,
	TeamMemberItemType,
	TeamMetricRow,
	TeamScoreProfileUpdate,
} from '../types'

const STALE_TIME = 30_000
const POLL_INTERVAL = 60_000

const queryPolicy = {
  staleTime: STALE_TIME,
  refetchOnWindowFocus: false as const,
  retry: false as const,
}

function scopeKey(route: RouteState): readonly unknown[] {
  return [
    route.scope.projectSetIds,
    route.scope.projectIds,
    route.scope.requirementIds,
    route.period,
    route.teamDimension,
    route.teamDimensionKey,
  ]
}

export function useScopeOptionsQuery(environment: Environment, enabled: boolean) {
  const workspaceId = environment.workspaceId > 0 ? environment.workspaceId : 1
  return useQuery({
    queryKey: biScreenKey(
      workspaceId,
      'scope-options',
      environment.sessionRevision,
    ),
    queryFn: ({ signal }) => environment.client.scopeOptions(signal),
    enabled,
    ...queryPolicy,
  })
}

export function dashboardQueryKey(
  environment: Pick<Environment, 'workspaceId' | 'sessionRevision'>,
  route: RouteState,
) {
  return biScreenKey(
    environment.workspaceId,
    'dashboard',
    environment.sessionRevision,
    ...scopeKey(route),
  )
}

export function useDashboardQuery(
  environment: Environment,
  route: RouteState,
  enabled: boolean,
) {
  const queryEnvironment = environment.workspaceId > 0
    ? environment
    : { ...environment, workspaceId: 1 }
  return useQuery({
    queryKey: dashboardQueryKey(queryEnvironment, route),
    queryFn: ({ signal }) => environment.client.dashboard(
      environment.workspaceId,
      route,
      signal,
    ),
    enabled,
    refetchInterval: () => (
      typeof document !== 'undefined' && document.visibilityState === 'visible'
        ? POLL_INTERVAL
        : false
    ),
    refetchIntervalInBackground: false,
    ...queryPolicy,
  })
}

export function useTeamMemberItemsQuery(
	environment: Environment,
	route: RouteState,
	input: {
		userId: number
		role: TeamMetricRow['role_key']
		itemType: TeamMemberItemType
	} | undefined,
	enabled = true,
) {
	return useQuery({
		queryKey: biScreenKey(
			environment.workspaceId,
			'team-member-items',
			environment.sessionRevision,
			...scopeKey(route),
			input?.role,
			input?.itemType,
			input?.userId,
		),
		queryFn: ({ signal }) => {
			if (!input) throw new Error('team member drilldown input is required')
			return environment.client.teamMemberItems(route, input, signal)
		},
		enabled: enabled && Boolean(input),
		...queryPolicy,
	})
}

export function useTeamScoreProfileMutation(
	environment: Environment,
	route: RouteState,
	canUpdate = true,
) {
	const queryClient = useQueryClient()
	return useMutation({
		mutationFn: (body: TeamScoreProfileUpdate) => canUpdate
			? environment.client.putTeamScoreProfile(body)
			: Promise.reject(new Error('BI screen scoring update permission required')),
		onSuccess: async () => {
			await queryClient.invalidateQueries({
				queryKey: dashboardQueryKey(environment, route),
			})
		},
	})
}
