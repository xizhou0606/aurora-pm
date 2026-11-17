import { useMutation, useQueryClient } from '@tanstack/react-query'

import type {
  ResourceAccessPolicyReplacement,
  ResourceShareLinkCreate,
} from '../model'
import type { ResourceAccessEnvironment } from './environment'
import { resourceAccessKeys } from './keys'

export function useReplaceResourceAccessPolicy(environment: ResourceAccessEnvironment) {
  const queryClient = useQueryClient()
  const keys = resourceAccessKeys(environment)
  return useMutation({
    mutationFn: (input: ResourceAccessPolicyReplacement) => (
      environment.client.replacePolicy(environment.resourceId, input)
    ),
    onSuccess: (policy) => {
      queryClient.setQueryData(keys.policy, policy)
      void queryClient.invalidateQueries({ queryKey: keys.links })
    },
  })
}

export function useCreateResourceShareLink(environment: ResourceAccessEnvironment) {
  const queryClient = useQueryClient()
  const keys = resourceAccessKeys(environment)
  return useMutation({
    gcTime: 0,
    mutationFn: (input: ResourceShareLinkCreate) => (
      environment.client.createLink(environment.resourceId, input)
    ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.links }),
  })
}

export function useRevokeResourceShareLink(environment: ResourceAccessEnvironment) {
  const queryClient = useQueryClient()
  const keys = resourceAccessKeys(environment)
  return useMutation({
    mutationFn: (linkId: string) => environment.client.revokeLink(environment.resourceId, linkId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.links }),
  })
}

export function useRotateResourceShareLink(environment: ResourceAccessEnvironment) {
  const queryClient = useQueryClient()
  const keys = resourceAccessKeys(environment)
  return useMutation({
    gcTime: 0,
    mutationFn: (linkId: string) => environment.client.rotateLink(environment.resourceId, linkId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.links }),
  })
}
