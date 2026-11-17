import { useCallback, useMemo } from 'react'

import {
  pmGrantingActions,
  PM_OPERATION_ACTIONS_BY_DOMAIN,
  type PMOperationActionCode,
  type PMOperationDomain,
} from '@/generated/access-catalog'
import { useActionPermissions } from '../hooks/hooks'

export type OperationCapabilityState<D extends PMOperationDomain> = {
  status: 'loading' | 'error' | 'ready'
  allows: (
    action: PMOperationActionCode<D>,
    locallyAvailable?: boolean,
  ) => boolean
  retry: () => void
}

/**
 * Combines the authoritative action grant with a domain-owned availability
 * decision. Feature hooks own object capabilities and business state; leaf
 * components receive only the resulting can* value.
 */
export function resolveOperationCapability(
  globallyGranted: boolean,
  locallyAvailable = true,
): boolean {
  return globallyGranted && locallyAvailable
}

export function useOperationCapabilities<D extends PMOperationDomain>(
  domain: D,
): OperationCapabilityState<D> {
  const permissions = useActionPermissions()
  const domainActions = useMemo(
    () => new Set<string>(PM_OPERATION_ACTIONS_BY_DOMAIN[domain]),
    [domain],
  )
  const allows = useCallback((
    action: PMOperationActionCode<D>,
    locallyAvailable = true,
  ) => (
    domainActions.has(action)
    && resolveOperationCapability(
      pmGrantingActions(action).some((granted) => permissions.canAction(granted)),
      locallyAvailable,
    )
  ), [domainActions, permissions.canAction])

  return {
    status: permissions.status,
    allows,
    retry: permissions.retry,
  }
}
