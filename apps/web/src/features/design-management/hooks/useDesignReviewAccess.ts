import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION, type PMOperationActionCode } from '@/generated/access-catalog'
import type { ReviewCapabilities } from '../model'

type AllowsDesignReviewAction = (
  action: PMOperationActionCode<'design-management'>,
  locallyAvailable?: boolean,
) => boolean

export function resolveDesignReviewAccess(
  allows: AllowsDesignReviewAction,
  local?: ReviewCapabilities,
) {
  return {
    canView: allows(PM_ACTION.DesignReviewsReviewsView, local?.can_view ?? false),
    canCreate: allows(PM_ACTION.DesignReviewsReviewsCreate, local?.can_create ?? false),
    canCreateNow: allows(PM_ACTION.DesignReviewsReviewsCreate, local?.creation_available ?? false),
    canManageDifferences: allows(PM_ACTION.DesignReviewsDifferencesManage, local?.can_manage_differences ?? false),
    canManageAnalysis: allows(PM_ACTION.DesignReviewsAnalysisManage, local?.can_manage_analysis ?? false),
    canAnalyzeNow: allows(PM_ACTION.DesignReviewsAnalysisManage, local?.analysis_available ?? false),
    canDelete: allows(PM_ACTION.DesignReviewsReviewsDelete, local?.can_delete ?? false),
    canManageCollectors: allows(PM_ACTION.DesignReviewsMobileCollectorsManage, local?.can_manage_collectors ?? false),
  }
}

export function useDesignReviewAccess(local?: ReviewCapabilities) {
  const operations = useOperationCapabilities('design-management')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    ...resolveDesignReviewAccess(operations.allows, local),
  }), [local, operations])
}
