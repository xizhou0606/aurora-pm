import { useCallback, useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'

import type { ScmConfigScope } from './scope'

const VIEW_ACTIONS = {
  connections: PM_ACTION.ScmIntegrationConnectionsView,
  repositories: PM_ACTION.ScmIntegrationRepositoriesView,
  bindings: PM_ACTION.ScmIntegrationBindingsView,
  settings: PM_ACTION.ScmIntegrationSettingsView,
} as const

const MANAGE_ACTIONS = {
  connections: PM_ACTION.ScmIntegrationConnectionsManage,
  repositories: PM_ACTION.ScmIntegrationRepositoriesManage,
  bindings: PM_ACTION.ScmIntegrationBindingsManage,
  settings: PM_ACTION.ScmIntegrationSettingsManage,
} as const

// Capability owner for the SCM config page: every section derives visibility
// and action enablement from this single hook so 显隐与提交共用同一结果.
export function useScmConfigAccess() {
  const operations = useOperationCapabilities('scm-config')
  const canView = useCallback(
    (scope: ScmConfigScope) => operations.allows(VIEW_ACTIONS[scope]),
    [operations.allows],
  )
  const canManage = useCallback(
    (scope: ScmConfigScope) => operations.allows(MANAGE_ACTIONS[scope]),
    [operations.allows],
  )
  return useMemo(
    () => ({
      status: operations.status,
      retry: operations.retry,
      canView,
      canManage,
    }),
    [canManage, canView, operations.retry, operations.status],
  )
}
