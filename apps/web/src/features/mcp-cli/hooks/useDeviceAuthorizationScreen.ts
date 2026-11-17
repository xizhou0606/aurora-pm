import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'

import {
  decideDeviceAuthorization,
  fetchDeviceAuthorization,
  type DeviceAuthorizationDecision,
} from '../api/device-api'
import type { DeviceAuthorizationScreenModel } from '../model/model'
import { effectiveAccessQueryKey } from './helpers'

export function useDeviceAuthorizationScreen(
  userCode?: string,
): DeviceAuthorizationScreenModel {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const queryWorkspaceId = workspaceId > 0 ? workspaceId : 1
  const queryClient = useQueryClient()
  const request = useQuery({
    queryKey: pmKeys.resource(
      queryWorkspaceId,
      'workspace',
      'externalTooling',
      'deviceAuthorization',
      userCode ?? 'invalid',
    ),
    queryFn: ({ signal }) => fetchDeviceAuthorization(userCode!, signal),
    enabled: workspaceId > 0 && Boolean(userCode),
    retry: false,
  })
  const decision = useMutation({
    mutationFn: (value: DeviceAuthorizationDecision) => (
      decideDeviceAuthorization(userCode!, value)
    ),
    onSuccess: (status) => {
      if (status === 'approved' && workspaceId > 0) {
        void queryClient.invalidateQueries({ queryKey: effectiveAccessQueryKey(workspaceId) })
      }
    },
  })
  if (!userCode) return { status: 'invalid' }
  if (request.isPending) return { status: 'loading' }
  if (request.isError || !request.data) {
    return { status: 'error', retry: () => { void request.refetch() } }
  }
  if (decision.isSuccess) {
    return { status: decision.data, request: request.data }
  }
  if (request.data.status !== 'pending') {
    return { status: request.data.status, request: request.data }
  }
  return {
    status: 'ready',
    request: request.data,
    submit: decision.isPending ? 'pending' : decision.isError ? 'error' : 'idle',
    authorize: () => decision.mutate('authorize'),
    deny: () => decision.mutate('deny'),
  }
}
