import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import type { TeamMetricSettingsUpdate } from '../types'
import type { Environment } from '../components/environment'
import { biScreenKey } from '../api/queryKeys'

function settingsKey(environment: Pick<Environment, 'workspaceId' | 'sessionRevision'>) {
  return biScreenKey(
    environment.workspaceId > 0 ? environment.workspaceId : 1,
    'team-metric-settings',
    environment.sessionRevision,
  )
}

export function useTeamMetricSettingsQuery(
  environment: Environment,
  enabled: boolean,
) {
  return useQuery({
    queryKey: settingsKey(environment),
    queryFn: ({ signal }) => environment.client.teamMetricSettings(signal),
    enabled,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    retry: false,
  })
}

export function useTeamMetricSettingsMutation(
  environment: Environment,
  canUpdate = true,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: TeamMetricSettingsUpdate) => canUpdate
      ? environment.client.putTeamMetricSettings(body)
      : Promise.reject(new Error('BI screen metric settings update permission required')),
    onSuccess: async (settings) => {
      queryClient.setQueryData(settingsKey(environment), settings)
      await queryClient.invalidateQueries({
        queryKey: biScreenKey(
          environment.workspaceId > 0 ? environment.workspaceId : 1,
          'dashboard',
          environment.sessionRevision,
        ),
      })
    },
    onError: async () => {
      await queryClient.invalidateQueries({
        queryKey: settingsKey(environment),
        refetchType: 'active',
      })
    },
  })
}
