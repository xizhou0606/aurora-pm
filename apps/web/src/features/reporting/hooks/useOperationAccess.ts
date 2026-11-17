import { useCallback, useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { PMSafeApiError } from '@/shared/api'

export function requireReportingOperation(allowed: boolean): void {
  if (!allowed) throw new PMSafeApiError('REPORTING_ACTION_PERMISSION_REQUIRED', 403)
}

export function useReportingOperationAccess() {
  const operations = useOperationCapabilities('reporting')
  const canViewMembers = useCallback(() => (
    operations.allows(PM_ACTION.ReportingMembersView)
  ), [operations.allows])
  const canGenerateMember = useCallback(() => (
    operations.allows(PM_ACTION.ReportingMembersGenerate)
  ), [operations.allows])
  const canViewGroup = useCallback(() => (
    operations.allows(PM_ACTION.ReportingGroupsView)
  ), [operations.allows])
  const canGenerateGroup = useCallback(() => (
    operations.allows(PM_ACTION.ReportingGroupsGenerate)
  ), [operations.allows])
  const canPublish = useCallback(() => (
    operations.allows(PM_ACTION.ReportingReportsPublish)
  ), [operations.allows])
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewMembers: canViewMembers(),
    canGenerateMember: canGenerateMember(),
    canViewGroup: canViewGroup(),
    canGenerateGroup: canGenerateGroup(),
    canPublish: canPublish(),
  }), [canGenerateGroup, canGenerateMember, canPublish, canViewGroup, canViewMembers, operations.retry, operations.status])
}
