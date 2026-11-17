import { useCallback, useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { PMSafeApiError } from '@/shared/api'
import type { QualityBug, QualityComment, QualitySceneOption } from '../model/types'

export function requireQualityOperation(allowed: boolean): void {
  if (!allowed) throw new PMSafeApiError('QUALITY_ACTION_PERMISSION_REQUIRED', 403)
}

export function useQualityOperationAccess() {
  const operations = useOperationCapabilities('quality-management')
  const canUpdateBug = useCallback((bug: QualityBug | null | undefined) => (
    operations.allows(PM_ACTION.QualityBugsUpdate, Boolean(bug?.capabilities.canUpdate))
  ), [operations.allows])
  const canDeleteBug = useCallback((bug: QualityBug | null | undefined) => (
    operations.allows(PM_ACTION.QualityBugsDelete, Boolean(bug?.capabilities.canDelete))
  ), [operations.allows])
  const canAssignBug = useCallback((bug: QualityBug | null | undefined) => (
    operations.allows(
      PM_ACTION.QualityBugsTransitionStatus,
      Boolean(bug?.capabilities.canAssign),
    )
  ), [operations.allows])
  const canResolveBug = useCallback((bug: QualityBug | null | undefined) => (
    operations.allows(
      PM_ACTION.QualityBugsTransitionStatus,
      Boolean(bug?.capabilities.canResolve),
    )
  ), [operations.allows])
  const canReturnBug = useCallback((bug: QualityBug | null | undefined) => (
    operations.allows(
      PM_ACTION.QualityBugsTransitionStatus,
      Boolean(bug?.capabilities.canReturn),
    )
  ), [operations.allows])
  const canTransitionBug = useCallback((bug: QualityBug | null | undefined) => (
    operations.allows(
      PM_ACTION.QualityBugsTransitionStatus,
      Boolean(
        bug?.capabilities.canTransition
        || bug?.capabilities.canResolve
        || bug?.capabilities.canReturn,
      ),
    )
  ), [operations.allows])
  const canUploadMaterial = useCallback((locallyAvailable = true) => (
    operations.allows(PM_ACTION.QualityMaterialsUpload, locallyAvailable)
  ), [operations.allows])
  const canDeleteComment = useCallback((comment: QualityComment | null | undefined) => (
    operations.allows(PM_ACTION.QualityCommentsDelete, Boolean(comment?.canDelete))
  ), [operations.allows])
  const canDeleteSceneOption = useCallback((option: QualitySceneOption | null | undefined) => (
    operations.allows(
      PM_ACTION.QualitySceneOptionsDelete,
      Boolean(option?.id && !option.builtIn && !option.legacy && !option.usageCount),
    )
  ), [operations.allows])

  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewBugs: operations.allows(PM_ACTION.QualityBugsView),
    canCreateBug: operations.allows(PM_ACTION.QualityBugsCreate),
    canViewDashboard: operations.allows(PM_ACTION.QualityDashboardView),
    canViewComments: operations.allows(PM_ACTION.QualityCommentsView),
    canCreateComment: operations.allows(PM_ACTION.QualityCommentsCreate),
    canViewSceneOptions: operations.allows(PM_ACTION.QualitySceneOptionsView),
    canCreateSceneOption: operations.allows(PM_ACTION.QualitySceneOptionsCreate),
    canUpdateBug,
    canDeleteBug,
    canAssignBug,
    canResolveBug,
    canReturnBug,
    canTransitionBug,
    canUploadMaterial,
    canDeleteComment,
    canDeleteSceneOption,
  }), [
    canAssignBug,
    canDeleteBug,
    canDeleteComment,
    canDeleteSceneOption,
    canResolveBug,
    canReturnBug,
    canTransitionBug,
    canUpdateBug,
    canUploadMaterial,
    operations.allows,
    operations.retry,
    operations.status,
  ])
}
